package result

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchemaURLPinsFullCommit(t *testing.T) {
	t.Parallel()
	commit := "0123456789012345678901234567890123456789"
	assert.Equal(t, "https://raw.githubusercontent.com/trungdlp/ai-gateway-testkit/"+commit+"/schemas/report.schema.json", SchemaURL(commit))
}

func TestSchemaURLFallsBackToMainForDevelopmentBuild(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "https://raw.githubusercontent.com/trungdlp/ai-gateway-testkit/main/schemas/report.schema.json", SchemaURL("unknown"))
}
