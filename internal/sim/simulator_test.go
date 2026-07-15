package sim

import (
	"context"
	"math/big"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"predictionmarket-simulator/internal/config"
	dbwriter "predictionmarket-simulator/internal/db"
	"predictionmarket-simulator/internal/scenario"
)

func TestPrepareExecutionPlanRegeneratesInsteadOfReadingExistingPlan(t *testing.T) {
	planFile := filepath.Join(t.TempDir(), "simulator-plan.json")
	stale := &Plan{
		Version: planVersion, GeneratedAt: "stale", Scenario: config.ScenarioCreateAndTrade,
		Participants: []PlanParticipant{{Index: 0, Address: "stale", PrivateKey: "stale"}},
		Markets:      []PlanMarket{{Index: 1, IPFSCID: "stale", Trades: []PlanTrade{}}},
	}
	if err := writePlan(planFile, stale); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Runtime: config.RuntimeConfig{
			Enabled: true, Mode: config.ModeExecute, PlanFile: planFile,
			RegeneratePlanOnExecute: true,
		},
		Scenario: config.ScenarioConfig{
			Type: config.ScenarioCreateAndTrade, MarketCount: 1, Participants: 2,
			TradesPerMarketMin: 1, TradesPerMarketMax: 1,
		},
		Market: config.MarketConfig{
			Types: []string{"TYPE_PRICE"}, InitialLiquidityMinBKC: "1", InitialLiquidityMaxBKC: "1",
			DurationMin: 24 * time.Hour, DurationMax: 24 * time.Hour,
		},
		Trade: config.TradeConfig{BuyMinBKC: "0.1", BuyMaxBKC: "0.1"},
	}
	simulator := New(cfg, nil)
	simulator.rng = rand.New(rand.NewSource(1))
	plan, err := simulator.prepareExecutionPlan()
	if err != nil {
		t.Fatal(err)
	}
	if plan.GeneratedAt == "stale" || len(plan.Participants) != 2 || plan.Participants[0].Address == "stale" {
		t.Fatalf("execution reused stale plan: %+v", plan)
	}
	saved, err := readPlan(planFile)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Participants[0].Address != plan.Participants[0].Address {
		t.Fatal("fresh execution plan was not persisted")
	}
}

func TestPrepareExecutionPlanReusesPlanWhenRegenerationDisabled(t *testing.T) {
	planFile := filepath.Join(t.TempDir(), "simulator-plan.json")
	want := &Plan{
		Version: planVersion, GeneratedAt: "replay", Scenario: config.ScenarioCreateAndTrade,
		Participants: []PlanParticipant{{Index: 0, Address: "saved", PrivateKey: "saved"}},
		Markets:      []PlanMarket{{Index: 1, IPFSCID: "saved", Trades: []PlanTrade{}}},
	}
	if err := writePlan(planFile, want); err != nil {
		t.Fatal(err)
	}
	simulator := New(&config.Config{Runtime: config.RuntimeConfig{PlanFile: planFile}}, nil)
	got, err := simulator.prepareExecutionPlan()
	if err != nil {
		t.Fatal(err)
	}
	if got.GeneratedAt != "replay" || got.Participants[0].Address != "saved" {
		t.Fatalf("saved plan was not reused: %+v", got)
	}
}

func TestExecuteOffchainTradeMatchesContractBuyYesFormula(t *testing.T) {
	initial := big.NewInt(100)
	state := newOffchainMarketState(7, "sim-cid", initial, 12345)

	trade, err := executeOffchainTrade(state, "0x1234567890123456789012345678901234567890", 0, big.NewInt(25))
	if err != nil {
		t.Fatal(err)
	}

	if got, want := state.reserveNo.String(), "125"; got != want {
		t.Fatalf("reserveNo = %s, want %s", got, want)
	}
	if got, want := state.reserveYes.String(), "80"; got != want {
		t.Fatalf("reserveYes = %s, want %s", got, want)
	}
	if got, want := trade.ShareAmountWei.String(), "45"; got != want {
		t.Fatalf("share amount = %s, want %s", got, want)
	}
	if got, want := dbwriter.ShareAt(trade.Extra.VirtualReservesNOYES, 0).String(), "125"; got != want {
		t.Fatalf("extra reserve NO = %s, want %s", got, want)
	}
	if got, want := dbwriter.ShareAt(trade.Extra.VirtualReservesNOYES, 1).String(), "80"; got != want {
		t.Fatalf("extra reserve YES = %s, want %s", got, want)
	}
	if got, want := trade.MySharesYesAfter, "45"; got != want {
		t.Fatalf("YES shares after = %s, want %s", got, want)
	}
}

