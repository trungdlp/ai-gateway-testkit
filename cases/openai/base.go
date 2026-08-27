package openai

import "github.com/trungdlp/ai-gateway-testkit/internal/testcase"

const protocol = "openai"

func base(id, title, description, area string, assertions []testcase.AssertionDefinition, run testcase.RunFunc) testcase.Definition {
	return testcase.Definition{
		ID: id, Revision: 1, Title: title, Description: description, Layer: testcase.LayerProtocol,
		Suite: protocol, Area: area, Stability: testcase.StabilityStable, Determinism: testcase.Deterministic,
		SpecReferences: []testcase.SpecReference{{Title: "OpenAI API reference", URL: "https://developers.openai.com/api/reference/"}},
		Cost:           testcase.Cost{Requests: 1, MaxInputTokens: 128, MaxOutputTokens: 64},
		Risk:           testcase.Risk{Mutation: "none", ContainsOutput: area == "responses" || area == "tools"},
		Assertions:     assertions, Run: run,
	}
}
