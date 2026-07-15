package scenario

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestSupportedTypesContainOnlyResolvableVersion2Templates(t *testing.T) {
	want := []string{
		TypePrice, TypeReturnThreshold, TypePriceThreshold,
		TypePriceRange, TypeRelative, TypeStreak,
	}
	if !reflect.DeepEqual(SupportedTypes(), want) {
		t.Fatalf("SupportedTypes()=%v, want %v", SupportedTypes(), want)
	}
}

func TestEveryGeneratedMarketCommitsVersion2ChainlinkRule(t *testing.T) {
	fixtureNow := time.Date(2026, time.July, 13, 7, 0, 0, 0, time.UTC)
	for index, typ := range SupportedTypes() {
		t.Run(typ, func(t *testing.T) {
			market, err := BuildMarket(BuildMarketInput{
				Type: typ, Creator: "0x0000000000000000000000000000000000000001",
				Duration: 48 * time.Hour, InitialBKC: "5", Now: fixtureNow, TemplateSeed: index,
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
			rule := metadata.ResolutionRule
			if rule["rule_version"] != float64(2) || rule["source"] != ChainlinkSource ||
				rule["timezone"] != BeijingTimezone || rule["boundary_policy"] != BoundaryLastAtOrBefore ||
				rule["source_contract"] != ChainlinkXAUUSDFeed {
				t.Fatalf("metadata=%s", market.MetadataJSON)
			}
			start := time.Unix(int64(rule["start_time_sec"].(float64)), 0)
			end := time.Unix(int64(rule["end_time_sec"].(float64)), 0)
			if !IsValidBeijingBoundary(start) || !IsValidBeijingBoundary(end) || !end.After(start) {
				t.Fatalf("invalid boundaries %s -> %s", start, end)
			}
			if duration := end.Sub(start); duration < 24*time.Hour || duration > 4*24*time.Hour {
				t.Fatalf("duration=%s, want 1-4 whole days", duration)
			}
		})
	}
}
