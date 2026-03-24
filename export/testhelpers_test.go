package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ---------
// Bridging test file between export and export_test packages for helpers.
// This enables export_test.go to be a gray box test rather than white box.
// ---------

func CreateSrcDirWithFiles(t *testing.T, baseDir string, fileNames []string) string {
	t.Helper()
	srcDir := filepath.Join(baseDir, "src-dir")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	for _, name := range fileNames {
		sourceFile := filepath.Join(srcDir, name)
		require.NoError(t, os.WriteFile(sourceFile, []byte(name), 0755))
	}
	return srcDir
}

func RequireEnvmanContainsValueForKey(t *testing.T, key, value string, secret bool, envmanStorePath string) {
	t.Helper()
	b, err := os.ReadFile(envmanStorePath)
	require.NoError(t, err)
	envstoreContent := string(b)

	t.Logf("envstoreContent: %s\n", envstoreContent)
	require.Equal(t, true, strings.Contains(envstoreContent, "- "+key+": "+value), envstoreContent)

	if secret {
		require.Equal(t, true, strings.Contains(envstoreContent, "is_sensitive: true"), envstoreContent)
	}
}

func SetupEnvman(t *testing.T) string {
	t.Helper()
	originalWorkDir, err := os.Getwd()
	require.NoError(t, err)

	tmpDir := t.TempDir()
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = os.Chdir(originalWorkDir)
		require.NoError(t, err)
	})
	require.NoError(t, err)

	tmpEnvStorePth := filepath.Join(tmpDir, ".envstore.yml")
	require.NoError(t, os.WriteFile(tmpEnvStorePth, []byte(""), 0777))

	t.Setenv("ENVMAN_ENVSTORE_PATH", tmpEnvStorePth)

	return tmpEnvStorePth
}

