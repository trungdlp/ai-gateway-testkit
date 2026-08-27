package compare

import (
	"sort"

	"github.com/trungdlp/ai-gateway-testkit/internal/result"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

const (
	NewFailure       = "new_failure"
	Resolved         = "resolved"
	UnchangedFailure = "unchanged_failure"
	UnchangedPass    = "unchanged_pass"
	NotComparable    = "not_comparable"
)

type Change struct {
	AssertionID string          `json:"assertion_id"`
	Baseline    testcase.Status `json:"baseline,omitempty"`
	Current     testcase.Status `json:"current,omitempty"`
	State       string          `json:"state"`
}

type Comparison struct {
	BaselineRunID string   `json:"baseline_run_id"`
	CurrentRunID  string   `json:"current_run_id"`
	Comparable    bool     `json:"comparable"`
	Changes       []Change `json:"changes"`
}

func Reports(baseline, current result.Report) Comparison {
	comparison := Comparison{BaselineRunID: baseline.RunID, CurrentRunID: current.RunID, Comparable: baseline.CatalogDigest == current.CatalogDigest}
	baselineAssertions := flatten(baseline)
	currentAssertions := flatten(current)
	ids := make(map[string]struct{}, len(baselineAssertions)+len(currentAssertions))
	for id := range baselineAssertions {
		ids[id] = struct{}{}
	}
	for id := range currentAssertions {
		ids[id] = struct{}{}
	}
	for id := range ids {
		before, beforeOK := baselineAssertions[id]
		after, afterOK := currentAssertions[id]
		comparison.Changes = append(comparison.Changes, Change{AssertionID: id, Baseline: before, Current: after, State: classify(before, beforeOK, after, afterOK, comparison.Comparable)})
	}
	sort.Slice(comparison.Changes, func(i, j int) bool { return comparison.Changes[i].AssertionID < comparison.Changes[j].AssertionID })
	return comparison
}

func ApplyBaseline(baseline, current result.Report) result.Report {
	comparison := Reports(baseline, current)
	states := make(map[string]string, len(comparison.Changes))
	for _, change := range comparison.Changes {
		states[change.AssertionID] = change.State
	}
	for scenarioIndex := range current.Scenarios {
		for assertionIndex := range current.Scenarios[scenarioIndex].Assertions {
			assertion := &current.Scenarios[scenarioIndex].Assertions[assertionIndex]
			assertion.BaselineState = states[assertion.ID]
		}
	}
	return current
}

func flatten(value result.Report) map[string]testcase.Status {
	assertions := map[string]testcase.Status{}
	for _, scenario := range value.Scenarios {
		for _, assertion := range scenario.Assertions {
			assertions[assertion.ID] = assertion.Status
		}
	}
	return assertions
}

func classify(before testcase.Status, beforeOK bool, after testcase.Status, afterOK bool, comparable bool) string {
	if !comparable || !beforeOK || !afterOK {
		return NotComparable
	}
	beforeFailure := before != testcase.StatusPass
	afterFailure := after != testcase.StatusPass
	switch {
	case !beforeFailure && afterFailure:
		return NewFailure
	case beforeFailure && !afterFailure:
		return Resolved
	case beforeFailure:
		return UnchangedFailure
	default:
		return UnchangedPass
	}
}
