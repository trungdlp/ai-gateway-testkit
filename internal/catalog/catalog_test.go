package catalog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trungdlp/ai-gateway-testkit/internal/testcase"
)

func TestCatalogValidation(t *testing.T) {
	t.Parallel()

	definition := validDefinition("OAI-RESP-001")
	c, err := New([]testcase.Definition{definition})
	require.NoError(t, err)
	assert.NotEmpty(t, c.Digest())
	_, ok := c.Assertion("OAI-RESP-001/A01")
	assert.True(t, ok)
}

func TestCatalogRejectsDuplicateAndCycles(t *testing.T) {
	t.Parallel()

	definition := validDefinition("OAI-RESP-001")
	_, err := New([]testcase.Definition{definition, definition})
	assert.ErrorContains(t, err, "duplicate")

	first := validDefinition("OAI-RESP-001")
	second := validDefinition("OAI-RESP-002")
	first.DependsOn = []string{second.ID}
	second.DependsOn = []string{first.ID}
	_, err = New([]testcase.Definition{first, second})
	assert.ErrorContains(t, err, "cycle")
}

func validDefinition(id string) testcase.Definition {
	return testcase.Definition{
		ID:          id,
		Revision:    1,
		Title:       "title",
		Description: "description",
		Layer:       testcase.LayerProtocol,
		Suite:       "openai",
		Area:        "responses",
		Stability:   testcase.StabilityStable,
		Determinism: testcase.Deterministic,
		Assertions: []testcase.AssertionDefinition{{
			ID: "A01", Title: "assertion", Requirement: testcase.Must, Impact: testcase.Blocker,
		}},
		Run: func(context.Context, testcase.Environment) testcase.Execution { return testcase.Execution{} },
	}
}
