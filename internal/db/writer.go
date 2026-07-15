package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"predictionmarket-simulator/internal/chain"
	"predictionmarket-simulator/internal/config"
	"predictionmarket-simulator/internal/scenario"

	_ "github.com/go-sql-driver/mysql"
)

type Writer struct {
	db              *sql.DB
	contractAddress string
}

type TradeRecord struct {
	GameID           int
	UserAddress      string
	OptionID         int
	AmountWei        *big.Int
	ShareAmountWei   *big.Int
	TxHash           string
	TimestampSec     int64
	Info             *chain.GameInfo
	Extra            *chain.GameExtraData
	YesPrice         float64
	NoPrice          float64
	PriceAtTrade     float64
	MySharesYesAfter string
	MySharesNoAfter  string
}

type ExistingMarketState struct {
	GameID         int
	IPFSCID        string
	TotalPool      *big.Int
	ReserveYes     *big.Int
	ReserveNo      *big.Int
	IsResolved     bool
	IsRefunded     bool
	WinningOption  int
	DeadlineSec    int64
	ExistingTrades int
}

func Open(ctx context.Context, cfg config.MySQLConfig, contractAddress string) (*Writer, error) {
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)
	db.SetConnMaxLifetime(cfg.ConnectionMaxLifetime)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return &Writer{db: db, contractAddress: normalizeAddress(contractAddress)}, nil
}

func (w *Writer) Close() error {
	return w.db.Close()
}

