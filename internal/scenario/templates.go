package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
			return fmt.Sprintf("%s 至 %s 黄金价格 %s", formatTime(start), formatTime(end), param)
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
			return fmt.Sprintf("%s 前黄金波幅超过 %s%%", formatTime(end), param)
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
			return fmt.Sprintf("%s 当日成交量 %s %s 吨", formatTime(end), operatorForSeed(seed), param)
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
			return "指标 " + param
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
			return fmt.Sprintf("周期内金价触及 %s USD", param)
		},
		BuildCond: func(param string, _ time.Time, end time.Time, _ int) string {
			return fmt.Sprintf("金价曾触及 %s USD (截至 %s)", param, formatTime(end))
		},
		DetailedInfo: "模拟用户创建的触价类黄金预测池。",
	},
	TypeRelative: {
		Type:      TypeRelative,
		Title:     "黄金相对表现预测",
		OptionYes: "YES",
		OptionNo:  "NO",
		Params:    []string{"BTC", "S&P 500", "白银"},
		BuildDesc: func(param string, _ time.Time, _ time.Time, _ int) string {
			return fmt.Sprintf("黄金收益率跑赢 %s", param)
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
			return fmt.Sprintf("截止 %s 金价 %s %s USD", formatTime(end), operatorForSeed(seed), param)
		},
		BuildCond: func(param string, _ time.Time, end time.Time, seed int) string {
			return fmt.Sprintf("黄金价格 %s %s USD (截至 %s)", operatorForSeed(seed), param, formatTime(end))
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
			return fmt.Sprintf("「%s」是否发生", param)
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
	metadata := map[string]any{
		"type":          tpl.Type,
		"desc":          desc,
		"condition":     condition,
		"avatarUrl":     "",
		"detailedInfo":  tpl.DetailedInfo,
		"optionYES":     tpl.OptionYes,
		"optionNO":      tpl.OptionNo,
		"creator":       input.Creator,
		"initialBKC":    input.InitialBKC,
		"durationSec":   int64(input.Duration / time.Second),
		"templateParam": param,
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
		DetailedInfo:     tpl.DetailedInfo,
		OptionYes:        tpl.OptionYes,
		OptionNo:         tpl.OptionNo,
		CreatorAddress:   input.Creator,
		DurationSeconds:  int64(input.Duration / time.Second),
		InitialLiquidity: input.InitialBKC,
		MetadataJSON:     string(raw),
	}, nil
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
