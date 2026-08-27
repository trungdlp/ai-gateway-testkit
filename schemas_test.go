package testkit

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaIDsUseRawGitHubContent(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"schemas/report.schema.json": "https://raw.githubusercontent.com/trungdlp/ai-gateway-testkit/main/schemas/report.schema.json",
		"schemas/target.schema.json": "https://raw.githubusercontent.com/trungdlp/ai-gateway-testkit/main/schemas/target.schema.json",
	}
	for path, expected := range tests {
		path, expected := path, expected
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			var document map[string]any
			require.NoError(t, json.Unmarshal(data, &document))
			assert.Equal(t, expected, document["$id"])
		})
	}
}
