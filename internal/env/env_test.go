package env_test

import (
	"os"
	"testing"

	"go-learning/internal/env"
)

func TestGetString(t *testing.T) {
	t.Run("returns fallback when key is not set", func(t *testing.T) {
		const key = "TEST_GET_STRING_UNSET"
		_ = os.Unsetenv(key)

		got := env.GetString(key, "fallback-val")
		if got != "fallback-val" {
			t.Errorf("expected fallback-val, got %s", got)
		}
	})

	t.Run("returns fallback when key is empty string", func(t *testing.T) {
		const key = "TEST_GET_STRING_EMPTY"
		t.Setenv(key, "")

		got := env.GetString(key, "fallback-val")
		if got != "fallback-val" {
			t.Errorf("expected fallback-val, got %s", got)
		}
	})

	t.Run("returns env value when key is set", func(t *testing.T) {
		const key = "TEST_GET_STRING_SET"
		t.Setenv(key, ":9090")

		got := env.GetString(key, ":8080")
		if got != ":9090" {
			t.Errorf("expected :9090, got %s", got)
		}
	})
}

func TestGetInt(t *testing.T) {
	t.Run("returns fallback when key is not set", func(t *testing.T) {
		const key = "TEST_GET_INT_UNSET"
		_ = os.Unsetenv(key)

		got := env.GetInt(key, 42)
		if got != 42 {
			t.Errorf("expected 42, got %d", got)
		}
	})

	t.Run("returns fallback when key is empty string", func(t *testing.T) {
		const key = "TEST_GET_INT_EMPTY"
		t.Setenv(key, "")

		got := env.GetInt(key, 42)
		if got != 42 {
			t.Errorf("expected 42, got %d", got)
		}
	})

	t.Run("returns fallback when value cannot be parsed as int", func(t *testing.T) {
		const key = "TEST_GET_INT_INVALID"
		t.Setenv(key, "not-an-integer")

		got := env.GetInt(key, 42)
		if got != 42 {
			t.Errorf("expected 42, got %d", got)
		}
	})

	t.Run("returns parsed int when value is a valid integer", func(t *testing.T) {
		const key = "TEST_GET_INT_VALID"
		t.Setenv(key, "120")

		got := env.GetInt(key, 42)
		if got != 120 {
			t.Errorf("expected 120, got %d", got)
		}
	})
}
