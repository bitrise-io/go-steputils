package commandhelper_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/commandhelper"
	"github.com/bitrise-io/go-steputils/v2/export"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/stretchr/testify/require"
)

// setupEnvman mirrors export.SetupEnvman from the export package's test
// helpers (which isn't exported across packages). Points envman at a
// per-test .envstore.yml so export calls don't fail.
func setupEnvman(t *testing.T) {
	t.Helper()
	originalWorkDir, err := os.Getwd()
	require.NoError(t, err)

	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(originalWorkDir) })

	storePath := filepath.Join(tmpDir, ".envstore.yml")
	require.NoError(t, os.WriteFile(storePath, []byte(""), 0777))
	t.Setenv("ENVMAN_ENVSTORE_PATH", storePath)
}

func Test_RunAndExportOutputWithReturningLastNLines(t *testing.T) {
	factory := command.NewFactory(env.NewRepository())
	e := export.NewExporter(factory, export.NewFileManager())

	scenarios := []struct {
		name          string
		args          []string
		numberOfLines int
		wantOutput    string
	}{
		{name: "zero lines requested", args: []string{"testing"}, numberOfLines: 0, wantOutput: ""},
		{name: "single line", args: []string{"testing"}, numberOfLines: 1, wantOutput: "testing"},
		{name: "last of many", args: []string{"my very\nelaborate\ntesting"}, numberOfLines: 1, wantOutput: "testing"},
		{name: "all lines", args: []string{"my very\nelaborate\ntesting"}, numberOfLines: 3, wantOutput: "my very\nelaborate\ntesting"},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			setupEnvman(t)
			tmpFile := filepath.Join(t.TempDir(), "out.log")
			cmd := factory.Create("echo", sc.args, nil)

			got, cmdErr, exportErr := commandhelper.RunAndExportOutputWithReturningLastNLines(cmd, e, tmpFile, "TEST_KEY", sc.numberOfLines)
			require.NoError(t, cmdErr)
			require.NoError(t, exportErr)
			require.Equal(t, sc.wantOutput, got)
		})
	}
}
