package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"github.com/bitrise-io/go-steputils/v2/stepconf"
)

type s3CacheConfig struct {
	AWSAcessKeyID      stepconf.Secret
	AWSSecretAccessKey stepconf.Secret
	AWSBucket          string
	AWSRegion          string
}

const cacheHitEnvVar = "BITRISE_CACHE_HIT"

// We need this prefix because there could be multiple restore steps in one workflow with multiple cache keys
const cacheHitUniqueEnvVarPrefix = "BITRISE_CACHE_HIT__"

func checksumOfFile(path string) (string, error) {
	hash := sha256.New()

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck

	_, err = io.Copy(hash, file)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
