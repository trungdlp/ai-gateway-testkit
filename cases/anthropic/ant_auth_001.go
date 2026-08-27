package anthropic

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/trungdlp/ai-gateway-testkit/cases/caseutil"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func init() {
	assertions := caseutil.Assertions(
		caseutil.Assertion("A01", "Missing credentials are rejected", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A02", "Invalid credentials are rejected", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A03", "Credentials are not reflected in error responses", testcase.Must, testcase.Blocker),
	)
	definition := base("ANT-AUTH-001", "Anthropic authentication boundary", "Verifies rejection and non-reflection of missing and invalid API keys.", "auth", assertions, nil)
	definition.Cost.Requests = 2
	definition.Run = func(ctx context.Context, environment testcase.Environment) testcase.Execution {
		if _, ok := environment.Target(protocol); !ok {
			return caseutil.NotApplicableAll(assertions, protocol)
		}
		missing, missingErr := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodGet, Path: "/models", Auth: testcase.AuthNone})
		invalid, invalidErr := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodGet, Path: "/models", Auth: testcase.AuthInvalid})
		results := make([]testcase.AssertionResult, 0, len(assertions))
		if missingErr != nil {
			results = append(results, caseutil.RequestError("A01", missingErr))
		} else {
			valid := missing.StatusCode == http.StatusUnauthorized || missing.StatusCode == http.StatusForbidden
			results = append(results, testcase.Result("A01", valid, "AUTH.MISSING_ACCEPTED", fmt.Sprintf("expected HTTP 401 or 403, got %d", missing.StatusCode)))
		}
		if invalidErr != nil {
			results = append(results, caseutil.RequestError("A02", invalidErr))
		} else {
			valid := invalid.StatusCode == http.StatusUnauthorized || invalid.StatusCode == http.StatusForbidden
			results = append(results, testcase.Result("A02", valid, "AUTH.INVALID_ACCEPTED", fmt.Sprintf("expected HTTP 401 or 403, got %d", invalid.StatusCode)))
		}
		if missingErr != nil || invalidErr != nil {
			results = append(results, testcase.Error("A03", "AUTH.EVIDENCE_UNAVAILABLE", "authentication response evidence is incomplete"))
		} else {
			reflected := strings.Contains(string(missing.Body), "agtk-invalid-credential") || strings.Contains(string(invalid.Body), "agtk-invalid-credential")
			results = append(results, testcase.Result("A03", !reflected, "AUTH.CREDENTIAL_REFLECTED", "an error response reflected a supplied credential"))
		}
		return testcase.Execution{Assertions: results}
	}
	register(definition)
}
