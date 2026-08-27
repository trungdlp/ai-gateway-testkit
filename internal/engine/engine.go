package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/trungdlp/ai-gateway-testkit/internal/catalog"
	"github.com/trungdlp/ai-gateway-testkit/internal/evaluator"
	"github.com/trungdlp/ai-gateway-testkit/internal/gateway"
	"github.com/trungdlp/ai-gateway-testkit/internal/profile"
	"github.com/trungdlp/ai-gateway-testkit/internal/result"
	"github.com/trungdlp/ai-gateway-testkit/internal/target"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

type Options struct {
	Profiles []string
	Build    result.Build
}

type Engine struct {
	catalog  *catalog.Catalog
	profiles *profile.Registry
	targets  target.Resolved
	agent    testcase.AgentRunner
}

func New(c *catalog.Catalog, profiles *profile.Registry, targets target.Resolved, agents ...testcase.AgentRunner) *Engine {
	engine := &Engine{catalog: c, profiles: profiles, targets: targets}
	if len(agents) > 0 {
		engine.agent = agents[0]
	}
	return engine
}

func (e *Engine) Run(ctx context.Context, options Options) (result.Report, error) {
	started := time.Now().UTC()
	selected, err := e.selectedCases(options.Profiles)
	if err != nil {
		return result.Report{}, err
	}
	environment := newLiveEnvironment(e.targets, e.agent)
	scenarios := make([]result.Scenario, 0, len(selected))
	byID := make(map[string]result.Scenario, len(selected))
	pending := append([]testcase.Definition(nil), selected...)
	for len(pending) > 0 {
		progress := false
		for index := 0; index < len(pending); {
			definition := pending[index]
			ready := true
			blockedBy := ""
			for _, dependency := range definition.DependsOn {
				dependencyResult, exists := byID[dependency]
				if !exists {
					ready = false
					break
				}
				if dependencyResult.Status != testcase.StatusPass {
					blockedBy = dependency
				}
			}
			if !ready {
				index++
				continue
			}
			var scenario result.Scenario
			if blockedBy != "" {
				scenario = blockedScenario(definition, blockedBy)
			} else {
				scenario = executeScenario(ctx, definition, environment)
			}
			scenarios = append(scenarios, scenario)
			byID[definition.ID] = scenario
			pending = append(pending[:index], pending[index+1:]...)
			progress = true
		}
		if !progress {
			return result.Report{}, fmt.Errorf("unable to resolve selected case dependencies")
		}
	}
	sort.Slice(scenarios, func(i, j int) bool { return scenarios[i].ID < scenarios[j].ID })
	evaluations, err := evaluator.Evaluate(options.Profiles, e.profiles, e.catalog, scenarios)
	if err != nil {
		return result.Report{}, err
	}
	report := result.Report{
		SchemaVersion: result.SchemaVersion, CatalogVersion: catalog.Version, CatalogDigest: e.catalog.Digest(),
		RunID: newRunID(), StartedAt: started, DurationMS: time.Since(started).Milliseconds(), Build: options.Build,
		Runtime: result.Runtime{OS: runtime.GOOS, Arch: runtime.GOARCH, GoVersion: runtime.Version(), DependencyVersions: dependencyVersions(), Retry: retryResult(e.targets, environment.RetryStats())},
		Target:  publicTarget(e.targets), SelectedProfiles: append([]string(nil), options.Profiles...), Profiles: evaluations, Scenarios: scenarios,
	}
	for _, scenario := range scenarios {
		for _, assertion := range scenario.Assertions {
			report.Summary.Total++
			switch assertion.Status {
			case testcase.StatusPass:
				report.Summary.Passed++
			case testcase.StatusFail:
				report.Summary.Failed++
			case testcase.StatusError:
				report.Summary.Errors++
			case testcase.StatusBlocked:
				report.Summary.Blocked++
			case testcase.StatusSkipped:
				report.Summary.Skipped++
			case testcase.StatusNotApplicable:
				report.Summary.NotApplicable++
			}
		}
	}
	return report, nil
}

func (e *Engine) selectedCases(profileIDs []string) ([]testcase.Definition, error) {
	selected := map[string]struct{}{}
	var addCase func(string) error
	addCase = func(id string) error {
		if _, exists := selected[id]; exists {
			return nil
		}
		definition, ok := e.catalog.Get(id)
		if !ok {
			return fmt.Errorf("unknown case %s", id)
		}
		selected[id] = struct{}{}
		for _, dependency := range definition.DependsOn {
			if err := addCase(dependency); err != nil {
				return err
			}
		}
		return nil
	}
	for _, profileID := range profileIDs {
		selection, err := e.profiles.Expand(profileID, e.catalog)
		if err != nil {
			return nil, err
		}
		for assertionID := range selection.Required {
			if err := addCase(strings.Split(assertionID, "/")[0]); err != nil {
				return nil, err
			}
		}
		for assertionID := range selection.Optional {
			if err := addCase(strings.Split(assertionID, "/")[0]); err != nil {
				return nil, err
			}
		}
	}
	definitions := make([]testcase.Definition, 0, len(selected))
	for id := range selected {
		definition, _ := e.catalog.Get(id)
		definitions = append(definitions, definition)
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].ID < definitions[j].ID })
	return definitions, nil
}

