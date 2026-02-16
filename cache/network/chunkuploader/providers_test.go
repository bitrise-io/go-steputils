package chunkuploader

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestByteSliceChunkProvider(t *testing.T) {
	chunks := [][]byte{
		[]byte("first chunk"),
		[]byte("second chunk with more data"),
		[]byte("third"),
	}

	provider := NewByteSliceChunkProvider(chunks)

	if provider.NumChunks() != 3 {
		t.Errorf("Expected 3 chunks, got %d", provider.NumChunks())
	}

	// Test chunk sizes
	expectedSizes := []int64{11, 27, 5}
	for i, expected := range expectedSizes {
		if provider.ChunkSize(i) != expected {
			t.Errorf("Chunk %d: expected size %d, got %d", i, expected, provider.ChunkSize(i))
		}
	}

	// Test reading chunks
	for i, expectedData := range chunks {
		reader, err := provider.GetChunk(i)
		if err != nil {
			t.Fatalf("GetChunk(%d) error: %v", i, err)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("ReadAll error: %v", err)
		}

		if string(data) != string(expectedData) {
			t.Errorf("Chunk %d: expected %q, got %q", i, expectedData, data)
		}
	}

	// Test out of range
	_, err := provider.GetChunk(-1)
	if err == nil {
		t.Error("Expected error for negative index")
	}

	_, err = provider.GetChunk(3)
	if err == nil {
		t.Error("Expected error for out of range index")
	}
}

func TestFileChunkProvider(t *testing.T) {
	// Create a temp file with test data
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")

	// Write 100 bytes of test data
	testData := make([]byte, 100)
	for i := range testData {
		testData[i] = byte(i)
	}
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create provider with 30-byte chunks (3 full + 1 partial = 4 chunks for 100 bytes)
	// Actually: 30+30+30+10 = 100
	provider, err := NewFileChunkProvider(testFile, 30, 10, 4)
	if err != nil {
		t.Fatalf("NewFileChunkProvider error: %v", err)
	}
	defer provider.Close()

	if provider.NumChunks() != 4 {
		t.Errorf("Expected 4 chunks, got %d", provider.NumChunks())
	}

	// Test chunk sizes
	for i := 0; i < 3; i++ {
		if provider.ChunkSize(i) != 30 {
			t.Errorf("Chunk %d: expected size 30, got %d", i, provider.ChunkSize(i))
		}
	}
	if provider.ChunkSize(3) != 10 {
		t.Errorf("Last chunk: expected size 10, got %d", provider.ChunkSize(3))
	}

	// Test reading all chunks
	var readData []byte
	for i := 0; i < 4; i++ {
		reader, err := provider.GetChunk(i)
		if err != nil {
			t.Fatalf("GetChunk(%d) error: %v", i, err)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("ReadAll error: %v", err)
		}

		readData = append(readData, data...)
	}

	if string(readData) != string(testData) {
		t.Errorf("Read data doesn't match original")
	}
}

func TestStreamChunkProvider(t *testing.T) {
	provider := NewStreamChunkProvider(3, 100)

	if provider.NumChunks() != 3 {
		t.Errorf("Expected 3 chunks, got %d", provider.NumChunks())
	}

	if provider.IsComplete() {
		t.Error("Provider should not be complete before adding chunks")
	}

	if provider.ReceivedCount() != 0 {
		t.Errorf("Expected 0 received, got %d", provider.ReceivedCount())
	}

	// Add chunks out of order
	if err := provider.AddChunk(2, []byte("third")); err != nil {
		t.Fatalf("AddChunk(2) error: %v", err)
	}

	if err := provider.AddChunk(0, []byte("first")); err != nil {
		t.Fatalf("AddChunk(0) error: %v", err)
	}

	if provider.ReceivedCount() != 2 {
		t.Errorf("Expected 2 received, got %d", provider.ReceivedCount())
	}

	if provider.IsComplete() {
		t.Error("Provider should not be complete with only 2 chunks")
	}

	// Try to read incomplete chunk
	_, err := provider.GetChunk(1)
	if err == nil {
		t.Error("Expected error reading incomplete chunk")
	}

	// Add final chunk
	if err := provider.AddChunk(1, []byte("second")); err != nil {
		t.Fatalf("AddChunk(1) error: %v", err)
	}

	if !provider.IsComplete() {
		t.Error("Provider should be complete")
	}

	// Verify all chunks can be read
	expected := []string{"first", "second", "third"}
	for i, exp := range expected {
		reader, err := provider.GetChunk(i)
		if err != nil {
			t.Fatalf("GetChunk(%d) error: %v", i, err)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("ReadAll error: %v", err)
		}

		if string(data) != exp {
			t.Errorf("Chunk %d: expected %q, got %q", i, exp, data)
		}
	}

	// Test out of range
	if err := provider.AddChunk(5, []byte("invalid")); err == nil {
		t.Error("Expected error for out of range index")
	}
}
