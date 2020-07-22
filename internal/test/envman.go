package test

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitrise-io/go-utils/envutil"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/stretchr/testify/require"
)

// EnvmanIsSetup ...
func EnvmanIsSetup(t *testing.T) (string, func() error) {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("test")
	require.NoError(t, err)

	tmpDir, err = ioutil.TempDir("", "envman")
	revokeFn, err := pathutil.RevokableChangeDir(tmpDir)
	require.NoError(t, err)

	tmpEnvStorePth := filepath.Join(tmpDir, ".envstore.yml")
	require.NoError(t, fileutil.WriteStringToFile(tmpEnvStorePth, ""))

	envstoreRevokeFn, err := envutil.RevokableSetenv("ENVMAN_ENVSTORE_PATH", tmpEnvStorePth)
	require.NoError(t, err)

	return tmpEnvStorePth, func() error {
		if err := revokeFn(); err != nil {
			return err
		}

		return envstoreRevokeFn()
	}
}

// RequireEnvmanContainsValueForKey ...
func RequireEnvmanContainsValueForKey(t *testing.T, key, value, envmanStorePath string) {
	envstoreContent, err := fileutil.ReadStringFromFile(envmanStorePath)
	require.NoError(t, err)
	t.Logf("envstoreContent: %s\n", envstoreContent)
	require.Equal(t, true, strings.Contains(envstoreContent, "- "+key+": "+value), envstoreContent)
}
