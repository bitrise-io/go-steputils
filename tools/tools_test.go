package tools

import (
	"os"
	"testing"

	"path/filepath"

	"fmt"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/stretchr/testify/require"
)

func TestExportEnvironmentWithEnvman(t *testing.T) {
	key := "ExportEnvironmentWithEnvmanKey"

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
		// envstor should be clear
		cmd := command.New("envman", "print")
		out, err := cmd.RunAndReturnTrimmedCombinedOutput()
		require.NoError(t, err, out)
		require.Equal(t, "", out)
	}

	value := "test"
	require.NoError(t, ExportEnvironmentWithEnvman(key, value))

	// envstore should contain ExportEnvironmentWithEnvmanKey env var
	cmd := command.New("envman", "print")
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	require.NoError(t, err, out)
	require.Equal(t, fmt.Sprintf("%s: %s", key, value), out)
}