type recordingMetadataUploader struct {
	payload string
	cid     string
}

func (u *recordingMetadataUploader) UploadMetadata(_ context.Context, payload string) (string, error) {
	u.payload = payload
	return u.cid, nil
}

func TestUploadMarketMetadataReplacesPlaceholderCID(t *testing.T) {
	uploader := &recordingMetadataUploader{cid: "local-v1-uploaded"}
	simulator := &Simulator{metadataUploader: uploader}
	market := &scenario.Market{
		IPFSCID:      "sim-placeholder",
		MetadataJSON: `{"desc":"黄金 上涨"}`,
	}

	if err := simulator.uploadMarketMetadata(context.Background(), market); err != nil {
		t.Fatal(err)
	}
	if uploader.payload != market.MetadataJSON {
		t.Fatalf("payload = %q, want %q", uploader.payload, market.MetadataJSON)
	}
	if market.IPFSCID != "local-v1-uploaded" {
		t.Fatalf("market cid = %q, want uploaded cid", market.IPFSCID)
	}
}

func TestRunPreviewWritesReviewablePlanFile(t *testing.T) {
	planFile := filepath.Join(t.TempDir(), "simulator-plan.json")
	cfg := &config.Config{
		Runtime: config.RuntimeConfig{
			Enabled:  true,
			Mode:     config.ModePreview,
			OnChain:  true,
			PlanFile: planFile,
		},
		Chain: config.ChainConfig{
			ContractAddress: "0xad4F9eD0F2b51A26314C9f83DF588cCcE26ae03c",
		},
		Scenario: config.ScenarioConfig{
			Type:               config.ScenarioCreateAndTrade,
			MarketCount:        2,
			Participants:       3,
			TradesPerMarketMin: 1,
			TradesPerMarketMax: 1,
		},
		Market: config.MarketConfig{
			Types:                  []string{"TYPE_PRICE", "TYPE_PRICE_RANGE"},
			InitialLiquidityMinBKC: "3",
			InitialLiquidityMaxBKC: "3",
			DurationMin:            24 * time.Hour,
			DurationMax:            24 * time.Hour,
		},
		Trade: config.TradeConfig{
			BuyMinBKC: "0.2",
			BuyMaxBKC: "0.2",
		},
		Timing: config.TimingConfig{
			Timeout: 5 * time.Second,
		},
	}

	if err := New(cfg, nil).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	plan, err := readPlan(planFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Participants) != 3 {
		t.Fatalf("participants = %d, want 3", len(plan.Participants))
	}
	if len(plan.Markets) != 2 {
		t.Fatalf("markets = %d, want 2", len(plan.Markets))
	}
	marketTypes := map[string]bool{}
	for _, market := range plan.Markets {
		marketTypes[market.Type] = true
	}
	if !marketTypes["TYPE_PRICE"] || !marketTypes["TYPE_PRICE_RANGE"] {
		t.Fatalf("preview did not include configured market types: %+v", marketTypes)
	}
	if len(plan.Markets[0].Trades) != 1 {
		t.Fatalf("first market trades = %d, want 1", len(plan.Markets[0].Trades))
	}
	if plan.Participants[0].PrivateKey == "" || plan.Participants[0].Address == "" {
		t.Fatalf("preview plan did not include replayable participant credentials: %+v", plan.Participants[0])
	}
}

