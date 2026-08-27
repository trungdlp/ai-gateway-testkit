package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/trungdlp/ai-gateway-testkit/cases"
	"github.com/trungdlp/ai-gateway-testkit/internal/agentrunner"
	"github.com/trungdlp/ai-gateway-testkit/internal/catalog"
	"github.com/trungdlp/ai-gateway-testkit/internal/compare"
	"github.com/trungdlp/ai-gateway-testkit/internal/config"
	"github.com/trungdlp/ai-gateway-testkit/internal/engine"
	"github.com/trungdlp/ai-gateway-testkit/internal/gateway"
	"github.com/trungdlp/ai-gateway-testkit/internal/profile"
	"github.com/trungdlp/ai-gateway-testkit/internal/report"
	"github.com/trungdlp/ai-gateway-testkit/internal/result"
	"github.com/trungdlp/ai-gateway-testkit/internal/target"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

const (
	ExitOK            = 0
	ExitChecksFailed  = 1
	ExitConfiguration = 2
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

type Environment func(string) (string, bool)

type app struct {
	stdout   io.Writer
	stderr   io.Writer
	getenv   Environment
	build    BuildInfo
	logger   *slog.Logger
	logLevel *slog.LevelVar
	verbose  bool
}

type runOptions struct {
	baseURL           string
	model             string
	protocol          string
	apiKeyEnv         string
	targetFile        string
	profiles          []string
	timeout           time.Duration
	format            string
	output            string
	baseline          string
	agentRunner       string
	retries           int
	retryBackoff      time.Duration
	retryMaxWait      time.Duration
	allowInsecureHTTP bool
}

type checksFailedError struct{}

func (checksFailedError) Error() string { return "one or more selected profiles did not pass" }

func Run(ctx context.Context, args []string, stdout, stderr io.Writer, getenv Environment, build BuildInfo) int {
	logLevel := new(slog.LevelVar)
	logLevel.Set(slog.LevelError)
	a := &app{stdout: stdout, stderr: stderr, getenv: getenv, build: build, logLevel: logLevel}
	a.logger = slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: logLevel}))
	command := a.newRootCommand()
	command.SetArgs(args)
	if err := command.ExecuteContext(ctx); err != nil {
		var failed checksFailedError
		if errors.As(err, &failed) {
			return ExitChecksFailed
		}
		a.logger.ErrorContext(ctx, "command failed", "error", err)
		return ExitConfiguration
	}
	return ExitOK
}

func (a *app) newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use: "agtk", Short: "Conformance and regression tests for AI gateways",
		SilenceErrors: true, SilenceUsage: true, Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error { return command.Help() },
	}
	root.SetOut(a.stdout)
	root.SetErr(a.stderr)
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().BoolVarP(&a.verbose, "verbose", "v", false, "emit diagnostic logs to stderr")
	root.AddCommand(a.newRunCommand(), a.newCatalogCommand(), a.newProfilesCommand(), a.newCompareCommand(), a.newReportCommand(), a.newVersionCommand())
	return root
}

func (a *app) newRunCommand() *cobra.Command {
	baseURLDefault, _ := a.getenv("AI_GATEWAY_BASE_URL")
	modelDefault, _ := a.getenv("AI_GATEWAY_MODEL")
	protocolDefault, ok := a.getenv("AI_GATEWAY_PROTOCOL")
	if !ok || strings.TrimSpace(protocolDefault) == "" {
		protocolDefault = string(config.ProtocolOpenAI)
	}
	options := runOptions{baseURL: baseURLDefault, model: modelDefault, protocol: protocolDefault, apiKeyEnv: "AI_GATEWAY_API_KEY", timeout: 30 * time.Second, format: report.FormatText, agentRunner: "none", retries: 2, retryBackoff: 250 * time.Millisecond, retryMaxWait: 5 * time.Second}
	command := &cobra.Command{Use: "run", Short: "Run one or more compatibility profiles", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		return a.runChecks(command.Context(), options)
	}}
	flags := command.Flags()
	flags.StringVar(&options.targetFile, "target", "", "target manifest in YAML format")
	flags.StringSliceVar(&options.profiles, "profile", nil, "profile ID; repeat or pass a comma-separated list")
	flags.StringVar(&options.baseURL, "base-url", options.baseURL, "legacy API base URL (env: AI_GATEWAY_BASE_URL)")
	flags.StringVar(&options.model, "model", options.model, "legacy model ID (env: AI_GATEWAY_MODEL)")
	flags.StringVar(&options.protocol, "protocol", options.protocol, "legacy protocol: openai, anthropic, or both")
	flags.StringVar(&options.apiKeyEnv, "api-key-env", options.apiKeyEnv, "legacy credential environment variable")
	flags.DurationVar(&options.timeout, "timeout", options.timeout, "timeout for each network operation")
	flags.StringVar(&options.format, "format", options.format, "report format: text or json")
	flags.StringVarP(&options.output, "output", "o", "", "write the report to a file instead of stdout")
	flags.StringVar(&options.baseline, "baseline", "", "annotate results against a baseline JSON report")
	flags.StringVar(&options.agentRunner, "agent-runner", options.agentRunner, "operational agent runner: none or sbx")
	flags.IntVar(&options.retries, "retries", options.retries, "retries after a transient failure (0-10)")
	flags.DurationVar(&options.retryBackoff, "retry-backoff", options.retryBackoff, "initial exponential retry backoff")
	flags.DurationVar(&options.retryMaxWait, "retry-max-wait", options.retryMaxWait, "maximum retry delay, including Retry-After")
	flags.BoolVar(&options.allowInsecureHTTP, "allow-insecure-http", false, "allow plain HTTP for non-loopback targets")
	return command
}

