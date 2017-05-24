package cache

import "github.com/bitrise-tools/go-steputils/tools"
import "os"
import "strings"

// GlobalCachePathsEnvironmentKey ...
const GlobalCachePathsEnvironmentKey = "BITRISE_CACHE_INCLUDE_PATHS"

// GlobalCacheIgnorePathsEnvironmentKey ...
const GlobalCacheIgnorePathsEnvironmentKey = "BITRISE_CACHE_EXCLUDE_PATHS"

// Cache ...
type Cache struct {
	include []string
	exclude []string
}

// New ...
func New() Cache {
	return Cache{}
}

// IncludePath ...
func (cache *Cache) IncludePath(item string) {
	cache.include = append(cache.include, item)
}

// ExcludePath ...
func (cache *Cache) ExcludePath(item string) {
	cache.exclude = append(cache.exclude, item)
}

// Commit ...
func (cache *Cache) Commit() error {
	err := appendCacheItem(cache.include...)
	if err != nil {
		return err
	}
	err = appendCacheIgnoreItem(cache.exclude...)
	if err != nil {
		return err
	}
	return nil
}

func appendCacheItem(values ...string) error {
	return combineEnvContent(GlobalCachePathsEnvironmentKey, values...)
}

func appendCacheIgnoreItem(values ...string) error {
	return combineEnvContent(GlobalCacheIgnorePathsEnvironmentKey, values...)
}

// GetIncludedPaths ...
func GetIncludedPaths() []string {
	list := GetListOfIncludedPaths()
	if list == "" {
		return nil
	}
	return strings.Split(list, "\n")
}

// GetExcludedPaths ...
func GetExcludedPaths() []string {
	list := GetListOfExcludedPaths()
	if list == "" {
		return nil
	}
	return strings.Split(list, "\n")
}

// GetListOfIncludedPaths ...
func GetListOfIncludedPaths() string {
	return os.Getenv(GlobalCachePathsEnvironmentKey)
}

// GetListOfExcludedPaths ...
func GetListOfExcludedPaths() string {
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
