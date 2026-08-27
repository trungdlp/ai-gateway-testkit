package behavioral

import (
	"fmt"
	"strings"

	"github.com/trungdlp/ai-gateway-testkit/cases/caseutil"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

const expected = "AGTK_OK"

func behavioralBase(id, title, suite string, dependency string, run testcase.RunFunc) testcase.Definition {
	return testcase.Definition{
		ID: id, Revision: 1, Title: title,
		Description: "Checks exact instruction following independently from protocol schema compatibility.",
		Layer:       testcase.LayerBehavioral, Suite: suite, Area: "instruction_following",
		Stability: testcase.StabilityStable, Determinism: testcase.Probabilistic,
		DependsOn: []string{dependency}, Cost: testcase.Cost{Requests: 1, MaxInputTokens: 64, MaxOutputTokens: 16},
		Risk:       testcase.Risk{Mutation: "none", ContainsOutput: true},
		Assertions: caseutil.Assertions(caseutil.Assertion("A01", "Model follows an exact-output instruction", testcase.Should, testcase.Medium)),
		Run:        run,
	}
}

func exactResult(environment testcase.Environment, output string) testcase.Execution {
	trimmed := strings.TrimSpace(output)
	message := fmt.Sprintf("output = %q, want %q", caseutil.Truncate(environment.Redact(trimmed), 160), expected)
	return testcase.Execution{
		Assertions: []testcase.AssertionResult{testcase.Result("A01", trimmed == expected, "BEHAVIOR.EXACT_OUTPUT_MISMATCH", message)},
		Evidence:   map[string]any{"output_preview": caseutil.Truncate(environment.Redact(trimmed), 160)},
	}
}
