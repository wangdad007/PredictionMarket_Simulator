package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	TypePrice           = "TYPE_PRICE"
	TypeReturnThreshold = "TYPE_RETURN_THRESHOLD"
	TypePriceThreshold  = "TYPE_PRICE_THRESHOLD"
	TypePriceRange      = "TYPE_PRICE_RANGE"
	TypeRelative        = "TYPE_RELATIVE"
	TypeStreak          = "TYPE_STREAK"

	// Legacy constants remain readable by older plans, but are not offered for creation.
	TypeVolatility = "TYPE_VOLATILITY"
	TypeVolume     = "TYPE_VOLUME"
	TypeTechnical  = "TYPE_TECHNICAL"
	TypeTouch      = "TYPE_TOUCH"
	TypeEvent      = "TYPE_EVENT"

	ChainlinkSource        = "CHAINLINK_DATA_FEED_ETHEREUM"
	ChainlinkXAUUSDFeed    = "0x214eD9Da11D2fbe465a6fc601a91E62EbEc1a0D6"
	ChainlinkBTCUSDFeed    = "0xF4030086522a5bEEa4988F8cA5B36dbC97BeE88c"
	ChainlinkETHUSDFeed    = "0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419"
	ChainlinkSOLUSDFeed    = "0x4ffC43a60e009B551865A93d232E33Fce9f01507"
	ChainlinkBNBUSDFeed    = "0x14e613AC84a31f709eadbdF89C6CC390fDc9540A"
	BoundaryLastAtOrBefore = "LAST_AT_OR_BEFORE"
	MaxStalenessSeconds    = int64(43200)
)

var benchmarkFeeds = map[string]string{
	"BTC": ChainlinkBTCUSDFeed,
	"ETH": ChainlinkETHUSDFeed,
	"SOL": ChainlinkSOLUSDFeed,
	"BNB": ChainlinkBNBUSDFeed,
}

type Template struct {
	Type         string
	Title        string
	OptionYes    string
	OptionNo     string
	Params       []string
	DetailedInfo string
}

type Market struct {
	Type             string
	IPFSCID          string
	Desc             string
	Condition        string
	AvatarURL        string
	DetailedInfo     string
	OptionYes        string
	OptionNo         string
	CreatorAddress   string
	DurationSeconds  int64
	InitialLiquidity string
	MetadataJSON     string
}

type BuildMarketInput struct {
	Type         string
	Index        int
	Creator      string
	Duration     time.Duration
	InitialBKC   string
	Now          time.Time
	TemplateSeed int
}

var templates = map[string]Template{
	TypePrice: {
		Type: TypePrice, Title: "黄金价格方向", OptionYes: "YES", OptionNo: "NO",
		Params: []string{"UP", "DOWN", "FLAT"}, DetailedInfo: "按北京时间整日边界比较 Chainlink XAU/USD 起止价格。",
	},
	TypeReturnThreshold: {
		Type: TypeReturnThreshold, Title: "黄金涨跌幅", OptionYes: "YES", OptionNo: "NO",
		Params: []string{"1", "2", "3"}, DetailedInfo: "按 Chainlink XAU/USD 起止价计算绝对涨跌幅。",
	},
	TypePriceThreshold: {
		Type: TypePriceThreshold, Title: "黄金价格阈值", OptionYes: "YES", OptionNo: "NO",
		Params: []string{"3900", "4100", "4300"}, DetailedInfo: "使用截止边界前最后一轮 Chainlink XAU/USD 价格。",
	},
	TypePriceRange: {
		Type: TypePriceRange, Title: "黄金价格区间", OptionYes: "YES", OptionNo: "NO",
		Params: []string{"3800,4000", "4000,4200", "4200,4400"}, DetailedInfo: "判断截止价是否位于预先约定的闭区间。",
	},
	TypeRelative: {
		Type: TypeRelative, Title: "黄金相对加密资产表现", OptionYes: "YES", OptionNo: "NO",
		Params: []string{"BTC", "ETH", "SOL", "BNB"}, DetailedInfo: "使用同一北京时间窗的 Chainlink XAU/USD 与所选加密资产/USD 计算收益率。",
	},
	TypeStreak: {
		Type: TypeStreak, Title: "黄金连续涨跌", OptionYes: "YES", OptionNo: "NO",
		Params: []string{"UP", "DOWN"}, DetailedInfo: "比较每个连续北京交易日边界的 Chainlink XAU/USD 价格。",
	},
}

func SupportedTypes() []string {
	return []string{
		TypePrice, TypeReturnThreshold, TypePriceThreshold,
		TypePriceRange, TypeRelative, TypeStreak,
	}
}

func TemplateForType(typ string) (Template, bool) {
	tpl, ok := templates[strings.TrimSpace(typ)]
	return tpl, ok
}

