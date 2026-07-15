package cli

import (
	"bytes"
	"reflect"
	"testing"

	"predictionmarket-simulator/internal/config"
)

func TestParseScenarioChoice(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		current string
		want    string
	}{
		{name: "trade existing by number", raw: "1", want: config.ScenarioTradeExisting},
		{name: "create by number", raw: "2", want: config.ScenarioCreateAndTrade},
		{name: "trade by alias", raw: "buy_existing", want: config.ScenarioTradeExisting},
		{name: "blank keeps current", raw: "", current: config.ScenarioCreateAndTrade, want: config.ScenarioCreateAndTrade},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseScenarioChoice(tt.raw, tt.current)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("scenario = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseGameIDs(t *testing.T) {
	got, err := parseGameIDs("[1, 2 3]")
	if err != nil {
		t.Fatal(err)
	}
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %#v, want %#v", got, want)
	}
}

func TestPromptScenarioOverridesConfig(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeConfig{
			Mode: config.ModePreview,
		},
		Scenario: config.ScenarioConfig{
			Type:            config.ScenarioCreateAndTrade,
			MarketCount:     3,
			ExistingGameIDs: []int{9},
		},
	}

	input := bytes.NewBufferString("1\n4,5\n")
	var output bytes.Buffer
	if err := promptScenario(input, &output, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Scenario.Type != config.ScenarioTradeExisting {
		t.Fatalf("scenario = %q, want %q", cfg.Scenario.Type, config.ScenarioTradeExisting)
	}
	wantIDs := []int{4, 5}
	if !reflect.DeepEqual(cfg.Scenario.ExistingGameIDs, wantIDs) {
		t.Fatalf("existing ids = %#v, want %#v", cfg.Scenario.ExistingGameIDs, wantIDs)
	}
}

func TestPromptScenarioConfiguresExecuteMode(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeConfig{
			Mode:                    config.ModeExecute,
			RegeneratePlanOnExecute: false,
		},
		Scenario: config.ScenarioConfig{
			Type:        config.ScenarioCreateAndTrade,
			MarketCount: 3,
		},
	}

	input := bytes.NewBufferString("2\n3\n")
	var output bytes.Buffer
	if err := promptScenario(input, &output, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Scenario.Type != config.ScenarioCreateAndTrade || cfg.Scenario.MarketCount != 3 {
		t.Fatalf("unexpected scenario: %+v", cfg.Scenario)
	}
	if !cfg.Runtime.RegeneratePlanOnExecute {
		t.Fatal("execute selection did not enable plan regeneration")
	}
}

func TestShouldPromptIncludesExecuteModeForRootCommand(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeConfig{
			Mode: config.ModeExecute,
		},
	}
	if !shouldPrompt(true, false, cfg) {
		t.Fatal("shouldPrompt returned false for root command in execute mode")
	}
	if shouldPrompt(false, false, cfg) {
		t.Fatal("shouldPrompt returned true for non-interactive command")
	}
}

func TestApplyScenarioOptionsSelectsExistingPools(t *testing.T) {
	cfg := &config.Config{
		Runtime:  config.RuntimeConfig{Mode: config.ModeExecute},
		Scenario: config.ScenarioConfig{Type: config.ScenarioCreateAndTrade, MarketCount: 3},
	}

	err := applyScenarioOptions(cfg, scenarioOptions{scenario: "existing", gameIDs: "7,9"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Scenario.Type != config.ScenarioTradeExisting {
		t.Fatalf("scenario = %q, want trade_existing", cfg.Scenario.Type)
	}
	wantIDs := []int{7, 9}
	if !reflect.DeepEqual(cfg.Scenario.ExistingGameIDs, wantIDs) {
		t.Fatalf("existing IDs = %#v, want %#v", cfg.Scenario.ExistingGameIDs, wantIDs)
	}
	if !cfg.Runtime.RegeneratePlanOnExecute {
		t.Fatal("scenario option did not enable plan regeneration")
	}
}

func TestApplyScenarioOptionsSelectsNewPools(t *testing.T) {
	cfg := &config.Config{
		Runtime:  config.RuntimeConfig{Mode: config.ModeExecute},
		Scenario: config.ScenarioConfig{Type: config.ScenarioTradeExisting, ExistingGameIDs: []int{7}},
	}

	err := applyScenarioOptions(cfg, scenarioOptions{scenario: "create", marketCount: 5})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Scenario.Type != config.ScenarioCreateAndTrade || cfg.Scenario.MarketCount != 5 {
		t.Fatalf("unexpected scenario: %+v", cfg.Scenario)
	}
	if len(cfg.Scenario.ExistingGameIDs) != 0 {
		t.Fatalf("existing IDs were not cleared: %#v", cfg.Scenario.ExistingGameIDs)
	}
}
