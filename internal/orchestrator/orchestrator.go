// Task → Repo Analysis → Context Selection → Planner → Coder → Patch Validation → Test → Reviewer → Result
package orchestrator

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/agents"
	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/logger"
	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/patch"
	"github.com/furkanbeydemir/orch/internal/providers"
	"github.com/furkanbeydemir/orch/internal/providers/openai"
	"github.com/furkanbeydemir/orch/internal/repo"
	"github.com/furkanbeydemir/orch/internal/tools"
)

type Orchestrator struct {
	cfg *config.Config
	// log, execution trace logger.
	log            *logger.Logger
	analyzer       *repo.Analyzer
	contextBuilder *repo.ContextBuilder
	planner        agents.Agent
	coder          agents.Agent
	reviewer       agents.Agent
	patchPipeline  *patch.Pipeline
	toolRegistry   *tools.Registry
	repoRoot       string
	// verbose controls detailed log output.
	verbose bool
}

func New(cfg *config.Config, repoRoot string, verbose bool) *Orchestrator {
	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	log := logger.New(runID, repoRoot, verbose)

	orch := &Orchestrator{
		cfg:            cfg,
		log:            log,
		analyzer:       repo.NewAnalyzer(repoRoot),
		contextBuilder: repo.NewContextBuilder(repoRoot),
		planner:        agents.NewPlanner(cfg.Models.Planner),
		coder:          agents.NewCoder(cfg.Models.Coder),
		reviewer:       agents.NewReviewer(cfg.Models.Reviewer),
		patchPipeline:  patch.NewPipeline(cfg.Patch.MaxFiles, cfg.Patch.MaxLines),
		toolRegistry:   tools.DefaultRegistryWithPolicy(repoRoot, buildPolicy(cfg, tools.ModeRun), nil),
		repoRoot:       repoRoot,
		verbose:        verbose,
	}

	orch.attachProviderRuntime()

	return orch
}

func (o *Orchestrator) attachProviderRuntime() {
	if o == nil || o.cfg == nil {
		return
	}
	if !o.cfg.Provider.Flags.OpenAIEnabled {
		return
	}
	if strings.TrimSpace(os.Getenv(o.cfg.Provider.OpenAI.APIKeyEnv)) == "" {
		return
	}

	registry := providers.NewRegistry()
	registry.Register(openai.New(o.cfg.Provider.OpenAI))
	router := providers.NewRouter(o.cfg, registry)
	runtime := &agents.LLMRuntime{Router: router}

	if planner, ok := o.planner.(*agents.Planner); ok {
		planner.SetRuntime(runtime)
	}
	if coder, ok := o.coder.(*agents.Coder); ok {
		coder.SetRuntime(runtime)
	}
	if reviewer, ok := o.reviewer.(*agents.Reviewer); ok {
		reviewer.SetRuntime(runtime)
	}
}

// Pipeline: Analyze → Plan → Code → Validate → Test → Review
func (o *Orchestrator) Run(task *models.Task) (*models.RunState, error) {
	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	o.log = logger.New(runID, o.repoRoot, o.verbose)

	state := &models.RunState{
		ID:        runID,
		Task:      *task,
		Status:    models.StatusCreated,
		Logs:      make([]models.LogEntry, 0),
		Retries:   models.RetryState{},
		StartedAt: time.Now(),
	}

	o.log.Log("orchestrator", "start", fmt.Sprintf("Task started: %s", task.Description))
	o.log.Log("policy", "mode", "policy decision mode=run read_only=false")
	o.toolRegistry = tools.DefaultRegistryWithPolicy(o.repoRoot, buildPolicy(o.cfg, tools.ModeRun), func(message string) {
		o.log.Log("policy", "decision", message)
	})

	// 1. Repository analysis
	if err := o.stepAnalyze(state); err != nil {
		return o.fail(state, err)
	}

	// 2. Planning
	if err := o.stepPlan(state); err != nil {
		return o.fail(state, err)
	}

	if err := o.stepCode(state); err != nil {
		return o.fail(state, err)
	}

	if err := o.stepValidateWithRetries(state); err != nil {
		return o.fail(state, err)
	}

	if err := o.stepTestWithRetries(state); err != nil {
		return o.fail(state, err)
	}

	if err := o.stepReviewWithRetries(state); err != nil {
		return o.fail(state, err)
	}

	if err := Transition(state, models.StatusCompleted); err != nil {
		return o.fail(state, err)
	}

	now := time.Now()
	state.CompletedAt = &now
	o.log.Log("orchestrator", "complete", "Pipeline completed successfully")

	state.Logs = o.log.Entries()
	_ = o.log.Save()

	return state, nil
}