func executeScenario(ctx context.Context, definition testcase.Definition, environment testcase.Environment) result.Scenario {
	started := time.Now()
	before := environmentRetryStats(environment)
	execution := safeRun(ctx, definition, environment)
	after := environmentRetryStats(environment)
	retries := after.Retries - before.Retries
	exhausted := after.Exhausted - before.Exhausted
	if retries > 0 || exhausted > 0 {
		if execution.Evidence == nil {
			execution.Evidence = map[string]any{}
		}
		execution.Evidence["retry"] = map[string]int64{"retries": retries, "exhausted": exhausted}
	}
	provided := make(map[string]testcase.AssertionResult, len(execution.Assertions))
	for _, assertion := range execution.Assertions {
		provided[assertion.ID] = assertion
	}
	scenario := result.Scenario{ID: definition.ID, Revision: definition.Revision, Title: definition.Title, Layer: definition.Layer, Suite: definition.Suite, Area: definition.Area, DurationMS: time.Since(started).Milliseconds(), Evidence: execution.Evidence, Status: testcase.StatusPass}
	for _, assertionDefinition := range definition.Assertions {
		assertionResult, ok := provided[assertionDefinition.ID]
		if !ok {
			assertionResult = testcase.Error(assertionDefinition.ID, "RUNNER.ASSERTION_MISSING", "scenario did not return this assertion")
		}
		assertion := result.Assertion{ID: definition.ID + "/" + assertionDefinition.ID, Title: assertionDefinition.Title, Requirement: assertionDefinition.Requirement, Impact: assertionDefinition.Impact, Status: assertionResult.Status, ReasonCode: assertionResult.ReasonCode, Message: environment.Redact(assertionResult.Message), Expected: environment.Redact(assertionResult.Expected), Observed: environment.Redact(assertionResult.Observed)}
		scenario.Assertions = append(scenario.Assertions, assertion)
		scenario.Status = combineStatus(scenario.Status, assertion.Status)
	}
	return scenario
}

type retryStatsEnvironment interface {
	RetryStats() gateway.Stats
}

func environmentRetryStats(environment testcase.Environment) gateway.Stats {
	provider, ok := environment.(retryStatsEnvironment)
	if !ok {
		return gateway.Stats{}
	}
	return provider.RetryStats()
}

func safeRun(ctx context.Context, definition testcase.Definition, environment testcase.Environment) (execution testcase.Execution) {
	defer func() {
		if recovered := recover(); recovered != nil {
			execution.Assertions = nil
			for _, assertion := range definition.Assertions {
				execution.Assertions = append(execution.Assertions, testcase.Error(assertion.ID, "RUNNER.PANIC", fmt.Sprintf("scenario runner panicked: %v", recovered)))
			}
		}
	}()
	return definition.Run(ctx, environment)
}

func blockedScenario(definition testcase.Definition, dependency string) result.Scenario {
	scenario := result.Scenario{ID: definition.ID, Revision: definition.Revision, Title: definition.Title, Layer: definition.Layer, Suite: definition.Suite, Area: definition.Area, Status: testcase.StatusBlocked}
	for _, assertion := range definition.Assertions {
		scenario.Assertions = append(scenario.Assertions, result.Assertion{ID: definition.ID + "/" + assertion.ID, Title: assertion.Title, Requirement: assertion.Requirement, Impact: assertion.Impact, Status: testcase.StatusBlocked, ReasonCode: "DEPENDENCY.BLOCKED", Message: "blocked by " + dependency})
	}
	return scenario
}

func combineStatus(current, next testcase.Status) testcase.Status {
	priority := map[testcase.Status]int{testcase.StatusPass: 0, testcase.StatusNotApplicable: 1, testcase.StatusSkipped: 2, testcase.StatusBlocked: 3, testcase.StatusError: 4, testcase.StatusFail: 5}
	if priority[next] > priority[current] {
		return next
	}
	return current
}

func publicTarget(resolved target.Resolved) result.Target {
	endpoints := make(map[string]result.TargetEndpoint, len(resolved.Endpoints))
	for protocol, endpoint := range resolved.Endpoints {
		endpoints[protocol] = result.TargetEndpoint{BaseURL: endpoint.BaseURL, Model: endpoint.Model, APIVersion: endpoint.APIVersion}
	}
	return result.Target{Name: resolved.Name, Fingerprint: resolved.Fingerprint(), Endpoints: endpoints}
}

func newRunID() string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	return "run-" + hex.EncodeToString(data)
}

func dependencyVersions() map[string]string {
	versions := map[string]string{}
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return versions
	}
	for _, dependency := range build.Deps {
		switch dependency.Path {
		case "github.com/openai/openai-go/v3":
			versions["openai-go"] = dependency.Version
		case "github.com/anthropics/anthropic-sdk-go":
			versions["anthropic-sdk-go"] = dependency.Version
		}
	}
	return versions
}

func retryResult(targets target.Resolved, stats gateway.Stats) result.Retry {
	var policy gateway.RetryPolicy
	for _, endpoint := range targets.Endpoints {
		policy = endpoint.Retry
		break
	}
	return result.Retry{
		MaxRetries: policy.MaxRetries, InitialBackoffMS: policy.InitialBackoff.Milliseconds(), MaxBackoffMS: policy.MaxBackoff.Milliseconds(),
		RawHTTPRequests: stats.Requests, RawHTTPAttempts: stats.Attempts, RawHTTPRetries: stats.Retries, Exhausted: stats.Exhausted,
	}
}
