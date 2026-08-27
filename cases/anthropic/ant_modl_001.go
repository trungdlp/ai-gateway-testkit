package anthropic

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
		caseutil.Assertion("A03", "Model entries use the model discriminator", testcase.Must, testcase.High),
		caseutil.Assertion("A04", "Configured model is discoverable", testcase.Must, testcase.High),
	)
	definition := base("ANT-MODL-001", "Anthropic model discovery", "Validates model entry discriminators and configured model discovery.", "models", assertions, nil)
	definition.DependsOn = []string{"ANT-AUTH-001"}
	definition.Run = func(ctx context.Context, environment testcase.Environment) testcase.Execution {
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
			Data []struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			results = append(results, testcase.Fail("A02", "JSON.INVALID", err.Error()))
			return caseutil.BlockRemaining(assertions, results, "DEPENDENCY.JSON", "models response is not valid JSON")
		}
		results = append(results, testcase.Pass("A02"))
		validTypes := len(payload.Data) > 0
		found := false
		for _, model := range payload.Data {
			validTypes = validTypes && model.Type == "model"
			found = found || model.ID == target.Model
		}
		results = append(results, testcase.Result("A03", validTypes, "SCHEMA.MODEL_TYPE_MISMATCH", "one or more model entries do not have type model"))
		results = append(results, testcase.Result("A04", found, "MODEL.NOT_DISCOVERABLE", fmt.Sprintf("model %q was not returned", target.Model)))
		return testcase.Execution{Assertions: results}
	}
	register(definition)
}
