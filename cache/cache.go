package cache

import "github.com/bitrise-tools/go-steputils/tools"

// GlobalCacheEnvironmentKey ...
const GlobalCacheEnvironmentKey = "BITRISE_GLOBAL_CACHE"

// AddRawValueToCacheEnvVar ...
func AddRawValueToCacheEnvVar(value string) error {
	return tools.ExportEnvironmentWithEnvman(GlobalCacheEnvironmentKey, value)
}

// GetRawValueFromCacheEnvVar ...
func GetRawValueFromCacheEnvVar() (string, error) {
	return tools.GetEnvironmentValueWithEnvman(GlobalCacheEnvironmentKey)
}

// AppendRawValueFromCacheEnvVar ...
func AppendRawValueFromCacheEnvVar(value string) (string, error) {
	currentValue, err := GetRawValueFromCacheEnvVar()
	if err != nil {
		return "", err
	}
	return currentValue + value, nil
}

// GetValueListFromCacheEnvVar ...
func GetValueListFromCacheEnvVar() {

}
