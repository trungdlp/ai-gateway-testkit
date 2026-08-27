package behavioral

import "github.com/trungdlp/ai-gateway-testkit/internal/testcase"

var definitions []testcase.Definition

func register(definition testcase.Definition) {
	definitions = append(definitions, definition)
}

func Definitions() []testcase.Definition {
	return append([]testcase.Definition(nil), definitions...)
}
