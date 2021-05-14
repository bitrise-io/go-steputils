package cache_test

import (
	"fmt"

	"github.com/bitrise-io/go-steputils/cache"
)

// Create ItemCollectors for listing Bitrise cache include and exclude patterns.
type SampleItemCollector struct{}

// List of include and exclude patterns are collected in a given directory, cacheLevel describes what files should be included in the cache.
func (c SampleItemCollector) Collect(dir string, cacheLevel cache.Level) ([]string, []string, error) {
	return []string{"include_me.md"}, []string{"exclude_me.txt"}, nil
}

func Example() {
	// Create a cache, usually using cache.New()
	getterSetter := NewMockGetterSetter()
	testConfig := cache.Config{getterSetter, []cache.VariableSetter{getterSetter}}
	c := testConfig.NewCache()

	for _, collector := range []cache.ItemCollector{SampleItemCollector{}} {
		// Run some Cache ItemCollectors
		in, ex, err := collector.Collect("", cache.LevelDeps)
		if err != nil {
			panic(err)
		}

		// Store the include and exclude patterns in the cache
		c.IncludePath(in...)
		c.ExcludePath(ex...)
	}
	// Commit the cache changes
	if err := c.Commit(); err != nil {
		panic(err)
	}

	printIncludeAndExcludeEnvs(getterSetter)
	// Output: include_me.md
	//
	// exclude_me.txt
}

func printIncludeAndExcludeEnvs(getterSetter MockGetterSetter) {
	includePaths, err := getterSetter.Get(cache.CacheIncludePathsEnvKey)
	if err != nil {
		panic(err)
	}
	fmt.Println(includePaths)

	excludePaths, err := getterSetter.Get(cache.CacheExcludePathsEnvKey)
	if err != nil {
		panic(err)
	}
	fmt.Println(excludePaths)
}
