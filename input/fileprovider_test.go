package input

import (
	"errors"
	"testing"

	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/stretchr/testify/assert"
)

func Test_WhenTrimmedFilePathCalled_ThenExpectCorrectValue(t *testing.T) {

	absPath := func(path string) string {
		pth, err := pathutil.AbsPath("file.txt")
		if err != nil {
			return err.Error()
		}

		return pth
	}

	scenarios := []struct {
		filePath string
		expected string
	}{
		{
			filePath: "file://file.txt",
			expected: absPath("file.txt"),
		},
		{
			filePath: "file:///file.txt",
			expected: "/file.txt",
		},
	}

	for _, scenario := range scenarios {
		// Given
		fileProvider := givenFileProvider(scenario.filePath, givenMockFileDownloader())

		// When
		actualFilePath, err := fileProvider.trimmedFilePath()

		// Then

		assert.NoError(t, err)
		assert.Equal(t, scenario.expected, actualFilePath)
	}
}

func Test_WhenFileNameFromPathURLCalled_ThenExpectCorrectValue(t *testing.T) {
	scenarios := []struct {
		input    string
		expected string
	}{
		{
			"https://something.com/best-file-ever.bitrise",
			"best-file-ever.bitrise",
		},
		{
			"https://something.com/otherfile.txt?queryparams",
			"otherfile.txt",
		},
		{
			"https://github.com/bitrise-steplib/awesome-step/archive/0.1.1.zip",
			"0.1.1.zip",
		},
	}

	for _, scenario := range scenarios {
		// Given
		fileProvider := givenFileProvider(scenario.input, givenMockFileDownloader())

		// When
		actualName, err := fileProvider.fileNameFromPathURL()

		// Then
		assert.NoError(t, err)
		assert.Equal(t, scenario.expected, actualName)
	}
}

func Test_GivenLocalFileProvided_WhenLocalPathCalled_ThenExpectLocalfilePath(t *testing.T) {
	// Given
	inputPath := "file:///path/tp/file/meinefile.txt"
	expectedPath := "/path/tp/file/meinefile.txt"
	mockFileDownloader := givenMockFileDownloader()
	fileProvider := givenFileProvider(inputPath, mockFileDownloader)

	// When
	actualPath, err := fileProvider.LocalPath()

	// Then
	assert.NoError(t, err)
	assert.Equal(t, expectedPath, actualPath)
	mockFileDownloader.AssertNotCalled(t, "Get")
}

func Test_GivenRemoteFileProvidedAndDownloadDails_WhenLocalPathCalled_ThenExpectError(t *testing.T) {
	// Given
	inputPath := "https://something.com/best-file-ever.bitrise"
	expectedError := errors.New("some error")
	mockFileDownloader := givenMockFileDownloader().GivenGetFails(expectedError)
	fileProvider := givenFileProvider(inputPath, mockFileDownloader)

	// When
	actualPath, err := fileProvider.LocalPath()

	// Then
	assert.EqualError(t, expectedError, err.Error())
	assert.Empty(t, actualPath)
}

func Test_GivenRemoteFileProvidedAndDownloadSucceeds_WhenLocalPathCalled_ThenPath(t *testing.T) {
	// Given
	inputPath := "https://something.com/best-file-ever.bitrise"
	mockFileDownloader := givenMockFileDownloader().GivenGetSucceed()
	fileProvider := givenFileProvider(inputPath, mockFileDownloader)

	// When
	actualPath, err := fileProvider.LocalPath()

	// Then
	assert.NoError(t, err)
	assert.NotEmpty(t, actualPath)
}

func givenFileProvider(path string, filedownloader FileDownloader) FileProvider {
	return NewFileProvider(path, filedownloader)
}

func givenMockFileDownloader() *MockFileDownloader {
	return new(MockFileDownloader)
}
