package target

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trungdlp/ai-gateway-testkit/internal/config"
	"github.com/trungdlp/ai-gateway-testkit/internal/gateway"
)

func TestResolveLegacyBoth(t *testing.T) {
	t.Parallel()

	manifest := Legacy("test", config.ProtocolBoth, "https://gateway.example/v1", "model", "GATEWAY_KEY")
	resolved, err := Resolve(manifest, func(name string) (string, bool) {
		assert.Equal(t, "GATEWAY_KEY", name)
		return "secret", true
	}, time.Second, false, gateway.RetryPolicy{MaxRetries: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Second})
	require.NoError(t, err)
	assert.Len(t, resolved.Endpoints, 2)
	assert.NotEmpty(t, resolved.Fingerprint())
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "target.yaml")
	require.NoError(t, os.WriteFile(path, []byte("name: test\nunknown: true\nopenai:\n  base_url: https://example.com/v1\n  model: model\n  credential_env: KEY\n"), 0o600))
	_, err := Load(path)
	assert.ErrorContains(t, err, "field unknown")
}

func TestResolveRejectsInvalidRetryPolicy(t *testing.T) {
	t.Parallel()
	manifest := Legacy("test", config.ProtocolOpenAI, "https://gateway.example/v1", "model", "KEY")
	_, err := Resolve(manifest, func(string) (string, bool) { return "secret", true }, time.Second, false, gateway.RetryPolicy{MaxRetries: 11, InitialBackoff: time.Millisecond, MaxBackoff: time.Second})
	assert.ErrorContains(t, err, "between 0 and 10")
}
