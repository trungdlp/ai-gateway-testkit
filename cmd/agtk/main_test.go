package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveBuildInfoUsesCleanVCSRevision(t *testing.T) {
	t.Parallel()
	value := resolveBuildInfo("dev", "unknown", "unknown", map[string]string{
		"vcs.revision": "0123456789012345678901234567890123456789",
		"vcs.time":     "2026-08-27T00:00:00Z",
		"vcs.modified": "false",
	})
	assert.Equal(t, "0123456789012345678901234567890123456789", value.Commit)
	assert.Equal(t, "2026-08-27T00:00:00Z", value.Date)
}

func TestResolveBuildInfoDoesNotPinDirtyRevision(t *testing.T) {
	t.Parallel()
	value := resolveBuildInfo("dev", "unknown", "unknown", map[string]string{
		"vcs.revision": "0123456789012345678901234567890123456789",
		"vcs.modified": "true",
	})
	assert.Equal(t, "unknown", value.Commit)
}
