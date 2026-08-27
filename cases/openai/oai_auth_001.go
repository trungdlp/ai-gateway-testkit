package openai

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
	definition := base("OAI-AUTH-001", "OpenAI authentication boundary", "Verifies rejection and non-reflection of missing and invalid bearer credentials.", "auth", assertions, func(ctx context.Context, environment testcase.Environment) testcase.Execution {
		if _, ok := environment.Target(protocol); !ok {
			return caseutil.NotApplicableAll(assertions, protocol)
		}
		missing, missingErr := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodGet, Path: "/models", Auth: testcase.AuthNone})
		invalid, invalidErr := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodGet, Path: "/models", Auth: testcase.AuthInvalid})
		results := make([]testcase.AssertionResult, 0, 3)
		if missingErr != nil {
			results = append(results, caseutil.RequestError("A01", missingErr))
		} else {
			ok := missing.StatusCode == http.StatusUnauthorized || missing.StatusCode == http.StatusForbidden
			results = append(results, testcase.Result("A01", ok, "AUTH.MISSING_ACCEPTED", fmt.Sprintf("expected HTTP 401 or 403, got %d", missing.StatusCode)))
		}
		if invalidErr != nil {
			results = append(results, caseutil.RequestError("A02", invalidErr))
		} else {
			ok := invalid.StatusCode == http.StatusUnauthorized || invalid.StatusCode == http.StatusForbidden
			results = append(results, testcase.Result("A02", ok, "AUTH.INVALID_ACCEPTED", fmt.Sprintf("expected HTTP 401 or 403, got %d", invalid.StatusCode)))
		}
		if missingErr != nil || invalidErr != nil {
			results = append(results, testcase.Error("A03", "AUTH.EVIDENCE_UNAVAILABLE", "authentication response evidence is incomplete"))
		} else {
			reflected := strings.Contains(string(missing.Body), "agtk-invalid-credential") || strings.Contains(string(invalid.Body), "agtk-invalid-credential")
			results = append(results, testcase.Result("A03", !reflected, "AUTH.CREDENTIAL_REFLECTED", "an error response reflected a supplied credential"))
		}
		return testcase.Execution{Assertions: results}
	})
	definition.Cost.Requests = 2
	register(definition)
}
