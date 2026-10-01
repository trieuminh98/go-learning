package env

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// GetString retrieves the environment variable value for key.
// If the variable is unset or empty, fallback is returned.
func GetString(key, fallback string) string {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return fallback
	}
	return val
}

// GetInt retrieves the environment variable value for key parsed as an integer.
// If the variable is unset, empty, or cannot be parsed as an int, fallback is returned.
func GetInt(key string, fallback int) int {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}

// GetBool retrieves the environment variable value for key parsed as a boolean.
// Accepts 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False.
// If the variable is unset, empty, or cannot be parsed as a bool, fallback is returned.
func GetBool(key string, fallback bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return fallback
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return b
}

// GetDuration retrieves the environment variable value for key parsed as a time.Duration.
// It parses standard duration strings (e.g. "15m", "1h", "300s"). If parsing as a duration string fails,
// it attempts to parse as integer seconds. If variable is unset or invalid, fallback is returned.
func GetDuration(key string, fallback time.Duration) time.Duration {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return fallback
	}
	d, err := time.ParseDuration(val)
	if err == nil {
		return d
	}
	if sec, err := strconv.Atoi(val); err == nil {
		return time.Duration(sec) * time.Second
	}
	return fallback
}

// GetStringSlice retrieves the environment variable value for key parsed as a comma-separated slice.
// If the variable is unset, empty, or whitespace-only, fallback is returned.
func GetStringSlice(key string, fallback []string) []string {
	val, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(val) == "" {
		return fallback
	}
	parts := strings.Split(val, ",")
	var result []string
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}