func (w *Writer) NextGameID(ctx context.Context) (int, error) {
	var maxID sql.NullInt64
	if err := w.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(game_id), 0) FROM gold_games`).Scan(&maxID); err != nil {
		return 0, fmt.Errorf("read max game_id: %w", err)
	}
	return int(maxID.Int64) + 1, nil
}

func (w *Writer) LoadExistingMarketState(ctx context.Context, gameID int) (*ExistingMarketState, error) {
	var state ExistingMarketState
	var totalPoolRaw, reserveYesRaw, reserveNoRaw string
	var resolved, refunded int
	err := w.db.QueryRowContext(ctx, `SELECT
		g.game_id, g.ipfs_cid, s.total_pool, s.reserve_yes, s.reserve_no,
		s.is_resolved, s.is_refunded, s.winning_option, s.deadline_sec,
		(SELECT COUNT(*) FROM gold_trades t WHERE t.game_id = g.game_id AND LOWER(t.contract_address) = ?)
		FROM gold_games g
		JOIN gold_chain_states s ON s.game_id = g.game_id
			AND LOWER(s.contract_address) = LOWER(g.contract_address)
		WHERE g.game_id = ? AND LOWER(g.contract_address) = ?`,
		w.contractAddress, gameID, w.contractAddress,
	).Scan(
		&state.GameID, &state.IPFSCID, &totalPoolRaw, &reserveYesRaw, &reserveNoRaw,
		&resolved, &refunded, &state.WinningOption, &state.DeadlineSec, &state.ExistingTrades,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("existing game %d was not found for contract %s", gameID, w.contractAddress)
	}
	if err != nil {
		return nil, fmt.Errorf("load existing game %d: %w", gameID, err)
	}
	state.TotalPool, err = parseStoredBigInt("total_pool", totalPoolRaw)
	if err != nil {
		return nil, err
	}
	state.ReserveYes, err = parseStoredBigInt("reserve_yes", reserveYesRaw)
	if err != nil {
		return nil, err
	}
	state.ReserveNo, err = parseStoredBigInt("reserve_no", reserveNoRaw)
	if err != nil {
		return nil, err
	}
	state.IsResolved = resolved != 0
	state.IsRefunded = refunded != 0
	return &state, nil
}

func parseStoredBigInt(field string, raw string) (*big.Int, error) {
	value := new(big.Int)
	if _, ok := value.SetString(strings.TrimSpace(raw), 10); !ok || value.Sign() < 0 {
		return nil, fmt.Errorf("existing market %s is not a non-negative integer", field)
	}
	return value, nil
}

func (w *Writer) SyncCreatedMarket(ctx context.Context, gameID int, market *scenario.Market, info *chain.GameInfo, initialLiquidityWei *big.Int, timestampSec int64) error {
	deadlineSec := time.Now().Unix() + market.DurationSeconds
	totalPool := new(big.Int).Set(initialLiquidityWei)
	reserveYes := new(big.Int).Set(initialLiquidityWei)
	reserveNo := new(big.Int).Set(initialLiquidityWei)
	if info != nil {
		deadlineSec = normalizeDeadlineSec(info.DeadlineRaw)
		if info.TotalPool != nil {
			totalPool = new(big.Int).Set(info.TotalPool)
		}
	}
	if timestampSec <= 0 {
		timestampSec = time.Now().Unix()
	}
	_, err := w.db.ExecContext(ctx, `INSERT INTO gold_games
		(game_id, contract_address, ipfs_cid, `+"`desc`"+`, `+"`condition`"+`,
		avatar_url, detailed_info, option_yes, option_no, creator_address, deadline_sec)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		contract_address=VALUES(contract_address), ipfs_cid=VALUES(ipfs_cid),
		`+"`desc`"+`=VALUES(`+"`desc`"+`), `+"`condition`"+`=VALUES(`+"`condition`"+`),
		avatar_url=VALUES(avatar_url), detailed_info=VALUES(detailed_info),
		option_yes=VALUES(option_yes), option_no=VALUES(option_no),
		creator_address=VALUES(creator_address), deadline_sec=VALUES(deadline_sec)`,
		gameID,
		w.contractAddress,
		market.IPFSCID,
		market.Desc,
		market.Condition,
		market.AvatarURL,
		market.DetailedInfo,
		market.OptionYes,
		market.OptionNo,
		normalizeAddress(market.CreatorAddress),
		deadlineSec,
	)
	if err != nil {
		return fmt.Errorf("sync gold_games: %w", err)
	}
	if err := w.upsertChainState(ctx, gameID, totalPool, false, false, 0, deadlineSec, reserveYes, reserveNo); err != nil {
		return err
	}
	if err := w.appendGoldPriceHistory(ctx, gameID, timestampSec, 50, 50, totalPool); err != nil {
		return err
	}
	return w.appendMarketHistory(ctx, gameID, timestampSec, 50, 50, reserveNo, reserveYes)
}

func (w *Writer) SyncTrade(ctx context.Context, trade *TradeRecord) error {
	deadlineSec := normalizeDeadlineSec(trade.Info.DeadlineRaw)
	reserveYes := shareAt(trade.Extra.VirtualReservesNOYES, 1)
	reserveNo := shareAt(trade.Extra.VirtualReservesNOYES, 0)
	if err := w.upsertChainState(ctx, trade.GameID, trade.Info.TotalPool, trade.Info.IsResolved, trade.Info.IsRefunded, trade.Info.WinningOption, deadlineSec, reserveYes, reserveNo); err != nil {
		return err
	}
	if err := w.upsertUserPosition(ctx, trade); err != nil {
		return err
	}
	if err := w.insertTrade(ctx, trade); err != nil {
		return err
	}
	if err := w.appendGoldPriceHistory(ctx, trade.GameID, trade.TimestampSec, trade.YesPrice, trade.NoPrice, trade.Info.TotalPool); err != nil {
		return err
	}
	return w.appendMarketHistory(ctx, trade.GameID, trade.TimestampSec, trade.YesPrice, trade.NoPrice, reserveNo, reserveYes)
}

func (w *Writer) upsertChainState(ctx context.Context, gameID int, totalPool *big.Int, resolved bool, refunded bool, winner int, deadlineSec int64, reserveYes *big.Int, reserveNo *big.Int) error {
	_, err := w.db.ExecContext(ctx, `INSERT INTO gold_chain_states
		(contract_address, game_id, total_pool, is_resolved, is_refunded, winning_option,
		deadline_sec, reserve_yes, reserve_no)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		total_pool=VALUES(total_pool), is_resolved=VALUES(is_resolved),
		is_refunded=VALUES(is_refunded), winning_option=VALUES(winning_option),
		deadline_sec=VALUES(deadline_sec), reserve_yes=VALUES(reserve_yes),
		reserve_no=VALUES(reserve_no)`,
		w.contractAddress, gameID, bigIntToDB(totalPool), boolToInt(resolved), boolToInt(refunded),
		winner, deadlineSec, bigIntToDB(reserveYes), bigIntToDB(reserveNo),
	)
	if err != nil {
		return fmt.Errorf("upsert gold_chain_states: %w", err)
	}
	return nil
}

func (w *Writer) upsertUserPosition(ctx context.Context, trade *TradeRecord) error {
	_, err := w.db.ExecContext(ctx, `INSERT INTO gold_user_positions
		(user_address, game_id, my_shares_yes, my_shares_no)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		my_shares_yes=VALUES(my_shares_yes), my_shares_no=VALUES(my_shares_no)`,
		normalizeAddress(trade.UserAddress), trade.GameID,
		bigIntToDB(shareAt(trade.Extra.MySharesYESNO, 0)),
		bigIntToDB(shareAt(trade.Extra.MySharesYESNO, 1)),
	)
	if err != nil {
		return fmt.Errorf("upsert gold_user_positions: %w", err)
	}
	return nil
}

func (w *Writer) insertTrade(ctx context.Context, trade *TradeRecord) error {
	_, err := w.db.ExecContext(ctx, `INSERT INTO gold_trades
		(game_id, contract_address, user_address, trade_type, option_id, amount_wei,
		share_amount_wei, shares_wei, price_at_trade, timestamp_sec, tx_hash, is_success,
		is_ai_managed, my_shares_yes_after, my_shares_no_after)
		VALUES (?, ?, ?, 'BUY', ?, ?, ?, ?, ?, ?, ?, 1, 0, ?, ?)`,
		trade.GameID, w.contractAddress, normalizeAddress(trade.UserAddress), trade.OptionID,
		bigIntToDB(trade.AmountWei), bigIntString(trade.ShareAmountWei), bigIntToDB(trade.ShareAmountWei),
		trade.PriceAtTrade, trade.TimestampSec, trade.TxHash, trade.MySharesYesAfter, trade.MySharesNoAfter,
	)
	if err != nil {
		return fmt.Errorf("insert gold_trades: %w", err)
	}
	return nil
}

func (w *Writer) appendGoldPriceHistory(ctx context.Context, gameID int, timestampSec int64, yesPrice float64, noPrice float64, totalPool *big.Int) error {
	_, err := w.db.ExecContext(ctx, `INSERT INTO gold_price_history
		(game_id, timestamp_sec, yes_price, no_price, total_pool)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		yes_price=VALUES(yes_price), no_price=VALUES(no_price), total_pool=VALUES(total_pool)`,
		gameID, timestampSec, yesPrice, noPrice, bigIntToDB(totalPool),
	)
	if err != nil {
		return fmt.Errorf("append gold_price_history: %w", err)
	}
	return nil
}

func (w *Writer) appendMarketHistory(ctx context.Context, gameID int, timestampSec int64, yesPrice float64, noPrice float64, reserveNo *big.Int, reserveYes *big.Int) error {
	_, err := w.db.ExecContext(ctx, `INSERT INTO market_history
		(contract_address, game_id, observed_at, yes_percent, no_percent, reserve_no, reserve_yes, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'chain')
		ON DUPLICATE KEY UPDATE
		yes_percent=VALUES(yes_percent), no_percent=VALUES(no_percent),
		reserve_no=VALUES(reserve_no), reserve_yes=VALUES(reserve_yes), source=VALUES(source)`,
		w.contractAddress, gameID, timestampSec, yesPrice, noPrice, bigIntToDB(reserveNo), bigIntToDB(reserveYes),
	)
	if err != nil {
		return fmt.Errorf("append market_history: %w", err)
	}
	return nil
}

func PricesFromReserves(reserveNo *big.Int, reserveYes *big.Int) (float64, float64) {
	noFloat := new(big.Float).SetInt(nonNilBig(reserveNo))
	yesFloat := new(big.Float).SetInt(nonNilBig(reserveYes))
	total := new(big.Float).Add(noFloat, yesFloat)
	if total.Sign() <= 0 {
		return 50, 50
	}
	yes, _ := new(big.Float).Quo(noFloat, total).Float64()
	no, _ := new(big.Float).Quo(yesFloat, total).Float64()
	yes *= 100
	no *= 100
	if math.IsNaN(yes) || math.IsInf(yes, 0) || math.IsNaN(no) || math.IsInf(no, 0) {
		return 50, 50
	}
	return yes, no
}

func PriceForOption(optionID int, yes float64, no float64) float64 {
	if optionID == 0 {
		return yes
	}
	return no
}

func ShareAt(values []*big.Int, index int) *big.Int {
	return shareAt(values, index)
}

func BigIntString(value *big.Int) string {
	return bigIntString(value)
}

func shareAt(values []*big.Int, index int) *big.Int {
	if index < 0 || index >= len(values) || values[index] == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(values[index])
}

func bigIntString(value *big.Int) string {
	if value == nil {
		return "0"
	}
	return value.String()
}

func bigIntToDB(value *big.Int) []byte {
	return []byte(bigIntString(value))
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nonNilBig(value *big.Int) *big.Int {
	if value == nil {
		return big.NewInt(0)
	}
	return value
}

func normalizeAddress(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeDeadlineSec(raw int64) int64 {
	if raw > 10_000_000_000 {
		return raw / 1000
	}
	return raw
}
