package cache

import "github.com/bitrise-tools/go-steputils/tools"
import "os"

// GlobalCachePathsEnvironmentKey ...
const GlobalCachePathsEnvironmentKey = "BITRISE_CACHE_INCLUDE_PATHS"

// GlobalCacheIgnorePathsEnvironmentKey ...
const GlobalCacheIgnorePathsEnvironmentKey = "BITRISE_CACHE_EXCLUDE_PATHS"

// AppendCacheItem ...
func AppendCacheItem(values ...string) error {
	return combineEnvContent(GlobalCachePathsEnvironmentKey, values...)
}

// AppendCacheIgnoreItem ...
func AppendCacheIgnoreItem(values ...string) error {
	return combineEnvContent(GlobalCacheIgnorePathsEnvironmentKey, values...)
}

// GetCacheItems ...
func GetCacheItems() string {
	return os.Getenv(GlobalCachePathsEnvironmentKey)
}

// GetCacheIgnoreItems ...
func GetCacheIgnoreItems() string {
	return os.Getenv(GlobalCacheIgnorePathsEnvironmentKey)
}

func combineEnvContent(envVar string, values ...string) error {
	content := os.Getenv(envVar)
	for _, line := range values {
		if content == "" {
			content += line
		} else {
			content += "\n" + line
		}
	}
	if err := tools.ExportEnvironmentWithEnvman(envVar, content); err != nil {
		return err
	}
	return nil
}
