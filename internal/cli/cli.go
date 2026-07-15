package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"unicode"

	"predictionmarket-simulator/internal/config"
	"predictionmarket-simulator/internal/sim"
)

// Main runs the simulator command and returns a process-style exit code.
func Main(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer, defaultInteractive bool) int {
	flags := flag.NewFlagSet("simulator", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "config.yaml", "path to simulator config.yaml")
	interactive := flags.Bool("interactive", defaultInteractive, "prompt for generated data type in the terminal")
	scenarioFlag := flags.String("scenario", "", "simulation mode: existing or create")
	gameIDsFlag := flags.String("game-ids", "", "existing game IDs separated by comma")
	marketCountFlag := flags.Int("market-count", 0, "number of new pools to create per round")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "simulator: %v\n", err)
		return 1
	}
	hasScenarioOptions := flagWasProvided(args, "scenario") || flagWasProvided(args, "game-ids") || flagWasProvided(args, "market-count")
	if hasScenarioOptions {
		if err := applyScenarioOptions(cfg, scenarioOptions{
			scenario: *scenarioFlag, gameIDs: *gameIDsFlag, marketCount: *marketCountFlag,
		}); err != nil {
			fmt.Fprintf(stderr, "simulator: %v\n", err)
			return 1
		}
	} else if *interactive && shouldPrompt(defaultInteractive, flagWasProvided(args, "interactive"), cfg) {
		if err := promptScenario(stdin, stdout, cfg); err != nil {
			fmt.Fprintf(stderr, "simulator: %v\n", err)
			return 1
		}
	}

	logger := log.New(stdout, "", 0)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := sim.New(cfg, logger).Run(ctx); err != nil {
		fmt.Fprintf(stderr, "simulator: %v\n", err)
		return 1
	}
	return 0
}

func shouldPrompt(defaultInteractive bool, interactiveFlagProvided bool, cfg *config.Config) bool {
	return defaultInteractive || interactiveFlagProvided
}

func flagWasProvided(args []string, name string) bool {
	shortPrefix := "-" + name
	longPrefix := "--" + name
	for _, arg := range args {
		if arg == shortPrefix || strings.HasPrefix(arg, shortPrefix+"=") || arg == longPrefix || strings.HasPrefix(arg, longPrefix+"=") {
			return true
		}
	}
	return false
}

type scenarioPrompter struct {
	reader *bufio.Reader
	out    io.Writer
}

type scenarioOptions struct {
	scenario    string
	gameIDs     string
	marketCount int
}

func applyScenarioOptions(cfg *config.Config, options scenarioOptions) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	scenarioType := strings.TrimSpace(options.scenario)
	if scenarioType == "" {
		switch {
		case strings.TrimSpace(options.gameIDs) != "" && options.marketCount > 0:
			return errors.New("-game-ids and -market-count cannot be used together")
		case strings.TrimSpace(options.gameIDs) != "":
			scenarioType = config.ScenarioTradeExisting
		case options.marketCount > 0:
			scenarioType = config.ScenarioCreateAndTrade
		default:
			scenarioType = cfg.Scenario.Type
		}
	}
	parsedType, err := parseScenarioChoice(scenarioType, cfg.Scenario.Type)
	if err != nil {
		return err
	}
	cfg.Scenario.Type = parsedType

	switch parsedType {
	case config.ScenarioTradeExisting:
		if options.marketCount > 0 {
			return errors.New("-market-count is only valid with -scenario=create")
		}
		if strings.TrimSpace(options.gameIDs) != "" {
			ids, err := parseGameIDs(options.gameIDs)
			if err != nil {
				return err
			}
			cfg.Scenario.ExistingGameIDs = ids
		}
		if len(cfg.Scenario.ExistingGameIDs) == 0 {
			return errors.New("existing mode requires -game-ids or scenario.existing_game_ids")
		}
	case config.ScenarioCreateAndTrade:
		if strings.TrimSpace(options.gameIDs) != "" {
			return errors.New("-game-ids is only valid with -scenario=existing")
		}
		if options.marketCount < 0 {
			return errors.New("-market-count must be positive")
		}
		if options.marketCount > 0 {
			cfg.Scenario.MarketCount = options.marketCount
		}
		cfg.Scenario.ExistingGameIDs = nil
	}
	if cfg.Runtime.Mode == config.ModeExecute {
		cfg.Runtime.RegeneratePlanOnExecute = true
	}
	return nil
}

