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

type TaskType string

const (
	TaskTypeUnknown  TaskType = "unknown"
	TaskTypeFeature  TaskType = "feature"
	TaskTypeBugfix   TaskType = "bugfix"
	TaskTypeTest     TaskType = "test"
	TaskTypeRefactor TaskType = "refactor"
	TaskTypeDocs     TaskType = "docs"
	TaskTypeChore    TaskType = "chore"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type ValidationStatus string

const (
	ValidationPass ValidationStatus = "pass"
	ValidationWarn ValidationStatus = "warn"
	ValidationFail ValidationStatus = "fail"
)

type ValidationSeverity string

const (
	SeverityLow      ValidationSeverity = "low"
	SeverityMedium   ValidationSeverity = "medium"
	SeverityHigh     ValidationSeverity = "high"
	SeverityCritical ValidationSeverity = "critical"
)

type Task struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type TaskBrief struct {
	TaskID            string    `json:"task_id"`
	UserRequest       string    `json:"user_request"`
	NormalizedGoal    string    `json:"normalized_goal"`
	TaskType          TaskType  `json:"task_type"`
	RiskLevel         RiskLevel `json:"risk_level"`
	Constraints       []string  `json:"constraints,omitempty"`
	Assumptions       []string  `json:"assumptions,omitempty"`
	SuccessDefinition []string  `json:"success_definition,omitempty"`
}

type AcceptanceCriterion struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type Plan struct {
	TaskID             string                `json:"task_id"`
	Summary            string                `json:"summary,omitempty"`
	TaskType           TaskType              `json:"task_type,omitempty"`
	RiskLevel          RiskLevel             `json:"risk_level,omitempty"`
	Steps              []PlanStep            `json:"steps"`
	FilesToModify      []string              `json:"files_to_modify"`
	FilesToInspect     []string              `json:"files_to_inspect"`
	Risks              []string              `json:"risks"`
	TestStrategy       string                `json:"test_strategy"`
	TestRequirements   []string              `json:"test_requirements,omitempty"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria,omitempty"`
	Invariants         []string              `json:"invariants,omitempty"`
	ForbiddenChanges   []string              `json:"forbidden_changes,omitempty"`
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

type PatchBudget struct {
	MaxFiles        int `json:"max_files"`
	MaxChangedLines int `json:"max_changed_lines"`
}

type ScopeExpansionPolicy struct {
	Allowed        bool `json:"allowed"`
	RequiresReason bool `json:"requires_reason"`
	MaxExtraFiles  int  `json:"max_extra_files"`
}

type ExecutionContract struct {
	TaskID               string               `json:"task_id"`
	PlanID               string               `json:"plan_id,omitempty"`
	AllowedFiles         []string             `json:"allowed_files,omitempty"`
	InspectFiles         []string             `json:"inspect_files,omitempty"`
	RequiredEdits        []string             `json:"required_edits,omitempty"`
	ProhibitedActions    []string             `json:"prohibited_actions,omitempty"`
	AcceptanceCriteria   []string             `json:"acceptance_criteria,omitempty"`
	Invariants           []string             `json:"invariants,omitempty"`
	PatchBudget          PatchBudget          `json:"patch_budget"`
	ScopeExpansionPolicy ScopeExpansionPolicy `json:"scope_expansion_policy"`
}

type ValidationResult struct {
	Name            string             `json:"name"`
	Stage           string             `json:"stage"`
	Status          ValidationStatus   `json:"status"`
	Severity        ValidationSeverity `json:"severity"`
	Summary         string             `json:"summary"`
	Details         []string           `json:"details,omitempty"`
	ActionableItems []string           `json:"actionable_items,omitempty"`
	Metadata        map[string]string  `json:"metadata,omitempty"`
}

type RetryDirective struct {
	Stage        string   `json:"stage"`
	Attempt      int      `json:"attempt"`
	Reasons      []string `json:"reasons,omitempty"`
	FailedGates  []string `json:"failed_gates,omitempty"`
	FailedTests  []string `json:"failed_tests,omitempty"`
	Instructions []string `json:"instructions,omitempty"`
	Avoid        []string `json:"avoid,omitempty"`
}

type ReviewScorecard struct {
	RequirementCoverage int            `json:"requirement_coverage"`
	ScopeControl        int            `json:"scope_control"`
	RegressionRisk      int            `json:"regression_risk"`
	Readability         int            `json:"readability"`
	Maintainability     int            `json:"maintainability"`
	TestAdequacy        int            `json:"test_adequacy"`
	Decision            ReviewDecision `json:"decision"`
	Findings            []string       `json:"findings,omitempty"`
}

type ConfidenceReport struct {
	Score    float64  `json:"score"`
	Band     string   `json:"band"`
	Reasons  []string `json:"reasons,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type TestFailure struct {
	Code    string   `json:"code"`
	Summary string   `json:"summary"`
	Details []string `json:"details,omitempty"`
	Flaky   bool     `json:"flaky,omitempty"`
}

type RunState struct {
	ID                string             `json:"id"`
	ProjectID         string             `json:"project_id,omitempty"`
	SessionID         string             `json:"session_id,omitempty"`
	Task              Task               `json:"task"`
	TaskBrief         *TaskBrief         `json:"task_brief,omitempty"`
	Status            RunStatus          `json:"status"`
	Plan              *Plan              `json:"plan,omitempty"`
	ExecutionContract *ExecutionContract `json:"execution_contract,omitempty"`
	Patch             *Patch             `json:"patch,omitempty"`
	Context           *ContextResult     `json:"context,omitempty"`
	ValidationResults []ValidationResult `json:"validation_results,omitempty"`
	RetryDirective    *RetryDirective    `json:"retry_directive,omitempty"`
	ReviewScorecard   *ReviewScorecard   `json:"review_scorecard,omitempty"`
	Confidence        *ConfidenceReport  `json:"confidence,omitempty"`
	TestFailures      []TestFailure      `json:"test_failures,omitempty"`
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
