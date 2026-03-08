package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type SearchCodeTool struct {
	repoRoot string
}

func NewSearchCodeTool(repoRoot string) *SearchCodeTool {
	return &SearchCodeTool{repoRoot: repoRoot}
}

func (t *SearchCodeTool) Name() string { return "search_code" }

func (t *SearchCodeTool) Description() string {
	return "Searches text/patterns in repository"
}

// Execute searches text in repository files.
// Params: "query" text to search, "path" optional search root.
func (t *SearchCodeTool) Execute(params map[string]string) (*models.ToolResult, error) {
	query, ok := params["query"]
	if !ok {
		return Failure("search_code", ErrCodeInvalidParams, "search_code: 'query' parameter is required", ""), nil
	}

	searchPath := params["path"]
	if searchPath == "" {
		searchPath = "."
	}

	fullPath := filepath.Join(t.repoRoot, searchPath)
	var results []string

	err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		relPath, _ := filepath.Rel(t.repoRoot, path)
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".pdf" || ext == ".zip" {
			return nil
		}
		scanner := bufio.NewScanner(file)
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				results = append(results, fmt.Sprintf("%s:%d: %s", relPath, lineNum, strings.TrimSpace(line)))
			}
		}

		return nil
	})

	if err != nil {
		return Failure("search_code", ErrCodeExecution, err.Error(), ""), nil
	}

	return Success("search_code", strings.Join(results, "\n")), nil
}
