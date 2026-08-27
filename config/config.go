// Package config loads server-wide settings for the online Kusokurae server
// from a .env file and the process environment.
//
// Precedence: real environment variables > .env file > built-in defaults
// (godotenv.Load never overrides a variable already present in the
// environment). A missing key falls back to its built-in default; an invalid
// or contradictory value returns an error so the caller can abort at startup
// instead of silently running with a wrong setting.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

const (
	// DefaultEnvFile is the .env file loaded when KUSOKURAE_ENV_FILE is unset.
	DefaultEnvFile = ".env"

	envFileKey = "KUSOKURAE_ENV_FILE"
)

// Built-in defaults, mirroring the historical server behavior. See
// .env.example at the repository root for the full, commented key list.
const (
	DefaultTurnTimeoutSeconds      = 30
	DefaultTurnSyncIntervalSeconds = 5
	DefaultMinTurnTimeoutSec       = 5
	DefaultMaxTurnTimeoutSec       = 120
	DefaultMinTurnSyncIntervalSec  = 1
	DefaultMaxTurnSyncIntervalSec  = 60
	DefaultPort                    = 8080
)

// Config holds every server-wide setting, resolved from the environment.
type Config struct {
	TurnTimeoutSeconds      int32
	TurnSyncIntervalSeconds int32
	MinTurnTimeoutSec       int32
	MaxTurnTimeoutSec       int32
	MinTurnSyncIntervalSec  int32
	MaxTurnSyncIntervalSec  int32
	Port                    int
}

// Address returns the HTTP listen address for this config (":8080" style).
func (c *Config) Address() string {
	return ":" + strconv.Itoa(c.Port)
}

// Load resolves the configuration from the file named by KUSOKURAE_ENV_FILE
// (default ".env" in the working directory) and the process environment.
// A missing default .env file is fine -- all built-in defaults apply; a
// missing file explicitly named by KUSOKURAE_ENV_FILE is an error.
func Load() (*Config, error) {
	envFile := os.Getenv(envFileKey)
	if envFile == "" {
		envFile = DefaultEnvFile
	}
	fileVars, err := readEnvFile(envFile)
	if err != nil {
		return nil, err
	}

	c := &Config{}
	if c.TurnTimeoutSeconds, err = intKey("KUSOKURAE_TURN_TIMEOUT_SECONDS", DefaultTurnTimeoutSeconds, fileVars); err != nil {
		return nil, err
	}
	if c.TurnSyncIntervalSeconds, err = intKey("KUSOKURAE_TURN_SYNC_INTERVAL_SECONDS", DefaultTurnSyncIntervalSeconds, fileVars); err != nil {
		return nil, err
	}
	if c.MinTurnTimeoutSec, err = intKey("KUSOKURAE_MIN_TURN_TIMEOUT_SECONDS", DefaultMinTurnTimeoutSec, fileVars); err != nil {
		return nil, err
	}
	if c.MaxTurnTimeoutSec, err = intKey("KUSOKURAE_MAX_TURN_TIMEOUT_SECONDS", DefaultMaxTurnTimeoutSec, fileVars); err != nil {
		return nil, err
	}
	if c.MinTurnSyncIntervalSec, err = intKey("KUSOKURAE_MIN_TURN_SYNC_INTERVAL_SECONDS", DefaultMinTurnSyncIntervalSec, fileVars); err != nil {
		return nil, err
	}
	if c.MaxTurnSyncIntervalSec, err = intKey("KUSOKURAE_MAX_TURN_SYNC_INTERVAL_SECONDS", DefaultMaxTurnSyncIntervalSec, fileVars); err != nil {
		return nil, err
	}
	var port int32
	if port, err = intKey("KUSOKURAE_PORT", DefaultPort, fileVars); err != nil {
		return nil, err
	}
	c.Port = int(port)
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func readEnvFile(path string) (map[string]string, error) {
	vars, err := godotenv.Read(path)
	if err == nil {
		return vars, nil
	}
	if path == DefaultEnvFile && os.IsNotExist(err) {
		// No .env in the working directory: run on built-in defaults.
		return nil, nil
	}
	return nil, fmt.Errorf("config: load env file %s: %w", path, err)
}

// intKey resolves key with precedence real environment > .env file > def.
// An explicitly set but invalid value is an error, never a silent fallback.
func intKey(key string, def int32, fileVars map[string]string) (int32, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		v, ok = fileVars[key]
	}
	if !ok {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("config: %s: invalid integer %q", key, v)
	}
	return int32(n), nil
}

// validate cross-checks the resolved values. Every check mirrors an invariant
// the server relies on; an invalid combination is a startup error, never a
// silent fallback.
func (c *Config) validate() error {
	checks := []struct {
		cond bool
		msg  string
	}{
		{c.MinTurnTimeoutSec < 1, "KUSOKURAE_MIN_TURN_TIMEOUT_SECONDS must be >= 1"},
		{c.MinTurnTimeoutSec >= c.MaxTurnTimeoutSec, "KUSOKURAE_MIN_TURN_TIMEOUT_SECONDS must be < KUSOKURAE_MAX_TURN_TIMEOUT_SECONDS"},
		{c.TurnTimeoutSeconds < c.MinTurnTimeoutSec, "KUSOKURAE_TURN_TIMEOUT_SECONDS must be >= KUSOKURAE_MIN_TURN_TIMEOUT_SECONDS"},
		{c.TurnTimeoutSeconds > c.MaxTurnTimeoutSec, "KUSOKURAE_TURN_TIMEOUT_SECONDS must be <= KUSOKURAE_MAX_TURN_TIMEOUT_SECONDS"},
		{c.MinTurnSyncIntervalSec < 1, "KUSOKURAE_MIN_TURN_SYNC_INTERVAL_SECONDS must be >= 1"},
		{c.MinTurnSyncIntervalSec >= c.MaxTurnSyncIntervalSec, "KUSOKURAE_MIN_TURN_SYNC_INTERVAL_SECONDS must be < KUSOKURAE_MAX_TURN_SYNC_INTERVAL_SECONDS"},
		{c.TurnSyncIntervalSeconds < c.MinTurnSyncIntervalSec, "KUSOKURAE_TURN_SYNC_INTERVAL_SECONDS must be >= KUSOKURAE_MIN_TURN_SYNC_INTERVAL_SECONDS"},
		{c.TurnSyncIntervalSeconds > c.MaxTurnSyncIntervalSec, "KUSOKURAE_TURN_SYNC_INTERVAL_SECONDS must be <= KUSOKURAE_MAX_TURN_SYNC_INTERVAL_SECONDS"},
		{c.TurnSyncIntervalSeconds >= c.TurnTimeoutSeconds, "KUSOKURAE_TURN_SYNC_INTERVAL_SECONDS must be < KUSOKURAE_TURN_TIMEOUT_SECONDS"},
		{c.Port < 1 || c.Port > 65535, "KUSOKURAE_PORT must be between 1 and 65535"},
	}
	for _, ch := range checks {
		if ch.cond {
			return fmt.Errorf("config: invalid configuration: %s", ch.msg)
		}
	}
	return nil
}
