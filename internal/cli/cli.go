package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
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
	if *interactive && shouldPrompt(defaultInteractive, flagWasProvided(args, "interactive"), cfg) {
		if err := promptScenario(stdin, stdout, cfg); err != nil {
			fmt.Fprintf(stderr, "simulator: %v\n", err)
			return 1
		}
	}

	logger := log.New(stdout, "", 0)
	if err := sim.New(cfg, logger).Run(context.Background()); err != nil {
		fmt.Fprintf(stderr, "simulator: %v\n", err)
		return 1
	}
	return 0
}

func shouldPrompt(defaultInteractive bool, interactiveFlagProvided bool, cfg *config.Config) bool {
	if cfg == nil {
		return true
	}
	if cfg.Runtime.Mode == config.ModePreview {
		return true
	}
	return interactiveFlagProvided || !defaultInteractive
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

func promptScenario(in io.Reader, out io.Writer, cfg *config.Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if cfg.Runtime.Mode != config.ModePreview {
		return fmt.Errorf("-interactive only affects generated data in runtime.mode=%q; current mode %q reads plan_file instead", config.ModePreview, cfg.Runtime.Mode)
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
		count, err := p.askPositiveInt("How many new pools should be created", cfg.Scenario.MarketCount)
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
		if cfg.Runtime.Mode == config.ModeExecute && !cfg.Runtime.OnChain {
			return errors.New("trade_existing execution requires runtime.on_chain=true; use preview mode to generate a plan without chain writes")
		}
	default:
		return fmt.Errorf("unsupported scenario %q", scenarioType)
	}

	fmt.Fprintf(out, "Selected scenario: %s\n", cfg.Scenario.Type)
	return nil
}

func (p *scenarioPrompter) askScenario(current string) (string, error) {
	for {
		fmt.Fprintf(p.out, "Choose generated data type:\n")
		fmt.Fprintf(p.out, "  1) create_and_trade - create new pools and buy them\n")
		fmt.Fprintf(p.out, "  2) trade_existing - buy configured existing pools\n")
		fmt.Fprintf(p.out, "Selection [%s]: ", current)

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
		fmt.Fprintf(p.out, "Existing game IDs, separated by comma or space")
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
	case "1", "create", "create_and_trade", "create-and-trade", "new", "new_pool", "new-pool":
		return config.ScenarioCreateAndTrade, nil
	case "2", "trade", "existing", "trade_existing", "trade-existing", "buy", "buy_existing", "buy-existing":
		return config.ScenarioTradeExisting, nil
	default:
		return "", fmt.Errorf("selection must be 1/%s or 2/%s", config.ScenarioCreateAndTrade, config.ScenarioTradeExisting)
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
