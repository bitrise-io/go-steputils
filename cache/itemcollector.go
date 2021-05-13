package cache

// Level defines the extent to which caching should be used.
// - LevelNone: no caching
// - LevelDeps: only dependencies will be cached
// - LevelAll: caching will include gradle and android build cache
type Level string

// Cache level
const (
	LevelNone = Level("none")
	LevelDeps = Level("only_deps")
	LevelAll  = Level("all")
)

// ItemCollector ...
type ItemCollector interface {
	Collect(dir string, cacheLevel Level) ([]string, []string, error)
}
