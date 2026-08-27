package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.env")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, int32(30), cfg.TurnTimeoutSeconds)
	assert.Equal(t, int32(5), cfg.TurnSyncIntervalSeconds)
	assert.Equal(t, int32(5), cfg.MinTurnTimeoutSec)
	assert.Equal(t, int32(120), cfg.MaxTurnTimeoutSec)
	assert.Equal(t, int32(1), cfg.MinTurnSyncIntervalSec)
	assert.Equal(t, int32(60), cfg.MaxTurnSyncIntervalSec)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, ":8080", cfg.Address())
}

func TestLoadFromEnvFile(t *testing.T) {
	path := writeEnvFile(t, `
# comments and blank lines are fine
KUSOKURAE_TURN_TIMEOUT_SECONDS=45
KUSOKURAE_TURN_SYNC_INTERVAL_SECONDS=10
KUSOKURAE_MAX_TURN_TIMEOUT_SECONDS=180
KUSOKURAE_PORT=9000
`)
	t.Setenv("KUSOKURAE_ENV_FILE", path)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, int32(45), cfg.TurnTimeoutSeconds)
	assert.Equal(t, int32(10), cfg.TurnSyncIntervalSeconds)
	assert.Equal(t, int32(180), cfg.MaxTurnTimeoutSec)
	assert.Equal(t, 9000, cfg.Port)
	assert.Equal(t, ":9000", cfg.Address())
}

func TestRealEnvOverridesEnvFile(t *testing.T) {
	path := writeEnvFile(t, "KUSOKURAE_TURN_TIMEOUT_SECONDS=40\n")
	t.Setenv("KUSOKURAE_ENV_FILE", path)
	t.Setenv("KUSOKURAE_TURN_TIMEOUT_SECONDS", "45")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, int32(45), cfg.TurnTimeoutSeconds)
}

func TestMissingDefaultEnvFileIsFine(t *testing.T) {
	// No KUSOKURAE_ENV_FILE and no .env in the test working directory:
	// Load must succeed on built-in defaults.
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, int32(30), cfg.TurnTimeoutSeconds)
}

func TestMissingExplicitEnvFileFails(t *testing.T) {
	t.Setenv("KUSOKURAE_ENV_FILE", filepath.Join(t.TempDir(), "nope.env"))
	_, err := Load()
	require.Error(t, err)
}

func TestInvalidIntegerFails(t *testing.T) {
	t.Setenv("KUSOKURAE_TURN_TIMEOUT_SECONDS", "abc")
	_, err := Load()
	require.Error(t, err)
}

func TestInvalidPortFails(t *testing.T) {
	for _, v := range []string{"0", "65536", "not-a-port"} {
		t.Setenv("KUSOKURAE_PORT", v)
		_, err := Load()
		require.Error(t, err, "port %q must be rejected", v)
	}
}

func TestCrossFieldValidation(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
	}{
		{"default-below-min", map[string]string{
			"KUSOKURAE_MIN_TURN_TIMEOUT_SECONDS": "60",
			"KUSOKURAE_TURN_TIMEOUT_SECONDS":     "30",
		}},
		{"default-above-max", map[string]string{
			"KUSOKURAE_MAX_TURN_TIMEOUT_SECONDS": "20",
			"KUSOKURAE_TURN_TIMEOUT_SECONDS":     "30",
		}},
		{"min-not-below-max", map[string]string{
			"KUSOKURAE_MIN_TURN_TIMEOUT_SECONDS": "120",
			"KUSOKURAE_MAX_TURN_TIMEOUT_SECONDS": "5",
		}},
		{"sync-default-not-below-timeout", map[string]string{
			"KUSOKURAE_TURN_SYNC_INTERVAL_SECONDS": "30",
			"KUSOKURAE_TURN_TIMEOUT_SECONDS":       "30",
		}},
		{"min-sync-not-below-max-sync", map[string]string{
			"KUSOKURAE_MIN_TURN_SYNC_INTERVAL_SECONDS": "60",
			"KUSOKURAE_MAX_TURN_SYNC_INTERVAL_SECONDS": "1",
		}},
		{"min-not-positive", map[string]string{
			"KUSOKURAE_MIN_TURN_TIMEOUT_SECONDS": "0",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := Load()
			require.Error(t, err)
		})
	}
}
