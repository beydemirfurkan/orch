// Package patch - Unified diff parser implementasyonu.
//
// Desteklenen format:
//
//	diff --git a/file b/file
//	--- a/file
//	+++ b/file
//	@@ -start,count +start,count @@
//	...
package patch

import (
	"fmt"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(rawDiff string) (*models.Patch, error) {
	patch := &models.Patch{
		RawDiff: rawDiff,
		Files:   make([]models.PatchFile, 0),
	}

	if strings.TrimSpace(rawDiff) == "" {
		return patch, nil
	}

	lines := strings.Split(rawDiff, "\n")
	hasDiffHeader := false
	var currentFile *models.PatchFile
	var currentDiff strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			hasDiffHeader = true
			if currentFile != nil {
				currentFile.Diff = currentDiff.String()
				patch.Files = append(patch.Files, *currentFile)
			}

			// Parse new file metadata
			path := parseDiffHeader(line)
			currentFile = &models.PatchFile{
				Path:   path,
				Status: "modified",
			}
			currentDiff.Reset()
		}

		if currentFile != nil {
			currentDiff.WriteString(line)
			currentDiff.WriteString("\n")

			// Determine file status
			if strings.HasPrefix(line, "new file") {
				currentFile.Status = "added"
			} else if strings.HasPrefix(line, "deleted file") {
				currentFile.Status = "deleted"
			}
		}
	}

	if currentFile != nil {
		currentFile.Diff = currentDiff.String()
		patch.Files = append(patch.Files, *currentFile)
	}

	if !hasDiffHeader {
		return nil, fmt.Errorf("invalid unified diff: missing 'diff --git' header")
	}

	return patch, nil
}

func parseDiffHeader(line string) string {
	// Format: diff --git a/path b/path
	parts := strings.Split(line, " ")
	if len(parts) >= 4 {
		return strings.TrimPrefix(parts[3], "b/")
	}
	return ""
}
