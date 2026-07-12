package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	TypePrice          = "TYPE_PRICE"
	TypeVolatility     = "TYPE_VOLATILITY"
	TypeVolume         = "TYPE_VOLUME"
	TypeTechnical      = "TYPE_TECHNICAL"
	TypeTouch          = "TYPE_TOUCH"
	TypeRelative       = "TYPE_RELATIVE"
	TypePriceThreshold = "TYPE_PRICE_THRESHOLD"
	TypeEvent          = "TYPE_EVENT"
)

type Template struct {
	Type         string
	Title        string
	OptionYes    string
	OptionNo     string
	Params       []string
	BuildDesc    func(param string, start time.Time, end time.Time, seed int) string
	BuildCond    func(param string, start time.Time, end time.Time, seed int) string
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
		Type:      TypePrice,
		Title:     "黄金价格方向预测",
		OptionYes: "YES",
		OptionNo:  "NO",
		Params:    []string{"上涨", "下跌", "持平"},
		BuildDesc: func(param string, start time.Time, end time.Time, _ int) string {
			return fmt.Sprintf("黄金价格 %s %s", param, durationDays(start, end))
		},
		BuildCond: func(param string, start time.Time, end time.Time, _ int) string {
			return fmt.Sprintf("黄金价格在 从 %s 到 %s 相对基准 %s", formatTime(start), formatTime(end), priceDirectionLabel(param))
		},
		DetailedInfo: "模拟用户创建的方向类黄金预测池。",
	},
	TypeVolatility: {
		Type:      TypeVolatility,
		Title:     "黄金波动率预测",
		OptionYes: "YES",
		OptionNo:  "NO",
		Params:    []string{"1.5", "3", "5"},
		BuildDesc: func(param string, _ time.Time, end time.Time, _ int) string {
			return fmt.Sprintf("黄金波动 大于 %s%%", param)
		},
		BuildCond: func(param string, _ time.Time, end time.Time, _ int) string {
			return fmt.Sprintf("周期内波幅 >= %s%% (截至 %s)", param, formatTime(end))
		},
		DetailedInfo: "模拟用户创建的波动率类黄金预测池。",
	},
	TypeVolume: {
		Type:      TypeVolume,
		Title:     "黄金交易量预测",
		OptionYes: "YES",
		OptionNo:  "NO",
		Params:    []string{"300", "500", "800"},
		BuildDesc: func(param string, _ time.Time, end time.Time, seed int) string {
			return fmt.Sprintf("黄金成交量 %s %s吨", operatorForSeed(seed), param)
		},
		BuildCond: func(param string, _ time.Time, end time.Time, seed int) string {
			return fmt.Sprintf("指定日成交量 %s %s 吨 (%s)", operatorForSeed(seed), param, formatTime(end))
		},
		DetailedInfo: "模拟用户创建的交易量类黄金预测池。",
	},
	TypeTechnical: {
		Type:      TypeTechnical,
		Title:     "黄金技术指标预测",
		OptionYes: "YES",
		OptionNo:  "NO",
		Params: []string{
			"RSI (14) 触发 大于 (Above) 70",
			"MACD (12,26,9) 触发 交叉向上 (Cross Up) 0",
			"KDJ (9,3,3) 触发 交叉向下 (Cross Down) 0",
			"BOLL (20,2) 触发 小于 (Below) 0",
		},
		BuildDesc: func(param string, _ time.Time, _ time.Time, _ int) string {
			return technicalDescription(param)
		},
		BuildCond: func(param string, _ time.Time, end time.Time, _ int) string {
			return fmt.Sprintf("指标 %s (截至 %s)", strings.Replace(param, "触发 ", "", 1), formatTime(end))
		},
		DetailedInfo: "模拟用户创建的技术指标类黄金预测池。",
	},
	TypeTouch: {
		Type:      TypeTouch,
		Title:     "黄金触价预测",
		OptionYes: "YES",
		OptionNo:  "NO",
		Params:    []string{"2500", "2800", "3000"},
		BuildDesc: func(param string, _ time.Time, _ time.Time, _ int) string {
			return fmt.Sprintf("黄金价格 触及 %sUSD/盎司", param)
		},
		BuildCond: func(param string, _ time.Time, end time.Time, _ int) string {
			return fmt.Sprintf("金价曾触及 %s USD/盎司 (截至 %s)", param, formatTime(end))
		},
		DetailedInfo: "模拟用户创建的触价类黄金预测池。",
	},
	TypeRelative: {
		Type:      TypeRelative,
		Title:     "黄金相对表现预测",
		OptionYes: "YES",
		OptionNo:  "NO",
		Params:    []string{"比特币", "标普500", "白银"},
		BuildDesc: func(param string, _ time.Time, _ time.Time, _ int) string {
			return fmt.Sprintf("黄金 跑赢 %s", benchmarkShortName(param))
		},
		BuildCond: func(param string, _ time.Time, end time.Time, _ int) string {
			return fmt.Sprintf("黄金收益率跑赢 %s (截至 %s)", param, formatTime(end))
		},
		DetailedInfo: "模拟用户创建的相对表现类黄金预测池。",
	},
	TypePriceThreshold: {
		Type:      TypePriceThreshold,
		Title:     "黄金价格阈值预测",
		OptionYes: "YES",
		OptionNo:  "NO",
		Params:    []string{"2800", "3000", "3200"},
		BuildDesc: func(param string, _ time.Time, end time.Time, seed int) string {
			return fmt.Sprintf("黄金价格 %s %sUSD/盎司", operatorForSeed(seed), param)
		},
		BuildCond: func(param string, _ time.Time, end time.Time, seed int) string {
			return fmt.Sprintf("黄金价格 %s %s USD/盎司 (截至 %s)", operatorForSeed(seed), param, formatTime(end))
		},
		DetailedInfo: "模拟用户创建的到期价格阈值类黄金预测池。",
	},
	TypeEvent: {
		Type:      TypeEvent,
		Title:     "宏观事件预测",
		OptionYes: "YES",
		OptionNo:  "NO",
		Params:    []string{"美联储降息", "CPI 低于预期", "美元指数大幅回落"},
		BuildDesc: func(param string, _ time.Time, _ time.Time, _ int) string {
			return fmt.Sprintf("发生 %s", param)
		},
		BuildCond: func(param string, _ time.Time, end time.Time, _ int) string {
			return fmt.Sprintf("事件「%s」是否发生 (截至 %s)", param, formatTime(end))
		},
		DetailedInfo: "模拟用户创建的宏观事件类黄金预测池。",
	},
}

