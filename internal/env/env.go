package env

import (
	"os"
	"strconv"
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
