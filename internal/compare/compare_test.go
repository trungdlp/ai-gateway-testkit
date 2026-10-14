package compare

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/trungdlp/ai-gateway-testkit/internal/result"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func TestReportsClassifiesAssertionChanges(t *testing.T) {
	t.Parallel()
	baseline := sampleReport("digest", testcase.StatusPass, testcase.StatusFail)
	current := sampleReport("digest", testcase.StatusFail, testcase.StatusPass)

	comparison := Reports(baseline, current)

	assert.True(t, comparison.Comparable)
	assert.Equal(t, NewFailure, comparison.Changes[0].State)
	assert.Equal(t, Resolved, comparison.Changes[1].State)
}

func TestReportsRejectsDifferentCatalogDigests(t *testing.T) {
	t.Parallel()
	comparison := Reports(sampleReport("one", testcase.StatusPass, testcase.StatusPass), sampleReport("two", testcase.StatusFail, testcase.StatusPass))
	assert.False(t, comparison.Comparable)
	assert.Equal(t, NotComparable, comparison.Changes[0].State)
}

func sampleReport(digest string, first, second testcase.Status) result.Report {
	return result.Report{CatalogDigest: digest, Scenarios: []result.Scenario{{Assertions: []result.Assertion{{ID: "OAI-RESP-001/A01", Status: first}, {ID: "OAI-RESP-001/A02", Status: second}}}}}
}

func TestReportsRejectsDifferentAuthenticationExecutionPolicies(t *testing.T) {
	baseline := sampleReport("same", testcase.StatusBlocked, testcase.StatusBlocked)
	current := sampleReport("same", testcase.StatusPass, testcase.StatusPass)
	current.Runtime.SkipAuthChecks = true
	comparison := Reports(baseline, current)
	assert.False(t, comparison.Comparable)
	for _, change := range comparison.Changes {
		assert.Equal(t, NotComparable, change.State)
	}
	baseline.Runtime.SkipAuthChecks = true
	assert.True(t, Reports(baseline, current).Comparable)
}