func SupportedTypes() []string {
	return []string{
		TypePrice,
		TypeVolatility,
		TypeVolume,
		TypeTechnical,
		TypeTouch,
		TypeRelative,
		TypePriceThreshold,
		TypeEvent,
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
		input.Now = time.Now().UTC()
	}
	if input.Duration <= 0 {
		return nil, fmt.Errorf("duration must be positive")
	}
	param := tpl.Params[input.TemplateSeed%len(tpl.Params)]
	end := input.Now.Add(input.Duration)
	desc := tpl.BuildDesc(param, input.Now, end, input.TemplateSeed)
	condition := tpl.BuildCond(param, input.Now, end, input.TemplateSeed)
	avatarURL := "template://" + tpl.Type
	metadata := map[string]any{
		"type":           tpl.Type,
		"desc":           desc,
		"condition":      condition,
		"avatarUrl":      avatarURL,
		"detailedInfo":   tpl.DetailedInfo,
		"optionYES":      tpl.OptionYes,
		"optionNO":       tpl.OptionNo,
		"creator":        input.Creator,
		"initialBKC":     input.InitialBKC,
		"durationSec":    int64(input.Duration / time.Second),
		"templateParam":  param,
		"resolutionRule": buildResolutionRule(tpl.Type, param, input.Now, end, input.TemplateSeed),
	}
	if sources := authoritativeSources(tpl.Type, param); len(sources) > 0 {
		metadata["authoritativeSources"] = sources
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return &Market{
		Type:             tpl.Type,
		IPFSCID:          "sim-" + hex.EncodeToString(sum[:])[:32],
		Desc:             desc,
		Condition:        condition,
		AvatarURL:        avatarURL,
		DetailedInfo:     tpl.DetailedInfo,
		OptionYes:        tpl.OptionYes,
		OptionNo:         tpl.OptionNo,
		CreatorAddress:   input.Creator,
		DurationSeconds:  int64(input.Duration / time.Second),
		InitialLiquidity: input.InitialBKC,
		MetadataJSON:     string(raw),
	}, nil
}

func authoritativeSources(typ, param string) []string {
	if typ == TypeEvent {
		if strings.Contains(param, "美联储") {
			return []string{"https://www.federalreserve.gov/newsevents/pressreleases.htm"}
		}
		if strings.Contains(strings.ToUpper(param), "CPI") {
			return []string{"https://www.bls.gov/cpi/"}
		}
	}
	return nil
}

func buildResolutionRule(typ, param string, start, end time.Time, seed int) map[string]any {
	rule := map[string]any{
		"type": typ, "symbol": "XAU", "source": "GOLD_API",
		"start_time_sec": start.Unix(), "end_time_sec": end.Unix(),
	}
	switch typ {
	case TypePrice:
		rule["direction"] = map[string]string{"上涨": "UP", "下跌": "DOWN", "持平": "FLAT"}[param]
		rule["flat_tolerance_percent"] = 0.1
	case TypeVolatility:
		rule["operator"], rule["threshold"] = "GTE", numericParam(param)
	case TypeVolume:
		rule["symbol"], rule["source"] = "COMEX_GC", "CME_GROUP"
		rule["operator"], rule["threshold"] = canonicalOperator(seed), numericParam(param)
		rule["volume_unit"] = "METRIC_TON_EQUIVALENT"
	case TypeTechnical:
		rule["indicator"] = strings.Fields(param)[0]
		rule["operator"] = technicalOperator(param)
		rule["interval"] = "hour"
	case TypeTouch:
		rule["threshold"] = numericParam(param)
	case TypeRelative:
		rule["benchmark"] = benchmarkShortName(param)
	case TypePriceThreshold:
		rule["operator"], rule["threshold"] = canonicalOperator(seed), numericParam(param)
	case TypeEvent:
		rule["source"] = "AUTHORITATIVE_DOCUMENTS"
		rule["event"] = param
		if sources := authoritativeSources(typ, param); len(sources) > 0 {
			rule["authoritative_sources"] = sources
		}
	}
	return rule
}

func numericParam(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func canonicalOperator(seed int) string {
	switch seed % 3 {
	case 1:
		return "LT"
	case 2:
		return "EQ"
	default:
		return "GT"
	}
}

func technicalOperator(value string) string {
	switch {
	case strings.Contains(value, "交叉向上"):
		return "CROSS_UP"
	case strings.Contains(value, "交叉向下"):
		return "CROSS_DOWN"
	case strings.Contains(value, "小于"):
		return "LT"
	default:
		return "GT"
	}
}

func operatorForSeed(seed int) string {
	switch seed % 3 {
	case 1:
		return "小于"
	case 2:
		return "等于"
	default:
		return "大于"
	}
}

func formatTime(value time.Time) string {
	return value.Format("2006-01-02 15:04")
}

func durationDays(start time.Time, end time.Time) string {
	hours := int64(end.Sub(start).Hours())
	if hours <= 0 {
		return ""
	}
	days := (hours + 23) / 24
	return fmt.Sprintf("%d天", days)
}

func benchmarkShortName(value string) string {
	switch value {
	case "比特币", "Bitcoin":
		return "BTC"
	default:
		return value
	}
}

func technicalDescription(value string) string {
	switch {
	case strings.HasPrefix(value, "RSI"):
		return "黄金RSI 大于 70"
	case strings.HasPrefix(value, "MACD"):
		return "黄金MACD 交叉向上"
	case strings.HasPrefix(value, "KDJ"):
		return "黄金KDJ 交叉向下"
	case strings.HasPrefix(value, "BOLL"):
		return "黄金BOLL 小于 0"
	default:
		return "黄金指标"
	}
}

func priceDirectionLabel(direction string) string {
	switch direction {
	case "上涨":
		return "上涨 (Price Up)"
	case "下跌":
		return "下跌 (Price Down)"
	case "持平":
		return "持平 (Flat/Range)"
	default:
		return direction
	}
}
