package cache

import "strings"

func (s *saver) canSkipSave(keyTemplate, evaluatedKey string, onlyCheckCacheKey bool) (canSkip bool, reason string) {
	if keyTemplate == evaluatedKey {
		return false, "key is not dynamic; the expectation is that the same key is used for saving different cache contents over and over"
	}

	cacheHits := s.getCacheHits()
	if len(cacheHits) == 0 {
		return false, "no cache was restored in the workflow, creating a new cache entry"
	}

	if _, ok := cacheHits[evaluatedKey]; ok {
		if onlyCheckCacheKey {
			return true, "a cache with the same key was restored in the workflow, new cache would have the same content"
		} else {
			return false, "a cache with the same key was restored in the workflow, but contents might have changed since then"
		}
	}

	return false, "there was no cache restore in the workflow with this key"
}

func (s *saver) canSkipUpload(newCacheKey, newCacheChecksum string) (canSkip bool, reason string) {
	cacheHits := s.getCacheHits()

	if len(cacheHits) == 0 {
		return false, "no cache was restored in the workflow"
	}

	if cacheHits[newCacheKey] == newCacheChecksum {
		return true, "new cache archive is the same as the restored one"
	} else {
		return false, "new cache archive doesn't match the restored one"
	}
}

// Returns cache hit information exposed by previous restore cache steps.
// The returned map's key is the restored cache key, and the value is the checksum of the cache archive
func (s *saver) getCacheHits() map[string]string {
	cacheHits := map[string]string{}
	for _, e := range s.envRepo.List() {
		envParts := strings.SplitN(e, "=", 2)
		if len(envParts) < 2 {
			continue
		}
		envKey := envParts[0]
		envValue := envParts[1]

		if strings.HasPrefix(envKey, cacheHitEnvVarPrefix) {
			cacheKey := strings.TrimPrefix(envKey, cacheHitEnvVarPrefix)
			cacheHits[cacheKey] = envValue
		}
	}
	return cacheHits
}
