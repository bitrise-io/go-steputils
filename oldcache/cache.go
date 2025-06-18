package oldcache

import (
	"strings"

	"github.com/bitrise-io/go-utils/v2/env"
)

// CacheIncludePathsEnvKey ...
const CacheIncludePathsEnvKey = "BITRISE_CACHE_INCLUDE_PATHS"

// CacheExcludePathsEnvKey ...
const CacheExcludePathsEnvKey = "BITRISE_CACHE_EXCLUDE_PATHS"

// Cache ...
type Cache struct {
	envRepository env.Repository

	include []string
	exclude []string
}

// New ...
func New(envRepository env.Repository) Cache {
	// defaultConfig := Config{NewOSVariableGetter(), []VariableSetter{NewOSVariableSetter(), NewEnvmanVariableSetter()}}
	return Cache{envRepository: envRepository}
}

// IncludePath ...
func (cache *Cache) IncludePath(item ...string) {
	cache.include = append(cache.include, item...)
}

// ExcludePath ...
func (cache *Cache) ExcludePath(item ...string) {
	cache.exclude = append(cache.exclude, item...)
}

// Commit ...
func (cache *Cache) Commit() error {
	commitCachePath := func(key string, values []string) error {
		content := cache.envRepository.Get(key)
		if content != "" {
			content += "\n"
		}

		content += strings.Join(values, "\n")
		content += "\n"

		return cache.envRepository.Set(key, content)
	}

	if err := commitCachePath(CacheIncludePathsEnvKey, cache.include); err != nil {
		return err
	}

	return commitCachePath(CacheExcludePathsEnvKey, cache.exclude)
}