func (o *Orchestrator) Plan(task *models.Task) (*models.Plan, error) {
	o.log.Log("policy", "mode", "policy decision mode=plan read_only=true")
	o.toolRegistry = tools.DefaultRegistryWithPolicy(o.repoRoot, buildPolicy(o.cfg, tools.ModePlan), func(message string) {
		o.log.Log("policy", "decision", message)
	})
	o.log.Log("orchestrator", "plan-only", fmt.Sprintf("Generating plan: %s", task.Description))

	// Repository analysis
	repoMap, err := o.analyzer.Analyze()
	if err != nil {
		return nil, fmt.Errorf("repository analysis failed: %w", err)
	}

	input := &agents.Input{
		Task:    task,
		RepoMap: repoMap,
	}

	output, err := o.planner.Execute(input)
	if err != nil {
		return nil, fmt.Errorf("planning failed: %w", err)
	}

	return output.Plan, nil
}

func (o *Orchestrator) stepAnalyze(state *models.RunState) error {
	if err := Transition(state, models.StatusAnalyzing); err != nil {
		return err
	}
	o.log.Log("analyzer", "analyze", "Scanning repository...")

	_, err := o.analyzer.Analyze()
	if err != nil {
		return fmt.Errorf("repository analysis failed: %w", err)
	}

	o.log.Log("analyzer", "analyze", "Repository analysis completed")
	return nil
}

func (o *Orchestrator) stepPlan(state *models.RunState) error {
	if err := Transition(state, models.StatusPlanning); err != nil {
		return err
	}
	o.log.Log("planner", "plan", "Generating plan...")

	input := &agents.Input{
		Task: &state.Task,
	}

	output, err := o.planner.Execute(input)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	state.Plan = output.Plan

	repoMap, err := o.analyzer.Analyze()
	if err != nil {
		return fmt.Errorf("context repository analysis failed: %w", err)
	}
	state.Context = o.contextBuilder.Build(&state.Task, repoMap, state.Plan)
	o.log.Log("context", "build", fmt.Sprintf("Context built: selected=%d tests=%d configs=%d", len(state.Context.SelectedFiles), len(state.Context.RelatedTests), len(state.Context.RelevantConfigs)))

	o.log.Log("planner", "plan", "Plan generated")
	return nil
}

func (o *Orchestrator) stepCode(state *models.RunState) error {
	if err := Transition(state, models.StatusCoding); err != nil {
		return err
	}
	o.log.Log("coder", "code", "Generating code changes...")

	input := &agents.Input{
		Task:    &state.Task,
		Plan:    state.Plan,
		Context: state.Context,
	}

	output, err := o.coder.Execute(input)
	if err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}

	state.Patch = output.Patch
	o.log.Log("coder", "code", "Code changes generated")
	return nil
}

func (o *Orchestrator) stepValidate(state *models.RunState) error {
	if err := Transition(state, models.StatusValidating); err != nil {
		return err
	}
	o.log.Log("validator", "validate", "Validating patch...")

	if state.Patch == nil {
		return fmt.Errorf("no patch found to validate")
	}

	if err := o.patchPipeline.Validate(state.Patch); err != nil {
		if o.cfg.Safety.FeatureFlags.PatchConflictReporting {
			return fmt.Errorf("patch validation failed (impacted files: %s): %w", strings.Join(patchFilePaths(state.Patch), ", "), err)
		}
		return fmt.Errorf("patch validation failed: %w", err)
	}

	o.log.Log("validator", "validate", "Patch validated")
	return nil
}

func (o *Orchestrator) stepTest(state *models.RunState) error {
	if err := Transition(state, models.StatusTesting); err != nil {
		return err
	}
	o.log.Log("test", "test", "Running tests...")

	result, err := o.toolRegistry.Execute("run_tests", map[string]string{"command": o.cfg.Commands.Test})
	if err != nil {
		return fmt.Errorf("failed to start test command: %w", err)
	}

	if result == nil {
		return fmt.Errorf("test result was not returned")
	}

	state.TestResults = strings.TrimSpace(result.Output)
	if !result.Success {
		o.log.Log("test", "test", "Tests failed")
		if state.TestResults == "" {
			state.TestResults = strings.TrimSpace(result.Error)
		}
		return fmt.Errorf("tests failed: %s", strings.TrimSpace(result.Error))
	}

	o.log.Log("test", "test", "Tests completed")
	return nil
}

func (o *Orchestrator) stepReview(state *models.RunState) error {
	if err := Transition(state, models.StatusReviewing); err != nil {
		return err
	}
	o.log.Log("reviewer", "review", "Reviewing changes...")

	input := &agents.Input{
		Task:        &state.Task,
		Plan:        state.Plan,
		Patch:       state.Patch,
		TestResults: state.TestResults,
	}

	output, err := o.reviewer.Execute(input)
	if err != nil {
		return fmt.Errorf("review failed: %w", err)
	}

	state.Review = output.Review
	o.log.Log("reviewer", "review", fmt.Sprintf("Review completed: %s", output.Review.Decision))
	return nil
}

