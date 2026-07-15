package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadReadsSimulatorConfig(t *testing.T) {
	path := writeConfig(t, `runtime:
  enabled: true
  mode: "preview"
  continuous: false
  on_chain: false
  dry_run: true
  plan_file: "out/custom-plan.json"
  regenerate_plan_on_execute: true
  approve_on_chain: false
chain:
  private_key: "replace-with-private-key"
  contract_address: "0xad4F9eD0F2b51A26314C9f83DF588cCcE26ae03c"
  rpc_url: "http://127.0.0.1:42515"
  broker_chain_url: "http://127.0.0.1:56741/"
  use_broker_chain: false
mysql:
  dsn: "root:secret@tcp(127.0.0.1:3306)/predictionmarket_local?parseTime=true"
ipfs:
  upload_url: "http://127.0.0.1:8081/api/v1/ipfs/add"
scenario:
  type: "create_and_trade"
  market_count: 2
  existing_game_ids: [7]
  participants: 5
  trades_per_market_min: 3
  trades_per_market_max: 6
market:
  types: ["TYPE_PRICE", "TYPE_PRICE_RANGE"]
  initial_liquidity_min_bkc: "3"
  initial_liquidity_max_bkc: "9"
  duration_min_seconds: 86400
  duration_max_seconds: 172800
trade:
  buy_min_bkc: "0.2"
  buy_max_bkc: "2"
  creator_also_trades: true
timing:
  pause_seconds: 0.5
  trade_interval_min_seconds: 2
  trade_interval_max_seconds: 8
  cycle_interval_min_seconds: 30
  cycle_interval_max_seconds: 90
  timeout_seconds: 90
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Runtime.Enabled || cfg.Runtime.OnChain || !cfg.Runtime.DryRun {
		t.Fatalf("unexpected runtime config: %+v", cfg.Runtime)
	}
	if cfg.Runtime.Mode != ModePreview || cfg.Runtime.PlanFile != "out/custom-plan.json" ||
		!cfg.Runtime.RegeneratePlanOnExecute || cfg.Runtime.ApproveOnChain {
		t.Fatalf("unexpected preview runtime config: %+v", cfg.Runtime)
	}
	if cfg.Scenario.Type != ScenarioCreateAndTrade || cfg.Scenario.MarketCount != 2 || cfg.Scenario.Participants != 5 {
		t.Fatalf("unexpected scenario config: %+v", cfg.Scenario)
	}
	if cfg.Timing.Pause != 500*time.Millisecond || cfg.Timing.TradeIntervalMin != 2*time.Second ||
		cfg.Timing.TradeIntervalMax != 8*time.Second || cfg.Timing.CycleIntervalMin != 30*time.Second ||
		cfg.Timing.CycleIntervalMax != 90*time.Second || cfg.Timing.Timeout != 90*time.Second {
		t.Fatalf("unexpected timing config: %+v", cfg.Timing)
	}
	if len(cfg.Market.Types) != 2 || cfg.Market.Types[1] != "TYPE_PRICE_RANGE" {
		t.Fatalf("unexpected market types: %#v", cfg.Market.Types)
	}
	if cfg.IPFS.UploadURL != "http://127.0.0.1:8081/api/v1/ipfs/add" {
		t.Fatalf("unexpected IPFS upload URL: %q", cfg.IPFS.UploadURL)
	}
}

func TestLoadAppliesReasonableDefaults(t *testing.T) {
	path := writeConfig(t, `runtime:
  enabled: true
chain:
  contract_address: "0xad4F9eD0F2b51A26314C9f83DF588cCcE26ae03c"
  rpc_url: "http://127.0.0.1:42515"
mysql:
  dsn: "root:secret@tcp(127.0.0.1:3306)/predictionmarket_local?parseTime=true"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Scenario.Type != ScenarioCreateAndTrade {
		t.Fatalf("scenario type = %q, want %q", cfg.Scenario.Type, ScenarioCreateAndTrade)
	}
	if cfg.Scenario.MarketCount != 1 || cfg.Scenario.Participants != 6 {
		t.Fatalf("unexpected default scenario: %+v", cfg.Scenario)
	}
	if cfg.Market.InitialLiquidityMinBKC != "3" || cfg.Market.InitialLiquidityMaxBKC != "12" {
		t.Fatalf("unexpected default market amounts: %+v", cfg.Market)
	}
	if len(cfg.Market.Types) != 6 {
		t.Fatalf("default market types = %d, want 6", len(cfg.Market.Types))
	}
	if cfg.Market.DurationMin != 24*time.Hour || cfg.Market.DurationMax != 4*24*time.Hour {
		t.Fatalf("unexpected default market duration: %+v", cfg.Market)
	}
	if cfg.Runtime.Mode != ModeExecute {
		t.Fatalf("default runtime mode = %q, want %q", cfg.Runtime.Mode, ModeExecute)
	}
	if cfg.Runtime.PlanFile == "" {
		t.Fatal("default runtime plan file is empty")
	}
}

func TestLoadRejectsSubDayMarketDuration(t *testing.T) {
	path := writeConfig(t, `runtime:
  enabled: true
  mode: "preview"
chain:
  contract_address: "0xad4F9eD0F2b51A26314C9f83DF588cCcE26ae03c"
mysql:
  dsn: ""
market:
  duration_min_seconds: 3600
  duration_max_seconds: 86400
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected sub-day duration to be rejected")
	}
}

func TestLoadPreviewModeDoesNotRequireExecutionCredentials(t *testing.T) {
	path := writeConfig(t, `runtime:
  enabled: true
  mode: "preview"
  on_chain: true
  plan_file: "out/preview.json"
chain:
  private_key: "replace-with-funded-private-key"
  contract_address: "0xad4F9eD0F2b51A26314C9f83DF588cCcE26ae03c"
  rpc_url: ""
mysql:
  dsn: ""
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runtime.Mode != ModePreview || !cfg.Runtime.OnChain {
		t.Fatalf("unexpected runtime config: %+v", cfg.Runtime)
	}
}

func TestLoadRejectsExecuteOnChainWithoutApproval(t *testing.T) {
	path := writeConfig(t, `runtime:
  enabled: true
  mode: "execute"
  on_chain: true
  plan_file: "out/preview.json"
  approve_on_chain: false
chain:
  private_key: "abc123"
  contract_address: "0xad4F9eD0F2b51A26314C9f83DF588cCcE26ae03c"
  rpc_url: "http://127.0.0.1:8545"
mysql:
  dsn: "root:secret@tcp(127.0.0.1:3306)/predictionmarket_local?parseTime=true"
`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load succeeded, want on-chain approval error")
	}
}

func TestLoadRejectsUnknownMarketType(t *testing.T) {
	path := writeConfig(t, `runtime:
  enabled: true
chain:
  contract_address: "0xad4F9eD0F2b51A26314C9f83DF588cCcE26ae03c"
  rpc_url: "http://127.0.0.1:42515"
mysql:
  dsn: "root:secret@tcp(127.0.0.1:3306)/predictionmarket_local?parseTime=true"
market:
  types: ["TYPE_NOT_REAL"]
`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load succeeded, want unknown type error")
	}
}

func TestLoadAllowsOffchainExistingTrades(t *testing.T) {
	path := writeConfig(t, `runtime:
  enabled: true
  on_chain: false
  dry_run: false
chain:
  contract_address: "0xad4F9eD0F2b51A26314C9f83DF588cCcE26ae03c"
mysql:
  dsn: "root:secret@tcp(127.0.0.1:3306)/predictionmarket_local?parseTime=true"
scenario:
  type: "trade_existing"
  existing_game_ids: [1]
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Scenario.Type != ScenarioTradeExisting || cfg.Runtime.OnChain {
		t.Fatalf("unexpected config: runtime=%+v scenario=%+v", cfg.Runtime, cfg.Scenario)
	}
}

func TestLoadRejectsReversedAmountRange(t *testing.T) {
	path := writeConfig(t, `runtime:
  enabled: true
chain:
  contract_address: "0xad4F9eD0F2b51A26314C9f83DF588cCcE26ae03c"
mysql:
  dsn: "root:secret@tcp(127.0.0.1:3306)/predictionmarket_local?parseTime=true"
trade:
  buy_min_bkc: "3"
  buy_max_bkc: "1"
`)
	if _, err := Load(path); err == nil {
		t.Fatal("Load succeeded, want reversed amount range error")
	}
}

func TestLoadReadsContinuousRuntimeMode(t *testing.T) {
	path := writeConfig(t, `runtime:
  enabled: true
  mode: "execute"
  continuous: true
  regenerate_plan_on_execute: true
chain:
  contract_address: "0xad4F9eD0F2b51A26314C9f83DF588cCcE26ae03c"
mysql:
  dsn: "root:secret@tcp(127.0.0.1:3306)/predictionmarket_local?parseTime=true"
timing:
  trade_interval_min_seconds: 5
  trade_interval_max_seconds: 15
  cycle_interval_min_seconds: 60
  cycle_interval_max_seconds: 120
  timeout_seconds: 600
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Runtime.Continuous {
		t.Fatal("runtime.continuous = false, want true")
	}
	if cfg.Timing.TradeIntervalMin != 5*time.Second || cfg.Timing.TradeIntervalMax != 15*time.Second {
		t.Fatalf("unexpected trade interval: %+v", cfg.Timing)
	}
	if cfg.Timing.CycleIntervalMin != time.Minute || cfg.Timing.CycleIntervalMax != 2*time.Minute {
		t.Fatalf("unexpected cycle interval: %+v", cfg.Timing)
	}
}

func TestLoadRejectsContinuousPreviewMode(t *testing.T) {
	path := writeConfig(t, `runtime:
  enabled: true
  mode: "preview"
  continuous: true
chain:
  contract_address: "0xad4F9eD0F2b51A26314C9f83DF588cCcE26ae03c"
`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load succeeded, want continuous preview validation error")
	}
}

func TestLoadRejectsReversedContinuousIntervals(t *testing.T) {
	path := writeConfig(t, `runtime:
  enabled: true
  mode: "execute"
  continuous: true
  regenerate_plan_on_execute: true
chain:
  contract_address: "0xad4F9eD0F2b51A26314C9f83DF588cCcE26ae03c"
mysql:
  dsn: "root:secret@tcp(127.0.0.1:3306)/predictionmarket_local?parseTime=true"
timing:
  trade_interval_min_seconds: 20
  trade_interval_max_seconds: 10
`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load succeeded, want reversed trade interval error")
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