func promptScenario(in io.Reader, out io.Writer, cfg *config.Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	p := &scenarioPrompter{
		reader: bufio.NewReader(in),
		out:    out,
	}

	scenarioType, err := p.askScenario(cfg.Scenario.Type)
	if err != nil {
		return err
	}
	cfg.Scenario.Type = scenarioType

	switch scenarioType {
	case config.ScenarioCreateAndTrade:
		count, err := p.askPositiveInt("每轮创建多少个新博弈池", cfg.Scenario.MarketCount)
		if err != nil {
			return err
		}
		cfg.Scenario.MarketCount = count
		cfg.Scenario.ExistingGameIDs = nil
	case config.ScenarioTradeExisting:
		ids, err := p.askGameIDs(cfg.Scenario.ExistingGameIDs)
		if err != nil {
			return err
		}
		cfg.Scenario.ExistingGameIDs = ids
	default:
		return fmt.Errorf("unsupported scenario %q", scenarioType)
	}
	if cfg.Runtime.Mode == config.ModeExecute {
		cfg.Runtime.RegeneratePlanOnExecute = true
	}

	fmt.Fprintf(out, "已选择模拟模式：%s\n", cfg.Scenario.Type)
	return nil
}

func (p *scenarioPrompter) askScenario(current string) (string, error) {
	for {
		fmt.Fprintf(p.out, "请选择模拟数据模式：\n")
		fmt.Fprintf(p.out, "  1) 已有博弈池 - 为指定 game_id 持续生成购买数据\n")
		fmt.Fprintf(p.out, "  2) 创建新博弈池 - 创建新池并持续生成购买数据\n")
		fmt.Fprintf(p.out, "请选择 [%s]: ", current)

		line, eof, err := p.readLine()
		if err != nil {
			return "", err
		}
		scenarioType, err := parseScenarioChoice(line, current)
		if err == nil {
			return scenarioType, nil
		}
		if eof {
			return "", err
		}
		fmt.Fprintf(p.out, "%v\n", err)
	}
}

func (p *scenarioPrompter) askPositiveInt(prompt string, current int) (int, error) {
	for {
		fmt.Fprintf(p.out, "%s [%d]: ", prompt, current)
		line, eof, err := p.readLine()
		if err != nil {
			return 0, err
		}
		if strings.TrimSpace(line) == "" {
			if current > 0 {
				return current, nil
			}
			err = errors.New("value must be positive")
		} else {
			value, parseErr := strconv.Atoi(strings.TrimSpace(line))
			if parseErr != nil || value <= 0 {
				err = errors.New("value must be a positive integer")
			} else {
				return value, nil
			}
		}
		if eof {
			return 0, err
		}
		fmt.Fprintf(p.out, "%v\n", err)
	}
}

func (p *scenarioPrompter) askGameIDs(current []int) ([]int, error) {
	for {
		fmt.Fprintf(p.out, "请输入已有博弈池 game_id，使用逗号或空格分隔")
		if len(current) > 0 {
			fmt.Fprintf(p.out, " [%s]", formatGameIDs(current))
		}
		fmt.Fprintf(p.out, ": ")

		line, eof, err := p.readLine()
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(line) == "" && len(current) > 0 {
			return append([]int(nil), current...), nil
		}
		ids, err := parseGameIDs(line)
		if err == nil {
			return ids, nil
		}
		if eof {
			return nil, err
		}
		fmt.Fprintf(p.out, "%v\n", err)
	}
}

func (p *scenarioPrompter) readLine() (line string, eof bool, err error) {
	line, err = p.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", false, err
	}
	return strings.TrimSpace(line), errors.Is(err, io.EOF), nil
}

func parseScenarioChoice(raw string, current string) (string, error) {
	choice := strings.ToLower(strings.TrimSpace(raw))
	if choice == "" {
		choice = strings.ToLower(strings.TrimSpace(current))
	}
	switch choice {
	case "2", "create", "create_and_trade", "create-and-trade", "new", "new_pool", "new-pool":
		return config.ScenarioCreateAndTrade, nil
	case "1", "trade", "existing", "trade_existing", "trade-existing", "buy", "buy_existing", "buy-existing":
		return config.ScenarioTradeExisting, nil
	default:
		return "", fmt.Errorf("selection must be 1/%s or 2/%s", config.ScenarioTradeExisting, config.ScenarioCreateAndTrade)
	}
}

func parseGameIDs(raw string) ([]int, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "[")
	cleaned = strings.TrimSuffix(cleaned, "]")
	parts := strings.FieldsFunc(cleaned, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
	if len(parts) == 0 {
		return nil, errors.New("at least one existing game ID is required")
	}
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("game ID %q must be a positive integer", part)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func formatGameIDs(ids []int) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, ",")
}
