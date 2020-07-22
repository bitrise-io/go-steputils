package commandhelper

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/bitrise-io/go-steputils/internal/test"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/log"
	"github.com/stretchr/testify/require"
)

func Test_RunAndExportOutputWithReturningLastNLines(t *testing.T) {
	scenarios := []struct {
		cmd            *command.Model
		numberOfLines  int
		expectedOutput string
	}{
		{
			cmd:            command.New("echo", "testing"),
			numberOfLines:  0,
			expectedOutput: "",
		},
		{
			cmd:            command.New("echo", "testing"),
			numberOfLines:  1,
			expectedOutput: "testing",
		},
		{
			cmd:            command.New("echo", "testing\ntesting"),
			numberOfLines:  1,
			expectedOutput: "testing",
		},
	}

	for _, scenario := range scenarios {
		// Given
		tmpFile := givenTmpFile(t)
		test.EnvmanIsSetup(t)

		// When
		actualOutput, cmdErr, exportErr := RunAndExportOutputWithReturningLastNLines(*scenario.cmd, tmpFile, "key", scenario.numberOfLines)

		// Then
		require.NoError(t, cmdErr)
		require.NoError(t, exportErr)
		require.Equal(t, scenario.expectedOutput, actualOutput)
	}
}

func Test_RunAndExportOutput(t *testing.T) {
	scenarios := []struct {
		cmd            *command.Model
		numberOfLines  int
		expectedOutput string
	}{
		{
			cmd:            command.New("echo", "testing"),
			numberOfLines:  0,
			expectedOutput: "",
		},
		{
			cmd:            command.New("echo", "testing"),
			numberOfLines:  1,
			expectedOutput: "testing",
		},
		{
			cmd:            command.New("echo", "testing\ntesting"),
			numberOfLines:  1,
			expectedOutput: "testing",
		},
		{
			cmd:            command.New("echo", "testing\ntesting"),
			numberOfLines:  2,
			expectedOutput: "testing\ntesting",
		},
	}

	for _, scenario := range scenarios {
		// Given
		tmpFile := givenTmpFile(t)
		test.EnvmanIsSetup(t)

		// When
		var err error
		actualOutput := captureOuput(t, func() {
			err = RunAndExportOutput(*scenario.cmd, tmpFile, "key", scenario.numberOfLines)
		})

		// Then
		require.NoError(t, err)
		if len(scenario.expectedOutput) > 0 {
			require.Contains(t, actualOutput, "You can find the last couple of lines")
			require.Contains(t, actualOutput, scenario.expectedOutput)
			require.Contains(t, actualOutput, "The log file is stored")
		} else {
			require.NotContains(t, actualOutput, "You can find the last couple of lines")
			require.Contains(t, actualOutput, "The log file is stored")
		}
	}
}

func captureOuput(t *testing.T, fn func()) string {
	tmpFile := givenTmpFile(t)
	fi, err := os.Create(tmpFile)
	require.NoError(t, err)
	log.SetOutWriter(fi)
	defer log.SetOutWriter(os.Stdout)

	fn()

	b, err := ioutil.ReadFile(tmpFile)
	require.NoError(t, err)

	return string(b)
}

func givenTmpFile(t *testing.T) string {
	tmp, err := ioutil.TempDir("", "log")
	require.NoError(t, err)

	return path.Join(tmp, "log.txt")
}
