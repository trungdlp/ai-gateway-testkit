package caseutil

import (
	"errors"
	"fmt"
	"strings"

	"github.com/trungdlp/ai-gateway-testkit/internal/gateway"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func Assertions(values ...testcase.AssertionDefinition) []testcase.AssertionDefinition { return values }

func Assertion(id, title string, requirement testcase.Requirement, impact testcase.Impact) testcase.AssertionDefinition {
	return testcase.AssertionDefinition{ID: id, Title: title, Requirement: requirement, Impact: impact}
}

func ErrorAll(assertions []testcase.AssertionDefinition, reason, message string) testcase.Execution {
	results := make([]testcase.AssertionResult, 0, len(assertions))
	for _, assertion := range assertions {
		results = append(results, testcase.Error(assertion.ID, reason, message))
	}
	return testcase.Execution{Assertions: results}
}

func FailAll(assertions []testcase.AssertionDefinition, reason, message string) testcase.Execution {
	results := make([]testcase.AssertionResult, 0, len(assertions))
	for _, assertion := range assertions {
		results = append(results, testcase.Fail(assertion.ID, reason, message))
	}
	return testcase.Execution{Assertions: results}
}

func RequestError(id string, err error) testcase.AssertionResult {
	var transient *gateway.TransientError
	if errors.As(err, &transient) {
		return testcase.Error(id, "HTTP.TRANSIENT_EXHAUSTED", transient.Error())
	}
	return testcase.Error(id, "HTTP.REQUEST_FAILED", err.Error())
}

func RequestErrorAll(assertions []testcase.AssertionDefinition, err error) testcase.Execution {
	var transient *gateway.TransientError
	if errors.As(err, &transient) {
		return ErrorAll(assertions, "HTTP.TRANSIENT_EXHAUSTED", transient.Error())
	}
	return ErrorAll(assertions, "HTTP.REQUEST_FAILED", err.Error())
}

func NotApplicableAll(assertions []testcase.AssertionDefinition, protocol string) testcase.Execution {
	results := make([]testcase.AssertionResult, 0, len(assertions))
	for _, assertion := range assertions {
		results = append(results, testcase.NotApplicable(assertion.ID, protocol+" target is not configured"))
	}
	return testcase.Execution{Assertions: results}
}

func BlockRemaining(assertions []testcase.AssertionDefinition, completed []testcase.AssertionResult, reason, message string) testcase.Execution {
	seen := make(map[string]struct{}, len(completed))
	for _, assertion := range completed {
		seen[assertion.ID] = struct{}{}
	}
	for _, assertion := range assertions {
		if _, ok := seen[assertion.ID]; !ok {
			completed = append(completed, testcase.Blocked(assertion.ID, reason, message))
		}
	}
	return testcase.Execution{Assertions: completed}
}

func UnexpectedStatus(response gateway.Response) string {
	body := Truncate(string(response.Body), 240)
	if body == "" {
		return fmt.Sprintf("expected HTTP 200, got %d", response.StatusCode)
	}
	return fmt.Sprintf("expected HTTP 200, got %d: %s", response.StatusCode, body)
}

func Truncate(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