func BuildMarket(input BuildMarketInput) (*Market, error) {
	tpl, ok := TemplateForType(input.Type)
	if !ok {
		return nil, fmt.Errorf("unsupported market type %q", input.Type)
	}
	if input.Now.IsZero() {
		input.Now = time.Now()
	}
	var start, end time.Time
	var err error
	if input.Type == TypeStreak {
		start, end, err = NormalizeStreakWindow(input.Now, input.Duration)
	} else {
		start, end, err = NormalizeMarketWindow(input.Now, input.Duration)
	}
	if err != nil {
		return nil, err
	}
	seed := nonNegativeSeed(input.TemplateSeed)
	param := tpl.Params[seed%len(tpl.Params)]
	desc, condition, rule := buildTemplateContent(tpl.Type, param, start, end, seed)
	avatarURL := "template://" + tpl.Type
	durationSeconds := int64(end.Sub(input.Now).Seconds())
	if durationSeconds <= 0 {
		return nil, fmt.Errorf("normalized market deadline must be in the future")
	}
	metadata := map[string]any{
		"type": tpl.Type, "desc": desc, "condition": condition, "avatarUrl": avatarURL,
		"detailedInfo": tpl.DetailedInfo, "optionYES": tpl.OptionYes, "optionNO": tpl.OptionNo,
		"creator": input.Creator, "initialBKC": input.InitialBKC, "durationSec": durationSeconds,
		"templateParam": param, "resolutionRule": rule,
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return &Market{
		Type: tpl.Type, IPFSCID: "sim-" + hex.EncodeToString(sum[:])[:32],
		Desc: desc, Condition: condition, AvatarURL: avatarURL, DetailedInfo: tpl.DetailedInfo,
		OptionYes: tpl.OptionYes, OptionNo: tpl.OptionNo, CreatorAddress: input.Creator,
		DurationSeconds: durationSeconds, InitialLiquidity: input.InitialBKC, MetadataJSON: string(raw),
	}, nil
}

func buildTemplateContent(typ, param string, start, end time.Time, seed int) (string, string, map[string]any) {
	days := int(end.Sub(start) / (24 * time.Hour))
	rule := baseResolutionRule(typ, start, end)
	period := fmt.Sprintf("北京时间 %s 00:00 至 %s 00:00", start.Format("2006-01-02"), end.Format("2006-01-02"))
	switch typ {
	case TypePrice:
		directionNames := map[string]string{"UP": "上涨", "DOWN": "下跌", "FLAT": "持平"}
		rule["direction"], rule["flat_tolerance_percent"] = param, 0.05
		return fmt.Sprintf("黄金价格 %s %d天", directionNames[param], days),
			fmt.Sprintf("%s，XAU/USD 收益率相对 ±0.05%% 容差判定%s", period, directionNames[param]), rule
	case TypeReturnThreshold:
		operator, operatorName := orderedOperator(seed)
		rule["operator"], rule["threshold"] = operator, numericParam(param)
		return fmt.Sprintf("黄金涨跌幅 %s %s%%", operatorName, param),
			fmt.Sprintf("%s，XAU/USD 绝对收益率%s%s%%", period, operatorName, param), rule
	case TypePriceThreshold:
		operator, operatorName := orderedOperator(seed)
		rule["operator"], rule["threshold"] = operator, numericParam(param)
		return fmt.Sprintf("黄金价格 %s %sUSD/盎司", operatorName, param),
			fmt.Sprintf("%s 截止边界的 XAU/USD 价格%s%sUSD/盎司", period, operatorName, param), rule
	case TypePriceRange:
		lower, upper := parseRange(param)
		operator, operatorName := "IN_RANGE", "位于"
		if seed%2 == 1 {
			operator, operatorName = "OUTSIDE_RANGE", "不在"
		}
		rule["operator"], rule["lower_threshold"], rule["upper_threshold"] = operator, lower, upper
		return fmt.Sprintf("黄金价格 %s %g-%gUSD/盎司", operatorName, lower, upper),
			fmt.Sprintf("%s 截止边界的 XAU/USD 价格%s闭区间 [%g, %g]USD/盎司", period, operatorName, lower, upper), rule
	case TypeRelative:
		rule["benchmark"], rule["benchmark_source_contract"] = param, benchmarkFeeds[param]
		return "黄金 跑赢 " + param,
			fmt.Sprintf("%s，XAU/USD 收益率严格高于 %s/USD 收益率", period, param), rule
	case TypeStreak:
		directionName := map[string]string{"UP": "上涨", "DOWN": "下跌"}[param]
		rule["direction"], rule["streak_days"] = param, days
		return fmt.Sprintf("黄金价格 连续%s %d天", directionName, days),
			fmt.Sprintf("%s，每个相邻北京日边界的 XAU/USD 价格均%s", period, directionName), rule
	default:
		return "", "", rule
	}
}

func baseResolutionRule(typ string, start, end time.Time) map[string]any {
	return map[string]any{
		"rule_version": 2, "type": typ, "symbol": "XAU", "source": ChainlinkSource,
		"source_contract": ChainlinkXAUUSDFeed, "timezone": BeijingTimezone,
		"boundary_policy": BoundaryLastAtOrBefore, "max_staleness_sec": MaxStalenessSeconds,
		"start_time_sec": start.Unix(), "end_time_sec": end.Unix(),
	}
}

func orderedOperator(seed int) (string, string) {
	if seed%2 == 1 {
		return "LTE", "小于等于"
	}
	return "GTE", "大于等于"
}

func numericParam(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func parseRange(value string) (float64, float64) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return 0, 0
	}
	return numericParam(parts[0]), numericParam(parts[1])
}

func nonNegativeSeed(seed int) int {
	if seed == math.MinInt {
		return math.MaxInt
	}
	if seed < 0 {
		return -seed
	}
	return seed
}