func (a *app) runChecks(ctx context.Context, options runOptions) error {
	if a.verbose {
		a.logLevel.Set(slog.LevelInfo)
	}
	if options.format != report.FormatText && options.format != report.FormatJSON {
		return fmt.Errorf("invalid --format %q: expected text or json", options.format)
	}
	c, registry, err := loadDefinitions()
	if err != nil {
		return err
	}
	manifest, err := a.manifest(options)
	if err != nil {
		return err
	}
	retryPolicy := gateway.RetryPolicy{MaxRetries: options.retries, InitialBackoff: options.retryBackoff, MaxBackoff: options.retryMaxWait}
	resolved, err := target.Resolve(manifest, target.Environment(a.getenv), options.timeout, options.allowInsecureHTTP, retryPolicy)
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	profiles := normalizedProfiles(options.profiles)
	if len(profiles) == 0 {
		profiles = defaultProfiles(resolved)
	}
	var runner testcase.AgentRunner
	switch options.agentRunner {
	case "none":
	case "sbx":
		runner = agentrunner.NewSBX()
	default:
		return fmt.Errorf("invalid --agent-runner %q: expected none or sbx", options.agentRunner)
	}
	a.logger.InfoContext(ctx, "starting profiles", "target", resolved.Name, "profiles", profiles)
	value, err := engine.New(c, registry, resolved, runner).Run(ctx, engine.Options{Profiles: profiles, Build: result.Build{Version: valueOr(a.build.Version, "dev"), Commit: valueOr(a.build.Commit, "unknown"), Date: valueOr(a.build.Date, "unknown")}})
	if err != nil {
		return err
	}
	if options.baseline != "" {
		baseline, err := readReport(options.baseline)
		if err != nil {
			return fmt.Errorf("read baseline: %w", err)
		}
		value = compare.ApplyBaseline(baseline, value)
	}
	writer, closeWriter, err := a.outputWriter(options.output)
	if err != nil {
		return err
	}
	defer closeWriter()
	if err := report.Write(writer, options.format, value); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	for _, evaluation := range value.Profiles {
		if evaluation.Verdict != result.VerdictPass {
			return checksFailedError{}
		}
	}
	return nil
}

func (a *app) manifest(options runOptions) (target.Manifest, error) {
	if options.targetFile != "" {
		return target.Load(options.targetFile)
	}
	protocol, err := config.ParseProtocol(options.protocol)
	if err != nil {
		return target.Manifest{}, err
	}
	if strings.TrimSpace(options.apiKeyEnv) == "" {
		return target.Manifest{}, fmt.Errorf("--api-key-env must not be empty")
	}
	return target.Legacy("gateway", protocol, options.baseURL, options.model, options.apiKeyEnv), nil
}

func (a *app) newCatalogCommand() *cobra.Command {
	command := &cobra.Command{Use: "catalog", Short: "Inspect the versioned test catalog", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error { return command.Help() }}
	command.AddCommand(
		&cobra.Command{Use: "list", Short: "List scenario IDs", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
			c, _, err := loadDefinitions()
			if err != nil {
				return err
			}
			for _, definition := range c.Definitions() {
				if _, err := fmt.Fprintf(a.stdout, "%-16s %-11s %s\n", definition.ID, definition.Layer, definition.Title); err != nil {
					return err
				}
			}
			return nil
		}},
		&cobra.Command{Use: "show CASE_ID", Short: "Show one scenario definition", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
			c, _, err := loadDefinitions()
			if err != nil {
				return err
			}
			definition, ok := c.Get(args[0])
			if !ok {
				return fmt.Errorf("unknown case %s", args[0])
			}
			return writeJSON(a.stdout, definition)
		}},
		&cobra.Command{Use: "validate", Short: "Validate catalog and embedded profiles", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
			c, registry, err := loadDefinitions()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(a.stdout, "catalog %s valid: %d cases, %d profiles, %s\n", catalog.Version, len(c.Definitions()), len(registry.Definitions()), c.Digest())
			return err
		}},
		&cobra.Command{Use: "export", Short: "Export the canonical catalog as JSON", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
			c, _, err := loadDefinitions()
			if err != nil {
				return err
			}
			return writeJSON(a.stdout, c.Document())
		}},
	)
	return command
}