func (o *Orchestrator) stepValidateWithRetries(state *models.RunState) error {
	maxRetries := 0
	if o.cfg.Safety.FeatureFlags.RetryLimits {
		maxRetries = o.cfg.Safety.Retry.ValidationMax
	}

	for {
		err := o.stepValidate(state)
		if err == nil {
			return nil
		}

		if state.Retries.Validation >= maxRetries {
			o.addUnresolvedFailure(state, "validation", err)
			return err
		}

		state.Retries.Validation++
		o.log.Log("orchestrator", "retry", fmt.Sprintf("Validation failed, retrying code generation (%d/%d)", state.Retries.Validation, maxRetries))
		if codeErr := o.stepCode(state); codeErr != nil {
			o.addUnresolvedFailure(state, "coding-after-validation", codeErr)
			return codeErr
		}
	}
}

func (o *Orchestrator) stepTestWithRetries(state *models.RunState) error {
	maxRetries := 0
	if o.cfg.Safety.FeatureFlags.RetryLimits {
		maxRetries = o.cfg.Safety.Retry.TestMax
	}

	for {
		err := o.stepTest(state)
		if err == nil {
			return nil
		}

		if state.Retries.Testing >= maxRetries {
			o.addUnresolvedFailure(state, "test", err)
			return err
		}

		state.Retries.Testing++
		o.log.Log("orchestrator", "retry", fmt.Sprintf("Tests failed, retrying code generation (%d/%d)", state.Retries.Testing, maxRetries))
		if codeErr := o.stepCode(state); codeErr != nil {
			o.addUnresolvedFailure(state, "coding-after-test", codeErr)
			return codeErr
		}
		if validateErr := o.stepValidate(state); validateErr != nil {
			o.addUnresolvedFailure(state, "validation-after-test", validateErr)
			return validateErr
		}
	}
}

func (o *Orchestrator) stepReviewWithRetries(state *models.RunState) error {
	maxRetries := 0
	if o.cfg.Safety.FeatureFlags.RetryLimits {
		maxRetries = o.cfg.Safety.Retry.ReviewMax
	}

	for {
		err := o.stepReview(state)
		if err != nil {
			o.addUnresolvedFailure(state, "review", err)
			return err
		}

		if state.Review == nil || state.Review.Decision != models.ReviewRevise {
			return nil
		}

		if state.Retries.Review >= maxRetries {
			err = fmt.Errorf("review requested revise beyond retry limit")
			o.addUnresolvedFailure(state, "review-revise", err)
			return err
		}

		state.Retries.Review++
		o.log.Log("orchestrator", "retry", fmt.Sprintf("Review requested revise, retrying code generation (%d/%d)", state.Retries.Review, maxRetries))

		if codeErr := o.stepCode(state); codeErr != nil {
			o.addUnresolvedFailure(state, "coding-after-review", codeErr)
			return codeErr
		}
		if validateErr := o.stepValidateWithRetries(state); validateErr != nil {
			return validateErr
		}
		if testErr := o.stepTestWithRetries(state); testErr != nil {
			return testErr
		}
	}
}

func (o *Orchestrator) addUnresolvedFailure(state *models.RunState, stage string, err error) {
	failure := fmt.Sprintf("%s: %v", stage, err)
	state.UnresolvedFailures = append(state.UnresolvedFailures, failure)
	state.BestPatchSummary = patch.Summarize(state.Patch)
	o.log.Log("orchestrator", "unresolved", failure)
}

func buildPolicy(cfg *config.Config, mode string) tools.Policy {
	policy := tools.Policy{Mode: mode}
	if cfg != nil {
		policy.RequireDestructiveApproval = cfg.Safety.RequireDestructiveApproval
	}
	if mode == tools.ModePlan {
		policy.RequireDestructiveApproval = false
	}
	return policy
}

func patchFilePaths(p *models.Patch) []string {
	if p == nil {
		return []string{"unknown"}
	}
	paths := make([]string, 0, len(p.Files))
	for _, file := range p.Files {
		if strings.TrimSpace(file.Path) == "" {
			continue
		}
		paths = append(paths, file.Path)
	}
	if len(paths) == 0 {
		return []string{"unknown"}
	}
	return paths
}

func (o *Orchestrator) fail(state *models.RunState, err error) (*models.RunState, error) {
	o.log.Log("orchestrator", "fail", fmt.Sprintf("Error: %v", err))
	state.Error = err.Error()
	_ = Transition(state, models.StatusFailed)

	now := time.Now()
	state.CompletedAt = &now
	state.Logs = o.log.Entries()
	_ = o.log.Save()

	return state, err
}
