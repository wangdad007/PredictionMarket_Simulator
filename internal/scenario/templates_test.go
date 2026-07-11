package scenario

import (
	"strings"
	"testing"
	"time"
)

func TestAllCurrentProjectMarketTypesAreSupported(t *testing.T) {
	want := []string{
		TypePrice,
		TypeVolatility,
		TypeVolume,
		TypeTechnical,
		TypeTouch,
		TypeRelative,
		TypePriceThreshold,
		TypeEvent,
	}
	if len(SupportedTypes()) != len(want) {
		t.Fatalf("SupportedTypes length = %d, want %d", len(SupportedTypes()), len(want))
	}
	for _, typ := range want {
		if _, ok := TemplateForType(typ); !ok {
			t.Fatalf("TemplateForType(%q) not supported", typ)
		}
	}
}

func TestBuildMarketProducesTypedMetadata(t *testing.T) {
	market, err := BuildMarket(BuildMarketInput{
		Type:         TypePriceThreshold,
		Index:        3,
		Creator:      "0x0000000000000000000000000000000000000001",
		Duration:     2 * time.Hour,
		InitialBKC:   "4.5",
		Now:          time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		TemplateSeed: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if market.Type != TypePriceThreshold {
		t.Fatalf("market type = %q", market.Type)
	}
	if !strings.Contains(market.Desc, "USD") || !strings.Contains(market.Condition, "USD") {
		t.Fatalf("price threshold metadata did not include USD: %+v", market)
	}
	if market.OptionYes == "" || market.OptionNo == "" || market.CreatorAddress == "" {
		t.Fatalf("missing display metadata: %+v", market)
	}
	if !strings.Contains(market.MetadataJSON, `"type":"TYPE_PRICE_THRESHOLD"`) {
		t.Fatalf("metadata JSON missing type: %s", market.MetadataJSON)
	}
	if !strings.HasPrefix(market.IPFSCID, "sim-") {
		t.Fatalf("simulated CID = %q, want sim-*", market.IPFSCID)
	}
}

func TestBuildMarketMatchesFrontendTitleConventions(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	end := "2026-07-10 12:00"
	tests := []struct {
		typ           string
		seed          int
		wantDesc      string
		wantCondition string
	}{
		{
			typ:           TypePrice,
			seed:          0,
			wantDesc:      "2026-07-09 12:00 至 " + end + " 黄金价格 上涨",
			wantCondition: "黄金价格在 从 2026-07-09 12:00 到 " + end + " 相对基准 上涨 (Price Up)",
		},
		{
			typ:           TypeVolatility,
			seed:          0,
			wantDesc:      end + " 前黄金波幅超过 1.5%",
			wantCondition: "周期内波幅 >= 1.5% (截至 " + end + ")",
		},
		{
			typ:           TypeVolume,
			seed:          0,
			wantDesc:      end + " 当日成交量 大于 300 吨",
			wantCondition: "指定日成交量 大于 300 吨 (" + end + ")",
		},
		{
			typ:           TypeTechnical,
			seed:          0,
			wantDesc:      "指标 RSI (14) 触发 大于 (Above) 70",
			wantCondition: "指标 RSI (14) 大于 (Above) 70 (截至 " + end + ")",
		},
		{
			typ:           TypeTouch,
			seed:          0,
			wantDesc:      "周期内金价触及 2500 USD",
			wantCondition: "金价曾触及 2500 USD (截至 " + end + ")",
		},
		{
			typ:           TypeRelative,
			seed:          0,
			wantDesc:      "黄金收益率跑赢 BTC",
			wantCondition: "黄金收益率跑赢 BTC (截至 " + end + ")",
		},
		{
			typ:           TypePriceThreshold,
			seed:          0,
			wantDesc:      "截止 " + end + " 金价 大于 2800 USD",
			wantCondition: "黄金价格 大于 2800 USD (截至 " + end + ")",
		},
		{
			typ:           TypeEvent,
			seed:          0,
			wantDesc:      "「美联储降息」是否发生",
			wantCondition: "事件「美联储降息」是否发生 (截至 " + end + ")",
		},
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			market, err := BuildMarket(BuildMarketInput{
				Type:         tt.typ,
				Creator:      "0x0000000000000000000000000000000000000001",
				Duration:     24 * time.Hour,
				InitialBKC:   "4.5",
				Now:          now,
				TemplateSeed: tt.seed,
			})
			if err != nil {
				t.Fatal(err)
			}
			if market.Desc != tt.wantDesc {
				t.Fatalf("desc = %q, want %q", market.Desc, tt.wantDesc)
			}
			if market.Condition != tt.wantCondition {
				t.Fatalf("condition = %q, want %q", market.Condition, tt.wantCondition)
			}
			if market.OptionYes != "YES" || market.OptionNo != "NO" {
				t.Fatalf("options = %q/%q, want YES/NO", market.OptionYes, market.OptionNo)
			}
		})
	}
}
