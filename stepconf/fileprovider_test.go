package stepconf

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-utils/v2/filedownloader"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRealFileProvider() FileProvider {
	logger := log.NewLogger()
	downloader := filedownloader.NewDownloader(logger)

	return NewFileProvider(
		downloader,
		fileutil.NewFileManager(),
		pathutil.NewPathProvider(),
		pathutil.NewPathModifier(),
	)
}

func TestFileProvider_LocalPath_FileScheme(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	provider := setupRealFileProvider()

	ctx := context.Background()
	fileURL := "file://" + testFile
	localPath, err := provider.LocalPath(ctx, fileURL)

	require.NoError(t, err)
	assert.Equal(t, testFile, localPath)
	assert.True(t, filepath.IsAbs(localPath), "should return absolute path")

	_, err = os.Stat(localPath)
	assert.NoError(t, err, "file should exist at returned path")
}

func TestFileProvider_LocalPath_FileScheme_RelativePath(t *testing.T) {
	// Create a file in current directory
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { require.NoError(t, os.Chdir(origDir)) }()

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	relPath := "relative/test.txt"
	err = os.MkdirAll(filepath.Dir(relPath), 0755)
	require.NoError(t, err)
	err = os.WriteFile(relPath, []byte("content"), 0644)
	require.NoError(t, err)

	provider := setupRealFileProvider()

	ctx := context.Background()
	fileURL := "file://" + relPath
	localPath, err := provider.LocalPath(ctx, fileURL)

	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(localPath), "should return absolute path")
	assert.Contains(t, localPath, "relative/test.txt")

	_, err = os.Stat(localPath)
	assert.NoError(t, err)
}

func TestFileProvider_LocalPath_HTTPUrl(t *testing.T) {
	expectedContent := []byte("downloaded content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/config.json", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(expectedContent)
		require.NoError(t, err)
	}))
	defer server.Close()

	provider := setupRealFileProvider()

	ctx := context.Background()
	localPath, err := provider.LocalPath(ctx, server.URL+"/config.json")

	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(localPath), "should return absolute path")
	assert.Contains(t, localPath, "config.json", "should preserve filename")

	content, err := os.ReadFile(localPath)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, content)
}

func TestFileProvider_LocalPath_HTTPUrl_WithPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/path/to/file.txt", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("file content"))
		require.NoError(t, err)
	}))
	defer server.Close()

	provider := setupRealFileProvider()

	ctx := context.Background()
	localPath, err := provider.LocalPath(ctx, server.URL+"/path/to/file.txt")

	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(localPath))
	assert.Contains(t, localPath, "file.txt")

	_, err = os.Stat(localPath)
	assert.NoError(t, err)
}

func TestFileProvider_LocalPath_HTTPUrl_404Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte("Not Found"))
		require.NoError(t, err)
	}))
	defer server.Close()

	provider := setupRealFileProvider()

	ctx := context.Background()
	localPath, err := provider.LocalPath(ctx, server.URL+"/notfound.txt")

	require.Error(t, err)
	assert.Empty(t, localPath)
	assert.Contains(t, err.Error(), "status code 404")
}

func TestFileProvider_Contents_FileScheme(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "local file content"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	provider := setupRealFileProvider()

	ctx := context.Background()
	fileURL := "file://" + testFile
	reader, err := provider.Contents(ctx, fileURL)

	require.NoError(t, err)
	require.NotNil(t, reader)
	defer func() { require.NoError(t, reader.Close()) }()

	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}

func TestFileProvider_Contents_HTTPUrl(t *testing.T) {
	expectedContent := "remote file content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(expectedContent))
		require.NoError(t, err)
	}))
	defer server.Close()

	provider := setupRealFileProvider()

	ctx := context.Background()
	reader, err := provider.Contents(ctx, server.URL+"/file.txt")

	require.NoError(t, err)
	require.NotNil(t, reader)
	defer func() { require.NoError(t, reader.Close()) }()

	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, string(content))
}

func TestFileProvider_Contents_HTTPUrl_Streaming(t *testing.T) {
	// Test that large content is streamed, not buffered
	largeContent := make([]byte, 1024*1024) // 1MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(largeContent)
		require.NoError(t, err)
	}))
	defer server.Close()

	provider := setupRealFileProvider()

	ctx := context.Background()
	reader, err := provider.Contents(ctx, server.URL+"/large.bin")

	require.NoError(t, err)
	require.NotNil(t, reader)
	defer func() { require.NoError(t, reader.Close()) }()

	// Read in chunks to verify streaming
	chunk := make([]byte, 4096)
	totalRead := 0
	for {
		n, err := reader.Read(chunk)
		totalRead += n
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}

	assert.Equal(t, len(largeContent), totalRead, "should read entire content")
}

func TestFileProvider_Contents_FileScheme_FileNotFound(t *testing.T) {
	provider := setupRealFileProvider()

	ctx := context.Background()
	fileURL := "file:///nonexistent/file.txt"
	reader, err := provider.Contents(ctx, fileURL)

	require.Error(t, err)
	assert.Nil(t, reader)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestFileProvider_Contents_HTTPUrl_404Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := setupRealFileProvider()

	ctx := context.Background()
	reader, err := provider.Contents(ctx, server.URL+"/notfound.txt")

	require.Error(t, err)
	assert.Nil(t, reader)
	assert.Contains(t, err.Error(), "status code 404")
}
