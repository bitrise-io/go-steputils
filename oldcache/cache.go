package oldcache

import (
	"strings"

	"github.com/bitrise-io/go-steputils/v2/stepenv"
	"github.com/bitrise-io/go-utils/v2/env"
)

// CacheIncludePathsEnvKey's value is a newline separated list of paths that should be included in the cache.
const CacheIncludePathsEnvKey = "BITRISE_CACHE_INCLUDE_PATHS"

// CacheExcludePathsEnvKey's value is a newline separated list of paths that should be excluded from the cache.
const CacheExcludePathsEnvKey = "BITRISE_CACHE_EXCLUDE_PATHS"

// Cache is an interface for managing cache paths.
type Cache interface {
	IncludePath(item ...string)
	ExcludePath(item ...string)
	Commit() error
}

type cache struct {
	envRepository env.Repository

	include []string
	exclude []string
}

// NewDefault creates a new Cache instance that set envs using envman.
func NewDefault() Cache {
	return New(stepenv.NewRepository(env.NewRepository()))
}

// New creates a new Cache instance with a custom env.Repository.
func New(envRepository env.Repository) Cache {
	return &cache{envRepository: envRepository}
}

// IncludePath appends paths to the cache include list.
func (cache *cache) IncludePath(item ...string) {
	cache.include = append(cache.include, item...)
}

// ExcludePath appends paths to the cache exclude list.
func (cache *cache) ExcludePath(item ...string) {
	cache.exclude = append(cache.exclude, item...)
}

// Commit writes paths to environment variables (also exported by envman when using stepenv.Repository).
func (cache *cache) Commit() error {
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
