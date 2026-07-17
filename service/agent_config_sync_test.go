package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupStringNilAllowsDatabaseFallback(t *testing.T) {
	group := map[string]any{}

	require.Empty(t, groupString(group["role"]))
	require.Equal(t, "primary_mainfield", firstSyncNonEmpty(
		groupString(group["role"]),
		"primary_mainfield",
	))
}
