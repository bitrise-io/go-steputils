package cache

import (
	"os"
	"testing"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/stretchr/testify/require"
)

const testEnvVarContent = `/tmp/mypath -> /tmp/mypath/cachefile
/tmp/otherpath
/tmp/anotherpath
/tmp/othercache
/somewhere/else`

func TestCacheEnvVarFunctions(t *testing.T) {
	t.Log("Init envman")
	{
		defer func() {
			require.NoError(t, os.RemoveAll("./.envstore.yml"))
		}()
		exists, err := pathutil.IsPathExists("./.envstore.yml")
		require.NoError(t, err)

		if !exists {
			cmd := command.New("envman", "init")
			out, err := cmd.RunAndReturnTrimmedCombinedOutput()
			require.NoError(t, err, out)
		}
	}

	t.Log("Test - AppendToCacheEnvVar")
	{
		err := AppendToCacheEnvVar("/tmp/mypath -> /tmp/mypath/cachefile", "/tmp/otherpath", "/tmp/anotherpath")
		require.NoError(t, err)

		err = AppendToCacheEnvVar("/tmp/othercache")
		require.NoError(t, err)

		err = AppendToCacheEnvVar("/somewhere/else")
		require.NoError(t, err)
	}

	t.Log("Test - GetFromCacheEnvVar")
	{
		content, err := GetFromCacheEnvVar()
		require.NoError(t, err)
		require.Equal(t, testEnvVarContent, content)
	}
}
