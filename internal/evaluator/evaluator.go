package evaluator

import (
	"sort"

	"github.com/trungdlp/ai-gateway-testkit/internal/catalog"
	"github.com/trungdlp/ai-gateway-testkit/internal/profile"
	"github.com/trungdlp/ai-gateway-testkit/internal/result"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func Evaluate(profileIDs []string, registry *profile.Registry, c *catalog.Catalog, scenarios []result.Scenario) ([]result.Profile, error) {
	byAssertion := make(map[string]testcase.Status)
	for _, scenario := range scenarios {
		for _, assertion := range scenario.Assertions {
			byAssertion[assertion.ID] = assertion.Status
		}
	}
	evaluations := make([]result.Profile, 0, len(profileIDs))
	for _, profileID := range profileIDs {
		definition, _ := registry.Get(profileID)
		selection, err := registry.Expand(profileID, c)
		if err != nil {
			return nil, err
		}
		evaluation := result.Profile{ID: definition.ID, Version: definition.Version, Title: definition.Title, Verdict: result.VerdictPass, Required: len(selection.Required)}
		ids := make([]string, 0, len(selection.Required))
		for id := range selection.Required {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			status, exists := byAssertion[id]
			if !exists {
				evaluation.Indeterminate++
				evaluation.IndeterminateIDs = append(evaluation.IndeterminateIDs, id)
				continue
			}
			switch status {
			case testcase.StatusPass:
				evaluation.Passed++
				evaluation.Evaluated++
			case testcase.StatusFail:
				evaluation.Failed++
				evaluation.Evaluated++
				evaluation.FailureIDs = append(evaluation.FailureIDs, id)
			default:
				evaluation.Indeterminate++
				evaluation.IndeterminateIDs = append(evaluation.IndeterminateIDs, id)
			}
		}
		if evaluation.Failed > 0 {
			evaluation.Verdict = result.VerdictFail
		} else if evaluation.Indeterminate > 0 {
			evaluation.Verdict = result.VerdictIndeterminate
		}
		if evaluation.Evaluated > 0 {
			evaluation.SuccessRatio = float64(evaluation.Passed) / float64(evaluation.Evaluated)
		}
		if evaluation.Required > 0 {
			evaluation.CoverageRatio = float64(evaluation.Evaluated) / float64(evaluation.Required)
		}
		evaluations = append(evaluations, evaluation)
	}
	return evaluations, nil
}
