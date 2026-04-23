// Package testresultexport exports a step's test result directory to
// $BITRISE_TEST_DEPLOY_DIR with a sidecar test-info.json.
//
// Deprecated: The v2 testreport package is the preferred abstraction for
// new callers. This package exists to keep two legacy consumers
// (bitrise-step-flutter-test, step-custom-test-results-export) working
// against go-steputils/v2 without a larger rewrite. New steps should
// produce a testreport.TestReport directly.
package testresultexport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-utils/v2/fileutil"
)

// ResultDescriptorFileName is the name of the test result descriptor file
// written next to the copied test result directory.
const ResultDescriptorFileName = "test-info.json"

// TestInfo is the payload serialized into test-info.json.
type TestInfo struct {
	Name string `json:"test-name" yaml:"test-name"`
}

// Exporter exports test result directories into a deploy-dir layout that
// Bitrise can pick up.
type Exporter interface {
	ExportTest(name, testResultPath string) error
}

type exporter struct {
	exportPath  string
	fileManager fileutil.FileManager
}

// NewExporter returns an Exporter that writes under exportPath using the
// given FileManager for file operations.
func NewExporter(exportPath string, fileManager fileutil.FileManager) Exporter {
	return &exporter{exportPath: exportPath, fileManager: fileManager}
}

// ExportTest copies the test result directory at testResultPath into
// <exportPath>/<name>/ and writes a sidecar test-info.json describing it.
func (e *exporter) ExportTest(name, testResultPath string) error {
	exportDir := filepath.Join(e.exportPath, name)

	if err := os.MkdirAll(exportDir, os.ModePerm); err != nil {
		return fmt.Errorf("skipping test result (%s): ensure export dir (%s): %w", testResultPath, exportDir, err)
	}

	infoBytes, err := json.Marshal(&TestInfo{Name: name})
	if err != nil {
		return fmt.Errorf("marshal test info: %w", err)
	}
	infoPath := filepath.Join(exportDir, ResultDescriptorFileName)
	if err := e.fileManager.WriteBytes(infoPath, infoBytes); err != nil {
		return fmt.Errorf("write %s: %w", infoPath, err)
	}

	return e.fileManager.CopyDir(testResultPath, exportDir, nil)
}
