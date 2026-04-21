package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createTestFile(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_file.txt")

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	return filePath
}

func createTestDirectory(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestReadFile_Success(t *testing.T) {
	content := "This is test content"
	filePath := createTestFile(t, content)

	input := ReadFileInput{Path: filePath}
	inputJSON, _ := json.Marshal(input)

	result, err := RunReadFileTool(context.Background(), inputJSON)

	assert.NoError(t, err)
	assert.Contains(t, result, "1: This is test content")
}

func TestReadFile_NonexistentFile(t *testing.T) {
	input := ReadFileInput{Path: "/nonexistent/file.txt"}
	inputJSON, _ := json.Marshal(input)

	result, err := RunReadFileTool(context.Background(), inputJSON)

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.IsType(t, &fs.PathError{}, err)
}

func TestReadFile_EmptyFile(t *testing.T) {
	filePath := createTestFile(t, "")

	input := ReadFileInput{Path: filePath}
	inputJSON, _ := json.Marshal(input)

	result, err := RunReadFileTool(context.Background(), inputJSON)

	assert.NoError(t, err)
	assert.Contains(t, result, "1: ")
}

func TestReadFile_MultilineFile(t *testing.T) {
	content := "Line 1\nLine 2\nLine 3"
	filePath := createTestFile(t, content)

	input := ReadFileInput{Path: filePath}
	inputJSON, _ := json.Marshal(input)

	result, err := RunReadFileTool(context.Background(), inputJSON)

	assert.NoError(t, err)
	assert.Contains(t, result, "1: Line 1")
	assert.Contains(t, result, "2: Line 2")
	assert.Contains(t, result, "3: Line 3")
}

func TestReadFile_LineRange(t *testing.T) {
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("Line %d", i))
	}
	content := strings.Join(lines, "\n")
	filePath := createTestFile(t, content)

	input := ReadFileInput{Path: filePath, StartLine: 5, EndLine: 10}
	inputJSON, _ := json.Marshal(input)

	result, err := RunReadFileTool(context.Background(), inputJSON)

	assert.NoError(t, err)
	assert.Contains(t, result, "5: Line 5")
	assert.Contains(t, result, "10: Line 10")
	assert.NotContains(t, result, "4: Line 4")
	assert.NotContains(t, result, "11: Line 11")
	assert.Contains(t, result, "10 lines remaining")
}

func TestReadFile_DefaultCap(t *testing.T) {
	var lines []string
	for i := 1; i <= 600; i++ {
		lines = append(lines, fmt.Sprintf("Line %d", i))
	}
	content := strings.Join(lines, "\n")
	filePath := createTestFile(t, content)

	input := ReadFileInput{Path: filePath}
	inputJSON, _ := json.Marshal(input)

	result, err := RunReadFileTool(context.Background(), inputJSON)

	assert.NoError(t, err)
	assert.Contains(t, result, "1: Line 1")
	assert.Contains(t, result, "500: Line 500")
	assert.NotContains(t, result, "501: Line 501")
	assert.Contains(t, result, "100 lines remaining")
	assert.Contains(t, result, "600 total lines")
}

func TestReadFile_StartLineBeyondEnd(t *testing.T) {
	content := "Line 1\nLine 2"
	filePath := createTestFile(t, content)

	input := ReadFileInput{Path: filePath, StartLine: 100}
	inputJSON, _ := json.Marshal(input)

	result, err := RunReadFileTool(context.Background(), inputJSON)

	assert.NoError(t, err)
	assert.Contains(t, result, "beyond end of file")
}

func TestReadFile_EndLineBeyondFileLength(t *testing.T) {
	content := "Line 1\nLine 2\nLine 3"
	filePath := createTestFile(t, content)

	input := ReadFileInput{Path: filePath, StartLine: 2, EndLine: 100}
	inputJSON, _ := json.Marshal(input)

	result, err := RunReadFileTool(context.Background(), inputJSON)

	assert.NoError(t, err)
	assert.Contains(t, result, "2: Line 2")
	assert.Contains(t, result, "3: Line 3")
	assert.NotContains(t, result, "lines remaining")
}

func TestReadFile_InvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{"invalid": json}`)

	_, err := RunReadFileTool(context.Background(), invalidJSON)

	assert.Error(t, err)
}

func TestReadFile_Directory(t *testing.T) {
	dirPath := createTestDirectory(t)

	input := ReadFileInput{Path: dirPath}
	inputJSON, _ := json.Marshal(input)

	result, err := RunReadFileTool(context.Background(), inputJSON)

	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestReadFile_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	content := "restricted content"
	filePath := createTestFile(t, content)

	err := os.Chmod(filePath, 0000)
	assert.NoError(t, err)
	defer os.Chmod(filePath, 0644)

	input := ReadFileInput{Path: filePath}
	inputJSON, _ := json.Marshal(input)

	result, err := RunReadFileTool(context.Background(), inputJSON)

	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestReadFileDefinition_Structure(t *testing.T) {
	assert.Equal(t, "read_file", ReadFileDefinition.Name)
	assert.NotEmpty(t, ReadFileDefinition.Description)
	assert.NotNil(t, ReadFileDefinition.InputSchema)
}

func TestReadFileInput_JSONMarshaling(t *testing.T) {
	input := ReadFileInput{Path: "/absolute/path/to/file.txt"}

	data, err := json.Marshal(input)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"path":"/absolute/path/to/file.txt"`)
}

func TestReadFileInput_JSONUnmarshaling(t *testing.T) {
	jsonData := `{"path":"/absolute/path/to/file.txt","start_line":10,"end_line":20}`

	var input ReadFileInput
	err := json.Unmarshal([]byte(jsonData), &input)

	assert.NoError(t, err)
	assert.Equal(t, "/absolute/path/to/file.txt", input.Path)
	assert.Equal(t, 10, input.StartLine)
	assert.Equal(t, 20, input.EndLine)
}

func BenchmarkReadFile_SmallFile(b *testing.B) {
	content := "Small file content for benchmarking"
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "small_file.txt")
	os.WriteFile(filePath, []byte(content), 0644)

	input := ReadFileInput{Path: filePath}
	inputJSON, _ := json.Marshal(input)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunReadFileTool(context.Background(), inputJSON)
	}
}

func BenchmarkReadFile_LargeFile(b *testing.B) {
	var lines []string
	for i := 0; i < 10000; i++ {
		lines = append(lines, "This is a line of content for benchmarking large file reads.")
	}
	largeContent := strings.Join(lines, "\n")

	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "large_file.txt")
	os.WriteFile(filePath, []byte(largeContent), 0644)

	input := ReadFileInput{Path: filePath}
	inputJSON, _ := json.Marshal(input)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunReadFileTool(context.Background(), inputJSON)
	}
}
