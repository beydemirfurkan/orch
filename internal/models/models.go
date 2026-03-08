package models

import "time"

// State machine: Created → Analyzing → Planning → Coding → Validating → Testing → Reviewing → Completed/Failed
type RunStatus string

const (
	StatusCreated    RunStatus = "created"
	StatusAnalyzing  RunStatus = "analyzing"
	StatusPlanning   RunStatus = "planning"
	StatusCoding     RunStatus = "coding"
	StatusValidating RunStatus = "validating"
	StatusTesting    RunStatus = "testing"
	StatusReviewing  RunStatus = "reviewing"
	StatusCompleted  RunStatus = "completed"
	StatusFailed     RunStatus = "failed"
)

type ReviewDecision string

const (
	ReviewAccept ReviewDecision = "accept"
	ReviewRevise ReviewDecision = "revise"
)

type Task struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type Plan struct {
	TaskID         string     `json:"task_id"`
	Steps          []PlanStep `json:"steps"`
	FilesToModify  []string   `json:"files_to_modify"`
	FilesToInspect []string   `json:"files_to_inspect"`
	// Risks contains identified risks.
	Risks        []string `json:"risks"`
	TestStrategy string   `json:"test_strategy"`
}

type PlanStep struct {
	Order       int    `json:"order"`
	Description string `json:"description"`
	TargetFile  string `json:"target_file,omitempty"`
}

type Patch struct {
	TaskID string      `json:"task_id"`
	Files  []PatchFile `json:"files"`
	// RawDiff contains raw unified diff text.
	RawDiff string `json:"raw_diff"`
}

type PatchFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Diff   string `json:"diff"`
}

type RepoMap struct {
	RootPath string `json:"root_path"`
	// Language is the detected primary language.
	Language       string `json:"language"`
	PackageManager string `json:"package_manager"`
	TestFramework  string `json:"test_framework"`
	// Files is the repository file inventory.
	Files []FileInfo `json:"files"`
}

type FileInfo struct {
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Size     int64    `json:"size"`
	Imports  []string `json:"imports,omitempty"`
}

type RunState struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Task      Task           `json:"task"`
	Status    RunStatus      `json:"status"`
	Plan      *Plan          `json:"plan,omitempty"`
	Patch     *Patch         `json:"patch,omitempty"`
	Context   *ContextResult `json:"context,omitempty"`
	// Review contains review output when available.
	Review *ReviewResult `json:"review,omitempty"`
	// TestResults stores summarized test execution output.
	TestResults        string     `json:"test_results,omitempty"`
	Retries            RetryState `json:"retries"`
	UnresolvedFailures []string   `json:"unresolved_failures,omitempty"`
	BestPatchSummary   string     `json:"best_patch_summary,omitempty"`
	Logs               []LogEntry `json:"logs"`
	StartedAt          time.Time  `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	Error              string     `json:"error,omitempty"`
}

type RetryState struct {
	Validation int `json:"validation"`
	Testing    int `json:"testing"`
	Review     int `json:"review"`
}

type ReviewResult struct {
	Decision    ReviewDecision `json:"decision"`
	Comments    []string       `json:"comments"`
	Suggestions []string       `json:"suggestions,omitempty"`
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Step      string    `json:"step"`
	Message   string    `json:"message"`
}

type ContextResult struct {
	SelectedFiles   []string `json:"selected_files"`
	RelatedTests    []string `json:"related_tests"`
	RelevantConfigs []string `json:"relevant_configs"`
}

type ToolResult struct {
	ToolName  string            `json:"tool_name"`
	Success   bool              `json:"success"`
	Output    string            `json:"output"`
	Error     string            `json:"error,omitempty"`
	ErrorCode string            `json:"error_code,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}
