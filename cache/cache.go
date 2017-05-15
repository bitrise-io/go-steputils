package cache

import "github.com/bitrise-tools/go-steputils/tools"

// GlobalCacheEnvironmentKey ...
const GlobalCacheEnvironmentKey = "BITRISE_GLOBAL_CACHE"

// AppendCacheItem ...
func AppendCacheItem(values ...string) error {
	content, err := tools.GetEnvironmentValueWithEnvman(GlobalCacheEnvironmentKey)
	if err != nil {
		return err
	}

	for _, line := range values {
		if content == "" {
			content += line
		} else {
			content += "\n" + line
		}
	}

	if err := tools.ExportEnvironmentWithEnvman(GlobalCacheEnvironmentKey, content); err != nil {
		return err
	}

	return nil
}

// GetCacheItems ...
func GetCacheItems() (string, error) {
	content, err := tools.GetEnvironmentValueWithEnvman(GlobalCacheEnvironmentKey)
	if err != nil {
		return "", err
	}
	return content, err
}
