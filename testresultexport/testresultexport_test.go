package testresultexport_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-steputils/v2/testresultexport"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/stretchr/testify/require"
)

func TestExportTest_writesTestInfoAndCopiesDir(t *testing.T) {
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "result.xml"), []byte("<testsuite/>"), 0644))

	exportRoot := t.TempDir()
	e := testresultexport.NewExporter(exportRoot, fileutil.NewFileManager())

	require.NoError(t, e.ExportTest("suite-a", srcDir))

	destDir := filepath.Join(exportRoot, "suite-a")

	infoBytes, err := os.ReadFile(filepath.Join(destDir, testresultexport.ResultDescriptorFileName))
	require.NoError(t, err)
	var info testresultexport.TestInfo
	require.NoError(t, json.Unmarshal(infoBytes, &info))
	require.Equal(t, "suite-a", info.Name)

	copied, err := os.ReadFile(filepath.Join(destDir, "result.xml"))
	require.NoError(t, err)
	require.Equal(t, "<testsuite/>", string(copied))
}

func TestExportTest_mkdirFails(t *testing.T) {
	// Use a path where a regular file blocks directory creation.
	base := t.TempDir()
	blockingFile := filepath.Join(base, "blocker")
	require.NoError(t, os.WriteFile(blockingFile, []byte{}, 0644))

	e := testresultexport.NewExporter(blockingFile, fileutil.NewFileManager())

	err := e.ExportTest("suite", base)
	require.Error(t, err)
}

func TestResultDescriptorFileName(t *testing.T) {
	require.Equal(t, "test-info.json", testresultexport.ResultDescriptorFileName)
}
