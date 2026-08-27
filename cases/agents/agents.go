package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/trungdlp/ai-gateway-testkit/cases/caseutil"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

const expectedFile = "AGTK_AGENT_OK"

func Definitions() []testcase.Definition {
	return []testcase.Definition{
		agentCase("CDX-EXEC-001", "Codex non-interactive coding workflow", "codex", "openai", "OAI-TOOL-001"),
		agentCase("CLC-EXEC-001", "Claude Code non-interactive coding workflow", "claude", "anthropic", "ANT-TOOL-001"),
	}
}

func agentCase(id, title, agent, protocol, dependency string) testcase.Definition {
	assertions := caseutil.Assertions(
		caseutil.Assertion("A01", "Agent process completes successfully", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A02", "Agent creates the requested file", testcase.Must, testcase.High),
		caseutil.Assertion("A03", "Created file has the exact requested content", testcase.Must, testcase.High),
		caseutil.Assertion("A04", "Agent invokes the fixture validator", testcase.Must, testcase.High),
	)
	return testcase.Definition{
		ID: id, Revision: 1, Title: title,
		Description: "Runs an isolated, non-interactive coding task and verifies filesystem and shell-tool effects.",
		Layer:       testcase.LayerAgent, Suite: agent, Area: "execution", Stability: testcase.StabilityExperimental,
		Determinism: testcase.Operational, DependsOn: []string{dependency},
		Preconditions: []string{"sbx is installed and authenticated", "agent sandbox image is available"},
		Cost:          testcase.Cost{Requests: 0, MaxInputTokens: 512, MaxOutputTokens: 256},
		Risk:          testcase.Risk{Mutation: "ephemeral_workspace", CleanupRequired: true, ContainsOutput: true},
		Assertions:    assertions,
		Run: func(ctx context.Context, environment testcase.Environment) testcase.Execution {
			if _, ok := environment.Target(protocol); !ok {
				return caseutil.NotApplicableAll(assertions, protocol)
			}
			outcome, err := environment.RunAgent(ctx, testcase.AgentRequest{
				Agent: agent, Protocol: protocol,
				Prompt: "Read TASK.md. Complete the task using filesystem tools, then execute ./verify.sh. Do not merely describe the changes.",
			})
			if err != nil {
				return caseutil.ErrorAll(assertions, "AGENT.RUNNER_FAILED", environment.Redact(err.Error()))
			}
			if !outcome.Available {
				results := make([]testcase.AssertionResult, 0, len(assertions))
				for _, assertion := range assertions {
					results = append(results, testcase.AssertionResult{ID: assertion.ID, Status: testcase.StatusSkipped, ReasonCode: "AGENT.RUNNER_DISABLED", Message: "enable the sbx agent runner to execute this operational case"})
				}
				return testcase.Execution{Assertions: results}
			}
			results := []testcase.AssertionResult{
				testcase.Result("A01", outcome.ExitCode == 0, "AGENT.EXIT_NONZERO", fmt.Sprintf("agent exited with code %d", outcome.ExitCode)),
				testcase.Result("A02", outcome.ResultFile != "", "AGENT.FILE_MISSING", "agent did not create result.txt"),
				testcase.Result("A03", strings.TrimSpace(outcome.ResultFile) == expectedFile, "AGENT.FILE_CONTENT_MISMATCH", fmt.Sprintf("result.txt = %q", caseutil.Truncate(outcome.ResultFile, 80))),
				testcase.Result("A04", outcome.ValidationMarker, "AGENT.VALIDATOR_NOT_RUN", "agent did not successfully invoke ./verify.sh"),
			}
			return testcase.Execution{Assertions: results, Evidence: map[string]any{"agent_output_preview": caseutil.Truncate(environment.Redact(outcome.Output), 2000)}}
		},
	}
}
