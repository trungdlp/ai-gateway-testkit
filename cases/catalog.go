package cases

import (
	"github.com/trungdlp/ai-gateway-testkit/cases/agents"
	"github.com/trungdlp/ai-gateway-testkit/cases/anthropic"
	"github.com/trungdlp/ai-gateway-testkit/cases/behavioral"
	"github.com/trungdlp/ai-gateway-testkit/cases/openai"
	"github.com/trungdlp/ai-gateway-testkit/internal/catalog"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func Definitions() []testcase.Definition {
	definitions := make([]testcase.Definition, 0, 14)
	definitions = append(definitions, openai.Definitions()...)
	definitions = append(definitions, anthropic.Definitions()...)
	definitions = append(definitions, behavioral.Definitions()...)
	definitions = append(definitions, agents.Definitions()...)
	return definitions
}

func Catalog() (*catalog.Catalog, error) {
	return catalog.New(Definitions())
}
