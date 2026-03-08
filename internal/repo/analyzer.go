// - Primary programming language
// - File inventory
package repo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/furkanbeydemir/orch/internal/models"
)

type Analyzer struct {
	rootPath string
}

func NewAnalyzer(rootPath string) *Analyzer {
	return &Analyzer{
		rootPath: rootPath,
	}
}

func (a *Analyzer) Analyze() (*models.RepoMap, error) {
	repoMap := &models.RepoMap{
		RootPath: a.rootPath,
		Files:    make([]models.FileInfo, 0),
	}

	err := filepath.Walk(a.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
		}

		// Gizli dizinleri ve .orch dizinini atla
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Relative path hesapla
		relPath, err := filepath.Rel(a.rootPath, path)
		if err != nil {
			return nil
		}

		fileInfo := models.FileInfo{
			Path:     relPath,
			Language: detectLanguage(path),
			Size:     info.Size(),
		}

		repoMap.Files = append(repoMap.Files, fileInfo)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("repository scan failed: %w", err)
	}

	repoMap.Language = a.detectMainLanguage(repoMap.Files)
	repoMap.PackageManager = a.detectPackageManager()
	repoMap.TestFramework = a.detectTestFramework()

	// Repo map'i dosyaya yaz
	if err := a.saveRepoMap(repoMap); err != nil {
		return nil, err
	}

	return repoMap, nil
}

func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	languages := map[string]string{
		".go":    "go",
		".js":    "javascript",
		".ts":    "typescript",
		".py":    "python",
		".rs":    "rust",
		".java":  "java",
		".rb":    "ruby",
		".cpp":   "cpp",
		".c":     "c",
		".cs":    "csharp",
		".php":   "php",
		".swift": "swift",
		".kt":    "kotlin",
		".md":    "markdown",
		".json":  "json",
		".yaml":  "yaml",
		".yml":   "yaml",
		".toml":  "toml",
		".xml":   "xml",
		".html":  "html",
		".css":   "css",
		".sql":   "sql",
		".sh":    "shell",
	}

	if lang, ok := languages[ext]; ok {
		return lang
	}
	return "unknown"
}

// detectMainLanguage infers the primary language from the file inventory.
func (a *Analyzer) detectMainLanguage(files []models.FileInfo) string {
	counts := make(map[string]int)
	for _, f := range files {
		if f.Language != "unknown" && f.Language != "markdown" && f.Language != "json" && f.Language != "yaml" {
			counts[f.Language]++
		}
	}

	maxLang := "unknown"
	maxCount := 0
	for lang, count := range counts {
		if count > maxCount {
			maxLang = lang
			maxCount = count
		}
	}

	return maxLang
}

func (a *Analyzer) detectPackageManager() string {
	indicators := map[string]string{
		"go.mod":           "go modules",
		"package.json":     "npm",
		"requirements.txt": "pip",
		"Cargo.toml":       "cargo",
		"pom.xml":          "maven",
		"build.gradle":     "gradle",
		"Gemfile":          "bundler",
		"composer.json":    "composer",
	}

	for file, manager := range indicators {
		if _, err := os.Stat(filepath.Join(a.rootPath, file)); err == nil {
			return manager
		}
	}

	return "unknown"
}

func (a *Analyzer) detectTestFramework() string {
	indicators := map[string]string{
		"go.mod":         "go test",
		"jest.config.js": "jest",
		"jest.config.ts": "jest",
		"pytest.ini":     "pytest",
		"setup.cfg":      "pytest",
		".rspec":         "rspec",
	}

	for file, framework := range indicators {
		if _, err := os.Stat(filepath.Join(a.rootPath, file)); err == nil {
			return framework
		}
	}

	return "unknown"
}

func (a *Analyzer) saveRepoMap(repoMap *models.RepoMap) error {
	orchDir := filepath.Join(a.rootPath, ".orch")
	if err := os.MkdirAll(orchDir, 0o755); err != nil {
		return fmt.Errorf("failed to create .orch directory: %w", err)
	}

	data, err := json.MarshalIndent(repoMap, "", "  ")
	if err != nil {
		return fmt.Errorf("repo map serialize edilemedi: %w", err)
	}

	mapPath := filepath.Join(orchDir, "repo-map.json")
	if err := os.WriteFile(mapPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write repo map file: %w", err)
	}

	return nil
}
