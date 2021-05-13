package cache_test

import (
	"fmt"

	"github.com/bitrise-io/go-steputils/cache"
)

type SampleItemCollector struct{}

func (c SampleItemCollector) Collect(dir string, cacheLevel cache.Level) ([]string, []string, error) {
	return []string{"include_me.md"}, []string{"exclude_me.txt"}, nil
}

func Example() {
	collectors := []cache.ItemCollector{
		SampleItemCollector{},
	}

	getterSetter := NewMockGetterSetter()
	// use NewDefaultCache usually
	c := cache.New(getterSetter, []cache.VariableSetter{getterSetter})
	for _, collector := range collectors {
		in, ex, err := collector.Collect("", cache.LevelDeps)
		if err != nil {
			panic(err)
		}
		c.IncludePath(in...)
		c.ExcludePath(ex...)
	}
	if err := c.Commit(); err != nil {
		panic(err)
	}

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
	// Output: include_me.md
	// exclude_me.txt
}
