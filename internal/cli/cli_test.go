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
		{name: "create by number", raw: "1", want: config.ScenarioCreateAndTrade},
		{name: "trade by number", raw: "2", want: config.ScenarioTradeExisting},
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

	input := bytes.NewBufferString("2\n4,5\n")
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

func TestPromptScenarioRejectsExecuteMode(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeConfig{
			Mode: config.ModeExecute,
		},
		Scenario: config.ScenarioConfig{
			Type:        config.ScenarioCreateAndTrade,
			MarketCount: 3,
		},
	}

	input := bytes.NewBufferString("1\n3\n")
	var output bytes.Buffer
	if err := promptScenario(input, &output, cfg); err == nil {
		t.Fatal("promptScenario succeeded, want execute mode error")
	}
}

func TestShouldPromptSkipsExecuteModeForDefaultInteractive(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeConfig{
			Mode: config.ModeExecute,
		},
	}
	if shouldPrompt(true, false, cfg) {
		t.Fatal("shouldPrompt returned true for default root command in execute mode")
	}
	if !shouldPrompt(true, true, cfg) {
		t.Fatal("shouldPrompt returned false for explicit interactive flag in execute mode")
	}
}
