package main

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/alexedwards/argon2id"
)

// helper parser funciton for argon2id params from env file
func parseUintConfig(key string, bitSize int) (uint64, bool) {
	valStr, exists := os.LookupEnv(key)
	if !exists || valStr == "" {
		return 0, false
	}

	val, err := strconv.ParseUint(valStr, 10, bitSize)
	if err != nil {
		slog.Error("invalid environment variable, expected a positive integer",
			"key", key,
			"value", valStr,
			"error", err)
		os.Exit(1)
	}

	return val, true
}

func loadArgon2idParams() *argon2id.Params {
	// hardcoded fallback baselines used only if environment variables are completely absent
	params := &argon2id.Params{
		Memory:      64 * 1024,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	if val, ok := parseUintConfig("ARGON2ID_MEMORY", 32); ok {
		params.Memory = uint32(val)
	}

	if val, ok := parseUintConfig("ARGON2ID_ITERATIONS", 32); ok {
		params.Iterations = uint32(val)
	}

	if val, ok := parseUintConfig("ARGON2ID_PARALLELISM", 8); ok {
		params.Parallelism = uint8(val)
	}

	if val, ok := parseUintConfig("ARGON2ID_SALT_LENGTH", 32); ok {
		params.SaltLength = uint32(val)
	}

	if val, ok := parseUintConfig("ARGON2ID_KEY_LENGTH", 32); ok {
		params.KeyLength = uint32(val)
	}

	return params
}