func TestExecutePlannedTradesRejectsUserAddressMismatch(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeConfig{Mode: config.ModeExecute},
		Timing:  config.TimingConfig{Timeout: time.Second},
	}
	participants := []participant{{
		privateKey: "abc",
		address:    "0x1234567890123456789012345678901234567890",
	}}
	trades := []PlanTrade{{
		Index:     1,
		UserIndex: 0,
		User:      "0x9999999999999999999999999999999999999999",
		OptionID:  0,
		AmountWei: "1",
	}}

	err := New(cfg, nil).executePlannedTradesForMarket(context.Background(), nil, participants, 1, nil, trades)
	if err == nil {
		t.Fatal("executePlannedTradesForMarket succeeded, want user mismatch error")
	}
}

func TestBuildPlannedTradesSchedulesDifferentUsersAtDifferentTimes(t *testing.T) {
	cfg := &config.Config{
		Scenario: config.ScenarioConfig{
			Participants: 4, TradesPerMarketMin: 12, TradesPerMarketMax: 12,
		},
		Trade: config.TradeConfig{
			BuyMinBKC: "0.1", BuyMaxBKC: "0.1", CreatorAlsoTrades: false,
		},
		Timing: config.TimingConfig{
			TradeIntervalMin: 2 * time.Second,
			TradeIntervalMax: 5 * time.Second,
		},
	}
	participants, err := buildParticipants(cfg.Scenario.Participants)
	if err != nil {
		t.Fatal(err)
	}
	simulator := New(cfg, nil)
	simulator.rng = rand.New(rand.NewSource(7))

	trades, err := simulator.buildPlannedTrades(participants, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i, trade := range trades {
		if trade.DelaySeconds < 2 || trade.DelaySeconds > 5 {
			t.Fatalf("trade #%d delay = %v, want [2, 5] seconds", trade.Index, trade.DelaySeconds)
		}
		if trade.UserIndex == 0 {
			t.Fatalf("trade #%d selected creator", trade.Index)
		}
		if i > 0 && trade.UserIndex == trades[i-1].UserIndex {
			t.Fatalf("trades #%d and #%d use the same participant %d", i, i+1, trade.UserIndex)
		}
	}
}

func TestRunContinuousStopsCleanlyWhenContextIsCanceled(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeConfig{
			Enabled: true, Mode: config.ModeExecute, Continuous: true,
			RegeneratePlanOnExecute: true,
		},
		Timing: config.TimingConfig{Timeout: time.Second},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := New(cfg, nil).Run(ctx); err != nil {
		t.Fatalf("Run returned %v after cancellation, want clean shutdown", err)
	}
}

func TestExecutePlannedTradesWaitsForScheduledDelay(t *testing.T) {
	cfg := &config.Config{Runtime: config.RuntimeConfig{Mode: config.ModeExecute}}
	participants := []participant{{
		privateKey: "abc",
		address:    "0x1234567890123456789012345678901234567890",
	}}
	trades := []PlanTrade{{
		Index: 1, UserIndex: 0, User: participants[0].address,
		OptionID: 0, AmountWei: "1", DelaySeconds: 1,
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := New(cfg, nil).executePlannedTradesForMarket(ctx, nil, participants, 1, nil, trades)
	if err != context.DeadlineExceeded {
		t.Fatalf("executePlannedTradesForMarket returned %v, want deadline while waiting", err)
	}
}

func TestOffchainStateFromStoredMarketPreservesExistingReserves(t *testing.T) {
	stored := &dbwriter.ExistingMarketState{
		GameID:         12,
		IPFSCID:        "existing-cid",
		TotalPool:      big.NewInt(900),
		ReserveYes:     big.NewInt(300),
		ReserveNo:      big.NewInt(600),
		DeadlineSec:    time.Now().Add(time.Hour).Unix(),
		ExistingTrades: 17,
	}

	state, err := offchainStateFromStoredMarket(stored)
	if err != nil {
		t.Fatal(err)
	}
	if state.info.ID != 12 || state.info.TotalPool.String() != "900" {
		t.Fatalf("unexpected game info: %+v", state.info)
	}
	if state.reserveYes.String() != "300" || state.reserveNo.String() != "600" {
		t.Fatalf("unexpected reserves: YES=%s NO=%s", state.reserveYes, state.reserveNo)
	}
	if state.nextTradeSeq != 17 {
		t.Fatalf("next trade sequence = %d, want 17", state.nextTradeSeq)
	}
}
