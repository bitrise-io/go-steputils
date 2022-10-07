package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// We need this prefix because there could be multiple restore steps in one workflow with multiple cache keys
const cacheHitEnvVarPrefix = "BITRISE_CACHE_HIT__"

func checksumOfFile(path string) (string, error) {
	hash := sha256.New()
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash.Write(b)
	return hex.EncodeToString(hash.Sum(nil)), nil
}
