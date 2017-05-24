package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/stretchr/testify/require"
)

const testEnvVarContent = `/tmp/mypath -> /tmp/mypath/cachefile
/tmp/otherpath
/tmp/anotherpath
/tmp/othercache
/somewhere/else`

const testIgnoreEnvVarContent = `/*.log
/*.bin
/*.lock`

func TestCacheFunctions(t *testing.T) {

	t.Log("Init envman")
	{
		// envman requires an envstore path to use, or looks for default envstore path: ./.envstore.yml
		workDir, err := pathutil.CurrentWorkingDirectoryAbsolutePath()
		require.NoError(t, err)
		defaultEnvstorePth := filepath.Join(workDir, ".envstore.yml")
		require.NoError(t, fileutil.WriteStringToFile(defaultEnvstorePth, ""))
		defer func() {
			require.NoError(t, os.Remove(defaultEnvstorePth))
		}()
		//

		{
			// envstore should be clear
			cmd := command.New("envman", "clear")
			out, err := cmd.RunAndReturnTrimmedCombinedOutput()
			require.NoError(t, err, out)
			cmd = command.New("envman", "print")
			out, err = cmd.RunAndReturnTrimmedCombinedOutput()
			require.NoError(t, err, out)
			require.Equal(t, "", out)
		}
	}

	t.Log("Test - AppendCacheItem")
	{
		err := AppendCacheItem("/tmp/mypath -> /tmp/mypath/cachefile", "/tmp/otherpath", "/tmp/anotherpath", "/tmp/othercache", "/somewhere/else")
		require.NoError(t, err)
	}

	t.Log("Test - AppendCacheIgnoreItem")
	{
		err := AppendCacheIgnoreItem("/*.log", "/*.bin", "/*.lock")
		require.NoError(t, err)
	}

	t.Log("Test - GetCacheItems")
	{
		content, err := getEnvironmentValueWithEnvman(GlobalCachePathsEnvironmentKey)
		require.NoError(t, err)
		require.Equal(t, testEnvVarContent, content)
	}

	t.Log("Test - GetCacheIgnoreItems")
	{
		content, err := getEnvironmentValueWithEnvman(GlobalCacheIgnorePathsEnvironmentKey)
		require.NoError(t, err)
		require.Equal(t, testIgnoreEnvVarContent, content)
	}
}

func getEnvironmentValueWithEnvman(key string) (string, error) {
	cmd := command.New("envman", "print", "--format", "json")
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
