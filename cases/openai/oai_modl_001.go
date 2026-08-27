package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/trungdlp/ai-gateway-testkit/cases/caseutil"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func init() {
	assertions := caseutil.Assertions(
		caseutil.Assertion("A01", "Models request succeeds", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A02", "Models response is valid JSON", testcase.Must, testcase.Blocker),
		caseutil.Assertion("A03", "Models response uses the list envelope", testcase.Must, testcase.High),
		caseutil.Assertion("A04", "Configured model is discoverable", testcase.Must, testcase.High),
	)
	definition := base("OAI-MODL-001", "OpenAI model discovery", "Validates the models list envelope and configured model discovery.", "models", assertions, func(ctx context.Context, environment testcase.Environment) testcase.Execution {
		target, ok := environment.Target(protocol)
		if !ok {
			return caseutil.NotApplicableAll(assertions, protocol)
		}
		response, err := environment.DoJSON(ctx, testcase.Request{Protocol: protocol, Method: http.MethodGet, Path: "/models", Auth: testcase.AuthValid})
		if err != nil {
			return caseutil.RequestErrorAll(assertions, err)
		}
		results := []testcase.AssertionResult{testcase.Result("A01", response.StatusCode == http.StatusOK, "HTTP.STATUS_UNEXPECTED", caseutil.UnexpectedStatus(response))}
		if response.StatusCode != http.StatusOK {
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.STATUS", "models request did not succeed")
		}
		var payload struct {
			Object string `json:"object"`
			Data   []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			results = append(results, testcase.Fail("A02", "JSON.INVALID", err.Error()))
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.JSON", "models response is not valid JSON")
		}
		results = append(results, testcase.Pass("A02"))
		results = append(results, testcase.Result("A03", payload.Object == "list", "SCHEMA.ENVELOPE_MISMATCH", fmt.Sprintf("object = %q, want list", payload.Object)))
		found := false
		for _, model := range payload.Data {
			found = found || model.ID == target.Model
		}
		results = append(results, testcase.Result("A04", found, "MODEL.NOT_DISCOVERABLE", fmt.Sprintf("model %q was not returned", target.Model)))
		return testcase.Execution{Assertions: results}
	})
	definition.DependsOn = []string{"OAI-AUTH-001"}
	register(definition)
}
