// Package patch contains patch validation implementation.
//
// - Sensitive files are protected.
package patch

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

var blockedFiles = []string{
	".env",
	".env.local",
	".env.production",
	"id_rsa",
	"id_ed25519",
	".pem",
	".key",
}

var binaryExtensions = []string{
	".exe", ".dll", ".so", ".dylib",
	".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg",
	".zip", ".tar", ".gz", ".rar",
	".pdf", ".doc", ".docx",
	".wasm",
}

type Validator struct {
	maxFiles int
	maxLines int
}

func NewValidator(maxFiles, maxLines int) *Validator {
	return &Validator{
		maxFiles: maxFiles,
		maxLines: maxLines,
	}
}

func (v *Validator) Validate(p *models.Patch) error {
	if p == nil {
		return fmt.Errorf("patch cannot be nil")
	}

	if len(p.Files) > v.maxFiles {
		return fmt.Errorf("patch contains too many files: %d (limit: %d)", len(p.Files), v.maxFiles)
	}

	lineCount := countLines(p.RawDiff)
	if lineCount > v.maxLines {
		return fmt.Errorf("patch contains too many lines: %d (limit: %d)", lineCount, v.maxLines)
	}

	for _, file := range p.Files {
		if isBinaryFile(file.Path) {
			return fmt.Errorf("binary file cannot be modified: %s", file.Path)
		}

		if isBlockedFile(file.Path) {
			return fmt.Errorf("sensitive file cannot be modified: %s", file.Path)
		}
	}

	return nil
}

func countLines(diff string) int {
	count := 0
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			if !strings.HasPrefix(line, "+++") && !strings.HasPrefix(line, "---") {
				count++
			}
		}
	}
	return count
}

func isBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, binExt := range binaryExtensions {
		if ext == binExt {
			return true
		}
	}
	return false
}

func isBlockedFile(path string) bool {
	base := filepath.Base(path)
	for _, blocked := range blockedFiles {
		if strings.EqualFold(base, blocked) {
			return true
		}
		if strings.HasPrefix(blocked, ".") && strings.HasSuffix(strings.ToLower(path), blocked) {
			return true
		}
	}
	return false
}
