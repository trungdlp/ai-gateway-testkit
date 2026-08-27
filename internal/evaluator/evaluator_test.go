package evaluator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trungdlp/ai-gateway-testkit/cases"
	"github.com/trungdlp/ai-gateway-testkit/internal/profile"
	"github.com/trungdlp/ai-gateway-testkit/internal/result"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func TestEvaluateDistinguishesFailFromIndeterminate(t *testing.T) {
	t.Parallel()
	catalog, err := cases.Catalog()
	require.NoError(t, err)
	profiles, err := profile.Load(catalog)
	require.NoError(t, err)

	scenarios := scenariosFor(catalog, "OAI-AUTH-001", testcase.StatusPass)
	evaluation, err := Evaluate([]string{"oai-core"}, profiles, catalog, scenarios)
	require.NoError(t, err)
	assert.Equal(t, result.VerdictIndeterminate, evaluation[0].Verdict)
	assert.Less(t, evaluation[0].CoverageRatio, 1.0)

	scenarios = append(scenarios, scenariosFor(catalog, "OAI-MODL-001", testcase.StatusFail)...)
	evaluation, err = Evaluate([]string{"oai-core"}, profiles, catalog, scenarios)
	require.NoError(t, err)
	assert.Equal(t, result.VerdictFail, evaluation[0].Verdict)
}

func scenariosFor(catalog interface {
	Get(string) (testcase.Definition, bool)
}, id string, status testcase.Status) []result.Scenario {
	definition, _ := catalog.Get(id)
	scenario := result.Scenario{ID: id}
	for _, assertion := range definition.Assertions {
		scenario.Assertions = append(scenario.Assertions, result.Assertion{ID: id + "/" + assertion.ID, Status: status})
	}
	return []result.Scenario{scenario}
}
