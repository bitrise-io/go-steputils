package oldcache_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	cache "github.com/bitrise-io/go-steputils/v2/oldcache"
	"github.com/bitrise-io/go-steputils/v2/stepenv"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/stretchr/testify/require"
)

// MockGetterSetter ...
type MockGetterSetter struct {
	values map[string]string
}

// NewMockGetterSetter ...
func NewMockGetterSetter() env.Repository {
	return MockGetterSetter{values: map[string]string{}}
}

// Get ...
func (g MockGetterSetter) Get(key string) string {
	return g.values[key]
}

// Set ...
func (g MockGetterSetter) Set(key, value string) error {
	g.values[key] = value
	return nil
}

// List ...
func (g MockGetterSetter) List() []string {
	var envs []string
	for k, v := range g.values {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}
	return envs
}

// Unset ...
func (g MockGetterSetter) Unset(key string) error {
	delete(g.values, key)
	return nil
}

const testEnvVarContent = `/tmp/mypath -> /tmp/mypath/cachefile
/tmp/otherpath
/tmp/anotherpath
/tmp/othercache
/somewhere/else
`

const testIgnoreEnvVarContent = `/*.log
/*.bin
/*.lock
`

const testThirdCommitIgnoreEnvVarContent = `/*.log
/*.bin
/*.lock

/*.lock

/*.lock
`

func TestCacheFunctions(t *testing.T) {
	filemanager := fileutil.NewFileManager()
	pathmodifier := pathutil.NewPathModifier()
	cacheEnvRepository := stepenv.NewRepository(env.NewRepository())

	t.Log("Init envman")
	{
		// envman requires an envstore path to use, or looks for default envstore path: ./.envstore.yml
		defaultEnvstorePth, err := pathmodifier.AbsPath(".envstore.yml")
		require.NoError(t, err)
		require.NoError(t, filemanager.WriteBytes(defaultEnvstorePth, []byte("")))
		defer func() {
			require.NoError(t, os.Remove(defaultEnvstorePth))
		}()

		{
			// envstore should be clear
			cmdFactory := command.NewFactory(env.NewRepository())
			cmd := cmdFactory.Create("envman", []string{"clear"}, nil)
			out, err := cmd.RunAndReturnTrimmedCombinedOutput()
			require.NoError(t, err, out)
			cmd = cmdFactory.Create("envman", []string{"print"}, nil)
			out, err = cmd.RunAndReturnTrimmedCombinedOutput()
			require.NoError(t, err, out)
			require.Equal(t, "", out)
		}
	}

	t.Log("Test - cache")
	{
		c := cache.New(cacheEnvRepository)
		c.IncludePath("/tmp/mypath -> /tmp/mypath/cachefile")
		c.IncludePath("/tmp/otherpath")
		c.IncludePath("/tmp/anotherpath")
		c.IncludePath("/tmp/othercache")
		c.IncludePath("/somewhere/else")
		c.ExcludePath("/*.log")
		c.ExcludePath("/*.bin")
		c.ExcludePath("/*.lock")
		err := c.Commit()
		require.NoError(t, err)

		content, err := getEnvironmentValueWithEnvman(cache.CacheIncludePathsEnvKey)
		require.NoError(t, err)
		require.Equal(t, testEnvVarContent, content)

		content, err = getEnvironmentValueWithEnvman(cache.CacheExcludePathsEnvKey)
		require.NoError(t, err)
		require.Equal(t, testIgnoreEnvVarContent, content)

		c = cache.New(cacheEnvRepository)
		c.ExcludePath("/*.lock")
		err = c.Commit()
		require.NoError(t, err)

		c = cache.New(cacheEnvRepository)
		c.ExcludePath("/*.lock")
		err = c.Commit()
		require.NoError(t, err)

		content, err = getEnvironmentValueWithEnvman(cache.CacheExcludePathsEnvKey)
		require.NoError(t, err)
		require.Equal(t, testThirdCommitIgnoreEnvVarContent, content)
	}
}

func getEnvironmentValueWithEnvman(key string) (string, error) {
	cmdFactory := command.NewFactory(env.NewRepository())
	cmd := cmdFactory.Create("envman", []string{"print", "--format", "json"}, nil)
	output, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s\n%s", output, err)
	}

	var data map[string]string
	err = json.Unmarshal([]byte(output), &data)
	if err != nil {
		return "", fmt.Errorf("%s\n%s", output, err)
	}

	return data[key], nil
}
