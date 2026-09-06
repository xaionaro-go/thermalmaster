package thermalmaster

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nativeFixtureLog(t *testing.T) string {
	t.Helper()
	base := os.Getenv("THERMALMASTER_NATIVE_FIXTURE_LOG")
	if base == "" {
		t.Skip("requires the deterministic native libusb fixture through LD_PRELOAD")
	}
	path := base + "." + strings.ReplaceAll(t.Name(), "/", "_")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	t.Setenv("THERMALMASTER_NATIVE_FIXTURE_LOG", path)
	return path
}

func nativeFixtureEvents(
	t *testing.T,
	path string,
) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	events := strings.Fields(string(data))
	assert.NotContains(t, events, "unexpected-synchronous-bulk-read")
	assert.NotContains(t, events, "incorrect-control-timeout")
	assert.NotContains(t, events, "manual-detach-0")
	assert.NotContains(t, events, "manual-detach-1")
	return events
}

func assertNativeEventsInOrder(
	t *testing.T,
	events []string,
	want []string,
) {
	t.Helper()
	next := 0
	for _, event := range events {
		if next < len(want) && event == want[next] {
			next++
		}
	}
	assert.Equal(t, len(want), next, "events: %v", events)
}

func countNativeEvents(
	events []string,
	want string,
) int {
	count := 0
	for _, event := range events {
		if event == want {
			count++
		}
	}
	return count
}