func (a *app) newProfilesCommand() *cobra.Command {
	return &cobra.Command{Use: "profiles", Short: "List available compatibility profiles", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		_, registry, err := loadDefinitions()
		if err != nil {
			return err
		}
		for _, definition := range registry.Definitions() {
			if _, err := fmt.Fprintf(a.stdout, "%-24s %-8s %s\n", definition.ID, definition.Version, definition.Title); err != nil {
				return err
			}
		}
		return nil
	}}
}

func (a *app) newCompareCommand() *cobra.Command {
	format := report.FormatText
	command := &cobra.Command{Use: "compare BASELINE CURRENT", Short: "Compare two canonical JSON reports", Args: cobra.ExactArgs(2), RunE: func(_ *cobra.Command, args []string) error {
		baseline, err := readReport(args[0])
		if err != nil {
			return err
		}
		current, err := readReport(args[1])
		if err != nil {
			return err
		}
		comparison := compare.Reports(baseline, current)
		if format == report.FormatJSON {
			return writeJSON(a.stdout, comparison)
		}
		if format != report.FormatText {
			return fmt.Errorf("invalid --format %q: expected text or json", format)
		}
		if _, err := fmt.Fprintf(a.stdout, "Comparable: %t\n", comparison.Comparable); err != nil {
			return err
		}
		for _, change := range comparison.Changes {
			if change.State != compare.UnchangedPass {
				if _, err := fmt.Fprintf(a.stdout, "%-24s %-18s %s -> %s\n", change.AssertionID, change.State, change.Baseline, change.Current); err != nil {
					return err
				}
			}
		}
		return nil
	}}
	command.Flags().StringVar(&format, "format", format, "output format: text or json")
	return command
}

func (a *app) newReportCommand() *cobra.Command {
	command := &cobra.Command{Use: "report", Short: "Process canonical reports", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error { return command.Help() }}
	command.AddCommand(&cobra.Command{Use: "sanitize INPUT", Short: "Remove endpoints, models, evidence, and diagnostic text", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		value, err := readReport(args[0])
		if err != nil {
			return err
		}
		return report.Write(a.stdout, report.FormatJSON, report.Sanitize(value))
	}})
	return command
}

func (a *app) newVersionCommand() *cobra.Command {
	return &cobra.Command{Use: "version", Short: "Print version information", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		_, err := fmt.Fprintf(a.stdout, "agtk %s (commit %s, built %s)\n", valueOr(a.build.Version, "dev"), valueOr(a.build.Commit, "unknown"), valueOr(a.build.Date, "unknown"))
		return err
	}}
}

func loadDefinitions() (*catalog.Catalog, *profile.Registry, error) {
	c, err := cases.Catalog()
	if err != nil {
		return nil, nil, fmt.Errorf("catalog validation: %w", err)
	}
	registry, err := profile.Load(c)
	if err != nil {
		return nil, nil, fmt.Errorf("profile validation: %w", err)
	}
	return c, registry, nil
}

func defaultProfiles(resolved target.Resolved) []string {
	var profiles []string
	if _, ok := resolved.Endpoints["openai"]; ok {
		profiles = append(profiles, "oai-tools", "oai-sdk-go")
	}
	if _, ok := resolved.Endpoints["anthropic"]; ok {
		profiles = append(profiles, "anthropic-tools", "anthropic-sdk-go")
	}
	return profiles
}

func normalizedProfiles(values []string) []string {
	seen := map[string]struct{}{}
	var result []string
	for _, value := range values {
		for _, id := range strings.Split(value, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				result = append(result, id)
			}
		}
	}
	sort.Strings(result)
	return result
}

func readReport(path string) (result.Report, error) {
	file, err := os.Open(path)
	if err != nil {
		return result.Report{}, err
	}
	defer file.Close()
	return report.Decode(file)
}

func (a *app) outputWriter(path string) (io.Writer, func(), error) {
	if path == "" {
		return a.stdout, func() {}, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, func() {}, fmt.Errorf("create output report: %w", err)
	}
	return file, func() { _ = file.Close() }, nil
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func OSEnvironment(name string) (string, bool) { return os.LookupEnv(name) }
