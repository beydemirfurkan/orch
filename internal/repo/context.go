// Package repo - Context Builder implementasyonu.
//
// Girdi:
//   - Planner hints (opsiyonel)
package repo

import (
	"path/filepath"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type ContextBuilder struct {
	rootPath string
}

func NewContextBuilder(rootPath string) *ContextBuilder {
	return &ContextBuilder{
		rootPath: rootPath,
	}
}

// Build returns context for the given depth:
//   - Shallow: only files listed in plan.FilesToModify
//   - Standard (default): FilesToModify + FilesToInspect
//   - Deep: Standard + all test and config files from repo
func (cb *ContextBuilder) Build(task *models.Task, repoMap *models.RepoMap, plan *models.Plan) *models.ContextResult {
	return cb.BuildWithDepth(task, repoMap, plan, models.ContextDepthStandard)
}

func (cb *ContextBuilder) BuildWithDepth(task *models.Task, repoMap *models.RepoMap, plan *models.Plan, depth models.ContextDepth) *models.ContextResult {
	result := &models.ContextResult{
		SelectedFiles:   make([]string, 0),
		RelatedTests:    make([]string, 0),
		RelevantConfigs: make([]string, 0),
	}

	if plan != nil {
		result.SelectedFiles = append(result.SelectedFiles, plan.FilesToModify...)
		if depth != models.ContextDepthShallow {
			result.SelectedFiles = append(result.SelectedFiles, plan.FilesToInspect...)
		}
	}

	if depth == models.ContextDepthDeep {
		for _, file := range repoMap.Files {
			if isTestFile(file.Path) {
				result.RelatedTests = append(result.RelatedTests, file.Path)
			} else if isConfigFile(file.Path) {
				result.RelevantConfigs = append(result.RelevantConfigs, file.Path)
			}
		}
	} else {
		// Standard/Shallow: only include tests that directly relate to plan files
		planFileSet := make(map[string]struct{})
		if plan != nil {
			for _, f := range plan.FilesToModify {
				planFileSet[stripExt(filepath.Base(f))] = struct{}{}
			}
		}
		for _, file := range repoMap.Files {
			if isTestFile(file.Path) {
				base := stripExt(filepath.Base(file.Path))
				base = strings.TrimSuffix(base, "_test")
				base = strings.TrimPrefix(base, "test_")
				if _, ok := planFileSet[base]; ok {
					result.RelatedTests = append(result.RelatedTests, file.Path)
				}
			}
		}
	}

	return result
}

func stripExt(name string) string {
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[:idx]
	}
	return name
}

func isTestFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	patterns := []string{
		"_test.go",
		".test.js", ".test.ts",
		".spec.js", ".spec.ts",
		"test_", "_test.py",
	}

	for _, pattern := range patterns {
		if strings.Contains(base, pattern) {
			return true
		}
	}

	dir := strings.ToLower(filepath.Dir(path))
	return strings.Contains(dir, "test") || strings.Contains(dir, "__tests__")
}

func isConfigFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	configs := []string{
		"config", "conf", ".env", ".rc",
		"tsconfig", "jest.config", "webpack",
		"package.json", "go.mod", "cargo.toml",
		"dockerfile", "docker-compose", "makefile",
	}

	for _, cfg := range configs {
		if strings.Contains(base, cfg) {
			return true
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml" || ext == ".toml" || ext == ".ini"
}
