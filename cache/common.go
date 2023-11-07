package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/bitrise-io/go-utils/v2/log"
	"io"
	"os"
	"strconv"
	"strings"
)

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

type Feature string

const (
	RemoteDownloader  Feature = "REMOTE_DOWNLOADER"
	MultipartUploader Feature = "MULTIPART_UPLOADER"
)

func isValidFeature(f string) bool {
	switch Feature(f) {
	case RemoteDownloader, MultipartUploader:
		return true
	default:
		return false
	}
}

func parseFeatureFlags(flagStr string, logger log.Logger) map[Feature]bool {
	featureMap := make(map[Feature]bool)
	featureSettings := strings.Split(flagStr, ",")
	for _, setting := range featureSettings {
		parts := strings.SplitN(strings.TrimSpace(setting), "=", 2)
		if len(parts) != 2 {
			logger.Printf("invalid feature flag format: %s", setting)
		}
		if !isValidFeature(parts[0]) {
			logger.Printf("invalid feature: %s", parts[0])
			continue
		}
		value, err := strconv.ParseBool(parts[1])
		if err != nil {
			logger.Printf("invalid boolean value for %s feature: %s", parts[0], parts[1])
		}
		featureMap[Feature(parts[0])] = value
	}
	return featureMap
}
