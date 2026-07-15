package scenario

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRelativeMarketCommitsBTCBenchmarkFeed(t *testing.T) {
	market, err := BuildMarket(BuildMarketInput{
		Type: TypeRelative, Creator: "0x0000000000000000000000000000000000000001",
		Duration: 48 * time.Hour, InitialBKC: "5",
		Now: time.Date(2026, 7, 13, 7, 12, 0, 0, time.UTC), TemplateSeed: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		ResolutionRule map[string]any `json:"resolutionRule"`
	}
	if err := json.Unmarshal([]byte(market.MetadataJSON), &metadata); err != nil {
		t.Fatal(err)
	}
	if got := metadata.ResolutionRule["benchmark_source_contract"]; got != ChainlinkBTCUSDFeed {
		t.Fatalf("benchmark_source_contract=%v", got)
	}
}

func TestRelativeMarketRotatesThroughSupportedBenchmarkFeeds(t *testing.T) {
	want := []struct {
		symbol string
		feed   string
	}{
		{"BTC", ChainlinkBTCUSDFeed},
		{"ETH", ChainlinkETHUSDFeed},
		{"SOL", ChainlinkSOLUSDFeed},
		{"BNB", ChainlinkBNBUSDFeed},
	}
	for seed, expected := range want {
		market, err := BuildMarket(BuildMarketInput{
			Type: TypeRelative, Creator: "0x0000000000000000000000000000000000000001",
			Duration: 48 * time.Hour, InitialBKC: "5",
			Now: time.Date(2026, 7, 13, 7, 12, 0, 0, time.UTC), TemplateSeed: seed,
		})
		if err != nil {
			t.Fatal(err)
		}
		var metadata struct {
			ResolutionRule map[string]any `json:"resolutionRule"`
		}
		if err := json.Unmarshal([]byte(market.MetadataJSON), &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata.ResolutionRule["benchmark"] != expected.symbol ||
			metadata.ResolutionRule["benchmark_source_contract"] != expected.feed {
			t.Fatalf("seed=%d rule=%v", seed, metadata.ResolutionRule)
		}
	}
}

func TestBuildMarketProducesTypedMetadataAndFutureDeadline(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	market, err := BuildMarket(BuildMarketInput{
		Type: TypePriceThreshold, Index: 3,
		Creator:  "0x0000000000000000000000000000000000000001",
		Duration: 48 * time.Hour, InitialBKC: "4.5", Now: now, TemplateSeed: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if market.Type != TypePriceThreshold || market.DurationSeconds <= int64((48*time.Hour)/time.Second) {
		t.Fatalf("market=%+v", market)
	}
	for _, expected := range []string{"USD/盎司", `"type":"TYPE_PRICE_THRESHOLD"`, `"rule_version":2`} {
		if !strings.Contains(market.MetadataJSON+market.Desc+market.Condition, expected) {
			t.Fatalf("market missing %q: %+v", expected, market)
		}
	}
	if market.AvatarURL != "template://TYPE_PRICE_THRESHOLD" || !strings.HasPrefix(market.IPFSCID, "sim-") {
		t.Fatalf("market=%+v", market)
	}
}

func TestBuildMarketMatchesCanonicalTitles(t *testing.T) {
	now := time.Date(2026, 7, 13, 7, 0, 0, 0, time.UTC)
	tests := []struct {
		typ  string
		seed int
		want string
	}{
		{TypePrice, 0, "黄金价格 上涨 2天"},
		{TypeReturnThreshold, 2, "黄金涨跌幅 大于等于 3%"},
		{TypePriceThreshold, 2, "黄金价格 大于等于 4300USD/盎司"},
		{TypePriceRange, 0, "黄金价格 位于 3800-4000USD/盎司"},
		{TypeRelative, 0, "黄金 跑赢 BTC"},
		{TypeStreak, 0, "黄金价格 连续上涨 2天"},
	}
	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			market, err := BuildMarket(BuildMarketInput{
				Type: tt.typ, Creator: "0x0000000000000000000000000000000000000001",
				Duration: 48 * time.Hour, InitialBKC: "4.5", Now: now, TemplateSeed: tt.seed,
			})
			if err != nil {
				t.Fatal(err)
			}
			if market.Desc != tt.want {
				t.Fatalf("desc=%q, want %q", market.Desc, tt.want)
			}
			if market.OptionYes != "YES" || market.OptionNo != "NO" || !strings.Contains(market.Condition, "北京时间") {
				t.Fatalf("market=%+v", market)
			}
		})
	}
}
