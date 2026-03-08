package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type ReadFileTool struct {
	repoRoot string
}

func NewReadFileTool(repoRoot string) *ReadFileTool {
	return &ReadFileTool{repoRoot: repoRoot}
}

func (t *ReadFileTool) Name() string { return "read_file" }

func (t *ReadFileTool) Description() string { return "Reads contents of the specified file" }

func (t *ReadFileTool) Execute(params map[string]string) (*models.ToolResult, error) {
	path, ok := params["path"]
	if !ok {
		return Failure("read_file", ErrCodeInvalidParams, "read_file: 'path' parameter is required", ""), nil
	}

	fullPath := filepath.Join(t.repoRoot, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return Failure("read_file", ErrCodeExecution, err.Error(), ""), nil
	}

	return Success("read_file", string(data)), nil
}

type WriteFileTool struct {
	repoRoot string
}

func NewWriteFileTool(repoRoot string) *WriteFileTool {
	return &WriteFileTool{repoRoot: repoRoot}
}

func (t *WriteFileTool) Name() string { return "write_file" }

func (t *WriteFileTool) Description() string { return "Writes content to the specified file" }

func (t *WriteFileTool) Execute(params map[string]string) (*models.ToolResult, error) {
	path, ok := params["path"]
	if !ok {
		return Failure("write_file", ErrCodeInvalidParams, "write_file: 'path' parameter is required", ""), nil
	}

	content, ok := params["content"]
	if !ok {
		return Failure("write_file", ErrCodeInvalidParams, "write_file: 'content' parameter is required", ""), nil
	}

	fullPath := filepath.Join(t.repoRoot, path)

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Failure("write_file", ErrCodeExecution, err.Error(), ""), nil
	}

	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return Failure("write_file", ErrCodeExecution, err.Error(), ""), nil
	}

	return Success("write_file", fmt.Sprintf("File written: %s", path)), nil
}

type ListFilesTool struct {
	repoRoot string
}

func NewListFilesTool(repoRoot string) *ListFilesTool {
	return &ListFilesTool{repoRoot: repoRoot}
}

func (t *ListFilesTool) Name() string { return "list_files" }

func (t *ListFilesTool) Description() string { return "Lists files in the specified directory" }

// Params: "path" optional directory path (default: ".").
func (t *ListFilesTool) Execute(params map[string]string) (*models.ToolResult, error) {
	path := params["path"]
	if path == "" {
		path = "."
	}

	fullPath := filepath.Join(t.repoRoot, path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return Failure("list_files", ErrCodeExecution, err.Error(), ""), nil
	}

	var files []string
	for _, entry := range entries {
		prefix := "F"
		if entry.IsDir() {
			prefix = "D"
		}
		files = append(files, fmt.Sprintf("[%s] %s", prefix, entry.Name()))
	}

	return Success("list_files", strings.Join(files, "\n")), nil
}
