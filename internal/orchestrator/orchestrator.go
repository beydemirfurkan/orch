// Task → Repo Analysis → Context Selection → Planner → Coder → Patch Validation → Test → Reviewer → Result
package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/agents"
	"github.com/furkanbeydemir/orch/internal/auth"
	"github.com/furkanbeydemir/orch/internal/confidence"
	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/council"
	"github.com/furkanbeydemir/orch/internal/execution"
	"github.com/furkanbeydemir/orch/internal/logger"
	"github.com/furkanbeydemir/orch/internal/models"
	"github.com/furkanbeydemir/orch/internal/patch"
	"github.com/furkanbeydemir/orch/internal/planning"
	"github.com/furkanbeydemir/orch/internal/providers"
	anthropicprovider "github.com/furkanbeydemir/orch/internal/providers/anthropic"
	ollamaprovider "github.com/furkanbeydemir/orch/internal/providers/ollama"
	"github.com/furkanbeydemir/orch/internal/providers/openai"
	"github.com/furkanbeydemir/orch/internal/repo"
	reviewengine "github.com/furkanbeydemir/orch/internal/review"
	"github.com/furkanbeydemir/orch/internal/skills"
	testingengine "github.com/furkanbeydemir/orch/internal/testing"
	"github.com/furkanbeydemir/orch/internal/tools"
)

type Orchestrator struct {
	cfg *config.Config
	// log, execution trace logger.
	log              *logger.Logger
	analyzer         *repo.Analyzer
	contextBuilder   *repo.ContextBuilder
	planner          agents.Agent
	coder            agents.Agent
	reviewer         agents.Agent
	explorer         *agents.Explorer
	oracle           *agents.Oracle
	fixer            *agents.Fixer
	patchPipeline    *patch.Pipeline
	contractBuilder  *execution.ContractBuilder
	scopeGuard       *execution.ScopeGuard
	planGuard        *execution.PlanComplianceGuard
	retryBuilder     *execution.RetryDirectiveBuilder
	testClassifier   *testingengine.Classifier
	reviewEngine     *reviewengine.Engine
	confidenceScorer *confidence.Scorer
	confidencePolicy *confidence.Policy
	toolRegistry     *tools.Registry
	skillRegistry    *skills.Registry
	// providerRegistry is kept so council members can be wired at runtime.
	providerRegistry *providers.Registry
	repoRoot         string
	providerReady    bool
	// verbose controls detailed log output.
	verbose bool
}

func New(cfg *config.Config, repoRoot string, verbose bool) *Orchestrator {
	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	log := logger.New(runID, repoRoot, verbose)

	orch := &Orchestrator{
		cfg:              cfg,
		log:              log,
		analyzer:         repo.NewAnalyzer(repoRoot),
		contextBuilder:   repo.NewContextBuilder(repoRoot),
		planner:          agents.NewPlanner(cfg.Models.Planner),
		coder:            agents.NewCoder(cfg.Models.Coder),
		reviewer:         agents.NewReviewer(cfg.Models.Reviewer),
		explorer:         agents.NewExplorer(),
		oracle:           agents.NewOracle(),
		fixer:            agents.NewFixer(),
		patchPipeline:    patch.NewPipeline(cfg.Patch.MaxFiles, cfg.Patch.MaxLines),
		contractBuilder:  execution.NewContractBuilder(cfg),
		scopeGuard:       execution.NewScopeGuard(),
		planGuard:        execution.NewPlanComplianceGuard(),
		retryBuilder:     execution.NewRetryDirectiveBuilder(),
		testClassifier:   testingengine.NewClassifier(),
		reviewEngine:     reviewengine.NewEngine(),
		confidenceScorer: confidence.New(),
		confidencePolicy: confidence.NewPolicy(cfg),
		toolRegistry:     buildToolRegistry(cfg, repoRoot, tools.ModeRun, nil),
		skillRegistry:    buildSkillRegistry(cfg),
		repoRoot:         repoRoot,
		verbose:          verbose,
	}

	orch.attachProviderRuntime()

	return orch
}

func (o *Orchestrator) attachProviderRuntime() {
	if o == nil || o.cfg == nil {
		return
	}

	registry := providers.NewRegistry()
	registeredProviders := 0

	if o.cfg.Provider.Flags.OpenAIEnabled {
		mode := strings.ToLower(strings.TrimSpace(o.cfg.Provider.OpenAI.AuthMode))
		hasEnvAPIKey := strings.TrimSpace(os.Getenv(o.cfg.Provider.OpenAI.APIKeyEnv)) != ""
		hasOpenAICreds := true
		if mode == "api_key" && !hasEnvAPIKey {
			cred, err := auth.Get(o.repoRoot, "openai")
			if err != nil || cred == nil || strings.ToLower(strings.TrimSpace(cred.Type)) != "api" || strings.TrimSpace(cred.Key) == "" {
				hasOpenAICreds = false
			}
		}
		if hasOpenAICreds {
			client := openai.New(o.cfg.Provider.OpenAI)
			var accountSession *auth.AccountSession
			if strings.ToLower(strings.TrimSpace(o.cfg.Provider.OpenAI.AuthMode)) == "account" && strings.TrimSpace(os.Getenv(o.cfg.Provider.OpenAI.AccountTokenEnv)) == "" {
				accountSession = auth.NewAccountSession(o.repoRoot, "openai")
				client.SetAccountFailoverHandler(func(ctx context.Context, err error) (string, bool, error) {
					return accountSession.Failover(ctx, openai.AccountFailoverCooldown(err), err.Error())
				})
				client.SetAccountSuccessHandler(func(ctx context.Context) {
					accountSession.MarkSuccess(ctx)
				})
			}
			client.SetTokenResolver(func(ctx context.Context) (string, error) {
				if strings.ToLower(strings.TrimSpace(o.cfg.Provider.OpenAI.AuthMode)) == "api_key" {
					cred, err := auth.Get(o.repoRoot, "openai")
					if err != nil || cred == nil {
						return "", err
					}
					if strings.ToLower(strings.TrimSpace(cred.Type)) == "api" {
						return strings.TrimSpace(cred.Key), nil
					}
					return "", nil
				}
				if accountSession == nil {
					return "", nil
				}
				return accountSession.ResolveToken(ctx)
			})
			registry.Register(client)
			registeredProviders++
		}
	}

	// Register Anthropic provider if API key is available.
	anthropicKey := strings.TrimSpace(os.Getenv(o.cfg.Provider.Anthropic.APIKeyEnv))
	if anthropicKey != "" {
		anthropicCfg := anthropicprovider.Config{
			APIKeyEnv:      o.cfg.Provider.Anthropic.APIKeyEnv,
			BaseURL:        o.cfg.Provider.Anthropic.BaseURL,
			TimeoutSeconds: o.cfg.Provider.Anthropic.TimeoutSeconds,
			MaxRetries:     o.cfg.Provider.Anthropic.MaxRetries,
		}
		registry.Register(anthropicprovider.New(anthropicCfg))
		registeredProviders++
	}

	// Register Ollama provider if explicitly enabled in config.
	if o.cfg.Provider.Ollama.Enabled {
		ollamaCfg := ollamaprovider.Config{
			BaseURL:        o.cfg.Provider.Ollama.BaseURL,
			TimeoutSeconds: o.cfg.Provider.Ollama.TimeoutSeconds,
		}
		registry.Register(ollamaprovider.New(ollamaCfg))
		registeredProviders++
	}

	if registeredProviders == 0 {
		return
	}

	router := providers.NewRouter(o.cfg, registry)
	runtime := &agents.LLMRuntime{Router: router}
	o.providerRegistry = registry
	o.providerReady = true

	if planner, ok := o.planner.(*agents.Planner); ok {
		planner.SetRuntime(runtime)
	}
	if coder, ok := o.coder.(*agents.Coder); ok {
		coder.SetRuntime(runtime)
	}
	if reviewer, ok := o.reviewer.(*agents.Reviewer); ok {
		reviewer.SetRuntime(runtime)
	}
	if o.explorer != nil {
		o.explorer.SetRuntime(runtime)
	}
	if o.oracle != nil {
		o.oracle.SetRuntime(runtime)
	}
	if o.fixer != nil {
		o.fixer.SetRuntime(runtime)
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
	if o.providerReady {
		o.log.Log("provider", "status", fmt.Sprintf("active=openai planner=%s coder=%s reviewer=%s auth_mode=%s", o.cfg.Provider.OpenAI.Models.Planner, o.cfg.Provider.OpenAI.Models.Coder, o.cfg.Provider.OpenAI.Models.Reviewer, o.cfg.Provider.OpenAI.AuthMode))
	} else {
		o.log.Log("provider", "status", "inactive; falling back to local agent behavior")
	}
	o.toolRegistry = buildToolRegistry(o.cfg, o.repoRoot, tools.ModeRun, func(message string) {
		o.log.Log("policy", "decision", message)
	})

	// 1. Repository analysis
	if err := o.stepAnalyze(state); err != nil {
		return o.fail(state, err)
	}

	// 1b. Explorer (optional — gated by feature flag)
	if o.cfg.Safety.FeatureFlags.ExplorerEnabled {
		if err := o.stepExplore(state); err != nil {
			o.log.Log("explorer", "warn", fmt.Sprintf("Explorer failed (non-fatal): %v", err))
		}
	}

	// 2. Planning
	if err := o.stepPlan(state); err != nil {
		return o.fail(state, err)
	}

	// 2b. Oracle (optional — gated by feature flag)
	if o.cfg.Safety.FeatureFlags.OracleEnabled {
		if err := o.stepOracle(state); err != nil {
			o.log.Log("oracle", "warn", fmt.Sprintf("Oracle failed (non-fatal): %v", err))
		}
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
	_, plan, err := o.PlanDetailed(task)
	return plan, err
}

func (o *Orchestrator) PlanDetailed(task *models.Task) (*models.TaskBrief, *models.Plan, error) {
	o.log.Log("policy", "mode", "policy decision mode=plan read_only=true")
	o.toolRegistry = buildToolRegistry(o.cfg, o.repoRoot, tools.ModePlan, func(message string) {
		o.log.Log("policy", "decision", message)
	})
	o.log.Log("orchestrator", "plan-only", fmt.Sprintf("Generating plan: %s", task.Description))

	repoMap, err := o.analyzer.Analyze()
	if err != nil {
		return nil, nil, fmt.Errorf("repository analysis failed: %w", err)
	}

	taskBrief, compiledPlan := o.compilePlanArtifacts(task, repoMap)
	input := &agents.Input{
		Task:      task,
		TaskBrief: taskBrief,
		RepoMap:   repoMap,
		Plan:      compiledPlan,
	}

	output, err := o.planner.Execute(input)
	if err != nil {
		return nil, nil, fmt.Errorf("planning failed: %w", err)
	}
	if output == nil || output.Plan == nil {
		return taskBrief, compiledPlan, nil
	}

	return taskBrief, output.Plan, nil
}

func (o *Orchestrator) stepExplore(state *models.RunState) error {
	o.log.Log("explorer", "explore", "Running codebase reconnaissance...")
	repoMap, err := o.analyzer.Analyze()
	if err != nil {
		return fmt.Errorf("explorer: repo analysis failed: %w", err)
	}
	input := &agents.Input{
		Task:         &state.Task,
		TaskBrief:    state.TaskBrief,
		RepoMap:      repoMap,
		Plan:         state.Plan,
		MaxTokens:    o.cfg.Budget.PlannerMaxTokens,
		ContextDepth: models.ContextDepthShallow,
	}
	output, err := o.explorer.Execute(input)
	if err != nil {
		return err
	}
	if output != nil {
		o.recordUsage(state, "exploring", string(providers.RoleExplorer), output.Usage)
	}
	o.log.Log("explorer", "explore", "Exploration complete")
	return nil
}

func (o *Orchestrator) stepOracle(state *models.RunState) error {
	if state.Plan == nil {
		return nil
	}
	o.log.Log("oracle", "advise", "Oracle reviewing plan...")
	input := &agents.Input{
		Task:      &state.Task,
		TaskBrief: state.TaskBrief,
		Plan:      state.Plan,
		MaxTokens: o.cfg.Budget.ReviewerMaxTokens,
	}
	output, err := o.oracle.Execute(input)
	if err != nil {
		return err
	}
	if output != nil {
		o.recordUsage(state, "oracle", string(providers.RoleOracle), output.Usage)
	}
	o.log.Log("oracle", "advise", "Oracle review complete")
	return nil
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
	if o.providerReady {
		o.log.Log("provider", "planner", fmt.Sprintf("model=%s", o.cfg.Provider.OpenAI.Models.Planner))
	}

	repoMap, err := o.analyzer.Analyze()
	if err != nil {
		return fmt.Errorf("context repository analysis failed: %w", err)
	}

	taskBrief, compiledPlan := o.compilePlanArtifacts(&state.Task, repoMap)
	input := &agents.Input{
		Task:       &state.Task,
		TaskBrief:  taskBrief,
		RepoMap:    repoMap,
		Plan:       compiledPlan,
		MaxTokens:  o.cfg.Budget.PlannerMaxTokens,
		SkillHints: o.skillHintsForAgent("planner"),
	}

	output, err := o.planner.Execute(input)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}
	if output != nil {
		o.recordUsage(state, "planning", string(providers.RolePlanner), output.Usage)
	}

	state.TaskBrief = taskBrief
	state.Plan = compiledPlan
	if output != nil && output.Plan != nil {
		state.Plan = output.Plan
	}
	state.Context = o.contextBuilder.Build(&state.Task, repoMap, state.Plan)
	o.log.Log("context", "build", fmt.Sprintf("Context built: selected=%d tests=%d configs=%d", len(state.Context.SelectedFiles), len(state.Context.RelatedTests), len(state.Context.RelevantConfigs)))

	o.log.Log("planner", "plan", "Plan generated")
	return nil
}

func (o *Orchestrator) stepCode(state *models.RunState) error {
	return o.stepCodeWithDepth(state, contextDepthForRetry(state))
}

func (o *Orchestrator) stepCodeWithDepth(state *models.RunState, depth models.ContextDepth) error {
	if err := Transition(state, models.StatusCoding); err != nil {
		return err
	}
	o.log.Log("coder", "code", fmt.Sprintf("Generating code changes (context=%s)...", depth))
	if o.providerReady {
		o.log.Log("provider", "coder", fmt.Sprintf("model=%s", o.cfg.Provider.OpenAI.Models.Coder))
	}

	repoMap, err := o.analyzer.Analyze()
	if err == nil && state.Plan != nil {
		state.Context = o.contextBuilder.BuildWithDepth(&state.Task, repoMap, state.Plan, depth)
		o.log.Log("context", "depth", fmt.Sprintf("depth=%s selected=%d tests=%d", depth, len(state.Context.SelectedFiles), len(state.Context.RelatedTests)))
	}

	state.ExecutionContract = o.contractBuilder.Build(&state.Task, state.TaskBrief, state.Plan, state.Context)
	if state.ExecutionContract != nil {
		o.log.Log("execution", "contract", fmt.Sprintf("allowed_files=%d inspect_files=%d required_edits=%d", len(state.ExecutionContract.AllowedFiles), len(state.ExecutionContract.InspectFiles), len(state.ExecutionContract.RequiredEdits)))
	}

	input := &agents.Input{
		Task:              &state.Task,
		TaskBrief:         state.TaskBrief,
		Plan:              state.Plan,
		ExecutionContract: state.ExecutionContract,
		Context:           state.Context,
		RetryDirective:    state.RetryDirective,
		MaxTokens:         o.cfg.Budget.CoderMaxTokens,
		ContextDepth:      depth,
		SkillHints:        o.skillHintsForAgent("coder"),
	}

	output, err := o.coder.Execute(input)
	if err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}
	if output != nil {
		o.recordUsage(state, "coding", string(providers.RoleCoder), output.Usage)
	}

	state.Patch = output.Patch
	if state.Patch != nil && strings.TrimSpace(state.Patch.RawDiff) != "" {
		parsedPatch, parseErr := patch.NewParser().Parse(state.Patch.RawDiff)
		if parseErr != nil {
			state.ValidationResults = append(state.ValidationResults, models.ValidationResult{
				Name:     "patch_parse_valid",
				Stage:    "validation",
				Status:   models.ValidationFail,
				Severity: models.SeverityHigh,
				Summary:  parseErr.Error(),
			})
			return fmt.Errorf("code generation produced an invalid patch: %w", parseErr)
		}
		parsedPatch.TaskID = state.Task.ID
		state.Patch = parsedPatch
	}
	state.RetryDirective = nil
	o.log.Log("coder", "code", "Code changes generated")
	return nil
}

func (o *Orchestrator) stepValidate(state *models.RunState) error {
	if err := Transition(state, models.StatusValidating); err != nil {
		return err
	}
	o.log.Log("validator", "validate", "Validating patch...")
	state.ValidationResults = []models.ValidationResult{}

	if state.Patch == nil {
		result := models.ValidationResult{
			Name:     "patch_present",
			Stage:    "validation",
			Status:   models.ValidationFail,
			Severity: models.SeverityHigh,
			Summary:  "no patch found to validate",
		}
		state.ValidationResults = append(state.ValidationResults, result)
		return fmt.Errorf("%s", result.Summary)
	}

	state.ValidationResults = append(state.ValidationResults, models.ValidationResult{
		Name:     "patch_parse_valid",
		Stage:    "validation",
		Status:   models.ValidationPass,
		Severity: models.SeverityLow,
		Summary:  "patch parsed successfully",
	})

	if err := o.patchPipeline.Validate(state.Patch); err != nil {
		state.ValidationResults = append(state.ValidationResults, models.ValidationResult{
			Name:     "patch_hygiene",
			Stage:    "validation",
			Status:   models.ValidationFail,
			Severity: models.SeverityHigh,
			Summary:  err.Error(),
		})
		if o.cfg.Safety.FeatureFlags.PatchConflictReporting {
			return fmt.Errorf("patch validation failed (impacted files: %s): %w", strings.Join(patchFilePaths(state.Patch), ", "), err)
		}
		return fmt.Errorf("patch validation failed: %w", err)
	}
	state.ValidationResults = append(state.ValidationResults, models.ValidationResult{
		Name:     "patch_hygiene",
		Stage:    "validation",
		Status:   models.ValidationPass,
		Severity: models.SeverityLow,
		Summary:  "patch passed patch hygiene validation",
	})

	scopeResult := o.scopeGuard.Validate(state.ExecutionContract, state.Patch)
	state.ValidationResults = append(state.ValidationResults, scopeResult)
	if scopeResult.Status == models.ValidationFail {
		return fmt.Errorf("%s", scopeResult.Summary)
	}

	planResult := o.planGuard.Validate(state.Plan, state.ExecutionContract, state.Patch)
	state.ValidationResults = append(state.ValidationResults, planResult)
	if planResult.Status == models.ValidationFail {
		return fmt.Errorf("%s", planResult.Summary)
	}

	o.log.Log("validator", "validate", fmt.Sprintf("Patch validated with %d gate results", len(state.ValidationResults)))
	return nil
}

func (o *Orchestrator) stepTest(state *models.RunState) error {
	if err := Transition(state, models.StatusTesting); err != nil {
		return err
	}
	o.log.Log("test", "test", "Running tests...")
	state.TestFailures = nil
	state.ValidationResults = filterOutStage(state.ValidationResults, "test")

	result, err := o.toolRegistry.Execute("run_tests", map[string]string{"command": o.cfg.Commands.Test})
	if err != nil {
		state.ValidationResults = append(state.ValidationResults,
			models.ValidationResult{
				Name:     "required_tests_executed",
				Stage:    "test",
				Status:   models.ValidationFail,
				Severity: models.SeverityHigh,
				Summary:  "failed to start test command",
			},
		)
		state.TestFailures = o.testClassifier.Classify("", err.Error())
		state.TestResults = strings.TrimSpace(err.Error())
		return fmt.Errorf("failed to start test command: %w", err)
	}

	if result == nil {
		state.ValidationResults = append(state.ValidationResults,
			models.ValidationResult{
				Name:     "required_tests_executed",
				Stage:    "test",
				Status:   models.ValidationFail,
				Severity: models.SeverityHigh,
				Summary:  "test result was not returned",
			},
		)
		state.TestFailures = o.testClassifier.Classify("", "test result was not returned")
		return fmt.Errorf("test result was not returned")
	}

	state.ValidationResults = append(state.ValidationResults,
		models.ValidationResult{
			Name:     "required_tests_executed",
			Stage:    "test",
			Status:   models.ValidationPass,
			Severity: models.SeverityLow,
			Summary:  "required tests were executed",
		},
	)

	state.TestResults = strings.TrimSpace(result.Output)
	if !result.Success {
		o.log.Log("test", "test", "Tests failed")
		if state.TestResults == "" {
			state.TestResults = strings.TrimSpace(result.Error)
		}
		state.TestFailures = o.testClassifier.Classify(result.Output, result.Error)
		summaries := make([]string, 0, len(state.TestFailures))
		for _, failure := range state.TestFailures {
			summaries = append(summaries, failure.Code+": "+failure.Summary)
		}
		state.ValidationResults = append(state.ValidationResults,
			models.ValidationResult{
				Name:     "required_tests_passed",
				Stage:    "test",
				Status:   models.ValidationFail,
				Severity: models.SeverityHigh,
				Summary:  strings.Join(summaries, " | "),
			},
		)
		return fmt.Errorf("tests failed: %s", strings.TrimSpace(result.Error))
	}

	state.ValidationResults = append(state.ValidationResults,
		models.ValidationResult{
			Name:     "required_tests_passed",
			Stage:    "test",
			Status:   models.ValidationPass,
			Severity: models.SeverityLow,
			Summary:  "required tests passed",
		},
	)
	state.TestFailures = nil
	o.log.Log("test", "test", "Tests completed")
	return nil
}

// buildCouncil constructs a Council from the config, wiring up each member's Chat function
// directly from the provider registry. Returns nil if council is not configured/enabled.
func (o *Orchestrator) buildCouncil() *council.Council {
	cfg := o.cfg.Council
	if !o.cfg.Safety.FeatureFlags.CouncilEnabled || !cfg.Enabled || len(cfg.Members) == 0 {
		return nil
	}
	if o.providerRegistry == nil {
		return nil
	}

	members := make([]council.CouncilMember, 0, len(cfg.Members))
	for _, mc := range cfg.Members {
		if strings.TrimSpace(mc.Model) == "" {
			continue
		}
		idx := strings.Index(mc.Model, ":")
		if idx <= 0 {
			o.log.Log("council", "warn", fmt.Sprintf("invalid council member model %q (expected providerName:modelID)", mc.Model))
			continue
		}
		providerName := strings.ToLower(strings.TrimSpace(mc.Model[:idx]))
		modelID := strings.TrimSpace(mc.Model[idx+1:])
		provider, err := o.providerRegistry.Get(providerName)
		if err != nil {
			o.log.Log("council", "warn", fmt.Sprintf("council member provider %q not registered: %v", providerName, err))
			continue
		}
		w := mc.Weight
		if w <= 0 {
			w = 1
		}
		// Capture loop variables for the closure.
		p := provider
		m := modelID
		members = append(members, council.CouncilMember{
			ProviderName: providerName,
			ModelID:      modelID,
			Weight:       w,
			Chat: func(ctx context.Context, req providers.ChatRequest) (providers.ChatResponse, error) {
				req.Model = m
				return p.Chat(ctx, req)
			},
		})
	}

	if len(members) == 0 {
		return nil
	}

	mode := strings.ToLower(strings.TrimSpace(cfg.SynthesisMode))
	if mode == "" {
		mode = "majority"
	}

	var synthChat func(context.Context, providers.ChatRequest) (providers.ChatResponse, error)
	if mode == "meta" && strings.TrimSpace(cfg.SynthesizerModel) != "" {
		idx := strings.Index(cfg.SynthesizerModel, ":")
		if idx > 0 {
			synthProviderName := strings.ToLower(strings.TrimSpace(cfg.SynthesizerModel[:idx]))
			synthModelID := strings.TrimSpace(cfg.SynthesizerModel[idx+1:])
			if sp, err := o.providerRegistry.Get(synthProviderName); err == nil {
				sm := synthModelID
				synthChat = func(ctx context.Context, req providers.ChatRequest) (providers.ChatResponse, error) {
					req.Model = sm
					return sp.Chat(ctx, req)
				}
			}
		}
	}

	maxTok := cfg.MaxTokensPerMember
	if maxTok <= 0 {
		maxTok = o.cfg.Budget.ReviewerMaxTokens
	}

	return &council.Council{
		Members:              members,
		SynthesisMode:        mode,
		SynthesizerChat:      synthChat,
		MaxTokens:            maxTok,
		MinSuccessfulMembers: len(members)/2 + 1,
	}
}

// councilShouldTrigger returns true when council should be used for this state's task.
func (o *Orchestrator) councilShouldTrigger(state *models.RunState) bool {
	if !o.cfg.Safety.FeatureFlags.CouncilEnabled || !o.cfg.Council.Enabled {
		return false
	}
	if state.TaskBrief == nil {
		return false
	}
	trigger := strings.ToLower(strings.TrimSpace(o.cfg.Council.TriggerRiskLevel))
	if trigger == "" {
		trigger = "high"
	}
	taskRisk := strings.ToLower(string(state.TaskBrief.RiskLevel))
	switch trigger {
	case "low":
		return true // trigger on any risk
	case "medium":
		return taskRisk == "medium" || taskRisk == "high"
	default: // "high"
		return taskRisk == "high"
	}
}

func (o *Orchestrator) stepReview(state *models.RunState) error {
	if err := Transition(state, models.StatusReviewing); err != nil {
		return err
	}
	o.log.Log("reviewer", "review", "Reviewing changes...")
	state.ValidationResults = filterOutStage(state.ValidationResults, "review")
	if o.providerReady {
		o.log.Log("provider", "reviewer", fmt.Sprintf("model=%s", o.cfg.Provider.OpenAI.Models.Reviewer))
	}

	input := &agents.Input{
		Task:              &state.Task,
		TaskBrief:         state.TaskBrief,
		Plan:              state.Plan,
		ExecutionContract: state.ExecutionContract,
		Patch:             state.Patch,
		ValidationResults: state.ValidationResults,
		TestResults:       state.TestResults,
		MaxTokens:         o.cfg.Budget.ReviewerMaxTokens,
		SkillHints:        o.skillHintsForAgent("reviewer"),
	}

	// Council deliberation path: replace single reviewer with multi-model consensus.
	if c := o.buildCouncil(); c != nil && o.councilShouldTrigger(state) {
		o.log.Log("council", "start", fmt.Sprintf("Council deliberating with %d member(s), mode=%s", len(c.Members), c.SynthesisMode))
		prompt := buildReviewPromptForCouncil(input)
		verdict, err := c.Deliberate(context.Background(), prompt)
		if err != nil {
			o.log.Log("council", "error", fmt.Sprintf("Council failed, falling back to single reviewer: %v", err))
		} else {
			o.log.Log("council", "verdict", fmt.Sprintf("decision=%s confidence=%.2f consensus=%v dissent=%v", verdict.Decision, verdict.Confidence, verdict.Consensus, verdict.Dissent))
			for _, vote := range verdict.MemberVotes {
				o.recordUsageForModel(state, "reviewing", string(providers.RoleReviewer), vote.ProviderName+":"+vote.ModelID, vote.Usage)
			}
			providerReview := reviewResultFromCouncilVerdict(verdict)
			scorecard, finalReview := o.reviewEngine.Evaluate(state, providerReview)
			state.ReviewScorecard = scorecard
			state.Review = finalReview
			state.Confidence = o.confidenceScorer.Score(state)
			if state.Review == nil {
				return fmt.Errorf("review engine did not produce a review result")
			}
			if state.Confidence != nil {
				o.log.Log("confidence", "score", fmt.Sprintf("score=%.2f band=%s", state.Confidence.Score, state.Confidence.Band))
			}
			if err := o.confidencePolicy.Apply(state); err != nil {
				return fmt.Errorf("confidence policy blocked completion: %w", err)
			}
			o.log.Log("council", "review", fmt.Sprintf("Council review completed: %s", state.Review.Decision))
			return nil
		}
	}

	output, err := o.reviewer.Execute(input)
	if err != nil {
		return fmt.Errorf("review failed: %w", err)
	}
	if output != nil {
		o.recordUsage(state, "reviewing", string(providers.RoleReviewer), output.Usage)
	}

	var providerReview *models.ReviewResult
	if output != nil {
		providerReview = output.Review
	}
	scorecard, finalReview := o.reviewEngine.Evaluate(state, providerReview)
	state.ReviewScorecard = scorecard
	state.Review = finalReview
	state.Confidence = o.confidenceScorer.Score(state)
	if state.Review == nil {
		return fmt.Errorf("review engine did not produce a review result")
	}
	if state.Confidence != nil {
		o.log.Log("confidence", "score", fmt.Sprintf("score=%.2f band=%s", state.Confidence.Score, state.Confidence.Band))
	}
	if err := o.confidencePolicy.Apply(state); err != nil {
		return fmt.Errorf("confidence policy blocked completion: %w", err)
	}
	o.log.Log("reviewer", "review", fmt.Sprintf("Review completed: %s", state.Review.Decision))
	return nil
}

// buildReviewPromptForCouncil assembles the review prompt from an agent Input.
func buildReviewPromptForCouncil(input *agents.Input) string {
	if input == nil || input.Task == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("Task: ")
	b.WriteString(input.Task.Description)
	if input.TaskBrief != nil {
		b.WriteString("\nTask Type: ")
		b.WriteString(string(input.TaskBrief.TaskType))
		b.WriteString("\nRisk Level: ")
		b.WriteString(string(input.TaskBrief.RiskLevel))
	}
	if input.ExecutionContract != nil {
		if len(input.ExecutionContract.AllowedFiles) > 0 {
			b.WriteString("\nAllowed Files: ")
			b.WriteString(strings.Join(input.ExecutionContract.AllowedFiles, ", "))
		}
		if len(input.ExecutionContract.RequiredEdits) > 0 {
			b.WriteString("\nRequired Edits: ")
			b.WriteString(strings.Join(input.ExecutionContract.RequiredEdits, " | "))
		}
		if len(input.ExecutionContract.Invariants) > 0 {
			b.WriteString("\nInvariants: ")
			b.WriteString(strings.Join(input.ExecutionContract.Invariants, " | "))
		}
	}
	if input.Patch != nil {
		b.WriteString(fmt.Sprintf("\nPatch size: %d chars", len(input.Patch.RawDiff)))
		if len(input.Patch.Files) > 0 {
			paths := make([]string, 0, len(input.Patch.Files))
			for _, file := range input.Patch.Files {
				if strings.TrimSpace(file.Path) != "" {
					paths = append(paths, file.Path)
				}
			}
			if len(paths) > 0 {
				b.WriteString("\nTouched Files: ")
				b.WriteString(strings.Join(paths, ", "))
			}
		}
		trimmedDiff := truncateForPrompt(strings.TrimSpace(input.Patch.RawDiff), 12000)
		if trimmedDiff != "" {
			b.WriteString("\nPatch Diff:\n")
			b.WriteString(trimmedDiff)
		}
	}
	if len(input.ValidationResults) > 0 {
		parts := make([]string, 0, len(input.ValidationResults))
		for _, vr := range input.ValidationResults {
			part := fmt.Sprintf("%s=%s", vr.Name, vr.Status)
			if strings.TrimSpace(vr.Summary) != "" {
				part += " (" + vr.Summary + ")"
			}
			parts = append(parts, part)
		}
		b.WriteString("\nValidation Gates: ")
		b.WriteString(strings.Join(parts, ", "))
	}
	if input.TestResults != "" {
		b.WriteString("\nTest Results: ")
		b.WriteString(truncateForPrompt(input.TestResults, 4000))
	}
	if input.Plan != nil && len(input.Plan.AcceptanceCriteria) > 0 {
		crits := make([]string, 0, len(input.Plan.AcceptanceCriteria))
		for _, c := range input.Plan.AcceptanceCriteria {
			crits = append(crits, c.Description)
		}
		b.WriteString("\nAcceptance Criteria: ")
		b.WriteString(strings.Join(crits, " | "))
	}
	b.WriteString("\nDecide ACCEPT or REVISE with brief reasoning.")
	return b.String()
}

func reviewResultFromCouncilVerdict(verdict *council.CouncilVerdict) *models.ReviewResult {
	if verdict == nil {
		return nil
	}
	decision := verdict.Decision
	if !verdict.Consensus {
		decision = models.ReviewRevise
	}
	comments := []string{
		fmt.Sprintf("Council verdict: decision=%s confidence=%.2f consensus=%v dissent=%v", decision, verdict.Confidence, verdict.Consensus, verdict.Dissent),
		verdict.Reasoning,
	}
	if !verdict.Consensus {
		comments = append(comments, "Council did not reach consensus; review is downgraded to REVISE.")
	}
	for _, vote := range verdict.MemberVotes {
		comments = append(comments,
			fmt.Sprintf("[%s/%s] %s", vote.ProviderName, vote.ModelID, vote.Decision),
			vote.Reasoning,
		)
	}
	return &models.ReviewResult{
		Decision: decision,
		Comments: uniqueNonEmptyStrings(comments),
	}
}

func truncateForPrompt(text string, maxChars int) string {
	trimmed := strings.TrimSpace(text)
	if maxChars <= 0 || len(trimmed) <= maxChars {
		return trimmed
	}
	return trimmed[:maxChars] + "\n...[truncated]"
}

func uniqueNonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
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
		state.RetryDirective = o.retryBuilder.FromValidation(state, state.Retries.Validation)
		if state.RetryDirective != nil {
			o.log.Log("orchestrator", "retry_contract", fmt.Sprintf("stage=%s attempt=%d failed_gates=%s", state.RetryDirective.Stage, state.RetryDirective.Attempt, strings.Join(state.RetryDirective.FailedGates, ",")))
		}
		o.log.Log("orchestrator", "retry", fmt.Sprintf("Validation failed, retrying code generation (%d/%d)", state.Retries.Validation, maxRetries))
		if codeErr := o.stepCodeWithFixer(state); codeErr != nil {
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
		state.RetryDirective = o.retryBuilder.FromTest(state, state.Retries.Testing)
		if state.RetryDirective != nil {
			o.log.Log("orchestrator", "retry_contract", fmt.Sprintf("stage=%s attempt=%d failed_tests=%d", state.RetryDirective.Stage, state.RetryDirective.Attempt, len(state.RetryDirective.FailedTests)))
		}
		o.log.Log("orchestrator", "retry", fmt.Sprintf("Tests failed, retrying code generation (%d/%d)", state.Retries.Testing, maxRetries))
		if codeErr := o.stepCodeWithFixer(state); codeErr != nil {
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
		state.RetryDirective = o.retryBuilder.FromReview(state, state.Retries.Review)
		if state.RetryDirective != nil {
			o.log.Log("orchestrator", "retry_contract", fmt.Sprintf("stage=%s attempt=%d reasons=%d", state.RetryDirective.Stage, state.RetryDirective.Attempt, len(state.RetryDirective.Reasons)))
		}
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

func filterOutStage(results []models.ValidationResult, stage string) []models.ValidationResult {
	filtered := make([]models.ValidationResult, 0, len(results))
	for _, result := range results {
		if strings.EqualFold(strings.TrimSpace(result.Stage), strings.TrimSpace(stage)) {
			continue
		}
		filtered = append(filtered, result)
	}
	return filtered
}

func (o *Orchestrator) compilePlanArtifacts(task *models.Task, repoMap *models.RepoMap) (*models.TaskBrief, *models.Plan) {
	taskBrief := planning.NormalizeTask(task)
	compiledPlan := planning.CompilePlan(task, taskBrief, repoMap)
	return taskBrief, compiledPlan
}

// stepCodeWithFixer tries the Fixer agent first (when enabled and a patch exists),
// falling back to the full Coder on failure or when Fixer produces no output.
func (o *Orchestrator) stepCodeWithFixer(state *models.RunState) error {
	if o.cfg.Safety.FeatureFlags.FixerEnabled && o.fixer != nil && state.Patch != nil {
		o.log.Log("fixer", "fix", "Attempting surgical fix...")
		input := &agents.Input{
			Task:              &state.Task,
			TaskBrief:         state.TaskBrief,
			Plan:              state.Plan,
			ExecutionContract: state.ExecutionContract,
			Patch:             state.Patch,
			ValidationResults: state.ValidationResults,
			RetryDirective:    state.RetryDirective,
			MaxTokens:         o.cfg.Budget.FixerMaxTokens,
		}
		output, err := o.fixer.Execute(input)
		if err == nil && output != nil && output.Patch != nil && strings.TrimSpace(output.Patch.RawDiff) != "" {
			o.recordUsage(state, "fixing", string(providers.RoleFixer), output.Usage)
			state.Patch = output.Patch
			state.RetryDirective = nil
			o.log.Log("fixer", "fix", "Surgical fix applied")
			return nil
		}
		o.log.Log("fixer", "fallback", "Fixer produced no output, escalating to full coder")
	}
	return o.stepCode(state)
}

// contextDepthForRetry returns Shallow on first attempt, Standard on second, Deep on third+.
func contextDepthForRetry(state *models.RunState) models.ContextDepth {
	totalRetries := state.Retries.Validation + state.Retries.Testing + state.Retries.Review
	switch {
	case totalRetries == 0:
		return models.ContextDepthShallow
	case totalRetries == 1:
		return models.ContextDepthStandard
	default:
		return models.ContextDepthDeep
	}
}

// buildSkillRegistry creates the skills registry with built-in skills enabled per config.
func buildSkillRegistry(cfg *config.Config) *skills.Registry {
	reg := skills.DefaultRegistry()
	if cfg == nil {
		return reg
	}
	// Enable globally configured skills.
	for _, name := range cfg.Skills.GlobalSkills {
		if s, err := reg.Get(name); err == nil {
			s.Enabled = true
		}
	}
	// Enable per-agent skills (mark them enabled so hints are collected).
	for _, skillNames := range cfg.Skills.AgentSkills {
		for _, name := range skillNames {
			if s, err := reg.Get(name); err == nil {
				s.Enabled = true
			}
		}
	}
	return reg
}

// skillHintsForAgent returns the combined system hint string for an agent,
// merging global skills and agent-specific skills from config.
func (o *Orchestrator) skillHintsForAgent(agentName string) string {
	if o.skillRegistry == nil || o.cfg == nil {
		return ""
	}
	names := make([]string, 0)
	names = append(names, o.cfg.Skills.GlobalSkills...)
	if agentSkills, ok := o.cfg.Skills.AgentSkills[agentName]; ok {
		names = append(names, agentSkills...)
	}
	return o.skillRegistry.CollectHints(names)
}

// buildToolRegistry creates the tools registry with policy and MCP tools.
func buildToolRegistry(cfg *config.Config, repoRoot, mode string, logf func(string)) *tools.Registry {
	policy := buildPolicy(cfg, mode)
	registry := tools.DefaultRegistryWithPolicy(repoRoot, policy, logf)
	if cfg != nil {
		tools.RegisterMCPTools(registry, cfg.MCP)
	}
	return registry
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

func (o *Orchestrator) recordUsage(state *models.RunState, stage, role string, usage providers.Usage) {
	o.recordUsageForModel(state, stage, role, o.modelIDForRole(role), usage)
}

func (o *Orchestrator) recordUsageForModel(state *models.RunState, stage, role, modelID string, usage providers.Usage) {
	if usage.TotalTokens == 0 {
		return
	}
	cost := providers.EstimateCostUSD(modelID, usage.InputTokens, usage.OutputTokens)
	state.TokenUsages = append(state.TokenUsages, models.TokenUsage{
		Stage:         stage,
		Role:          role,
		InputTokens:   usage.InputTokens,
		OutputTokens:  usage.OutputTokens,
		EstimatedCost: cost,
	})
}

func (o *Orchestrator) modelIDForRole(role string) string {
	switch role {
	case string(providers.RolePlanner):
		return o.cfg.Provider.OpenAI.Models.Planner
	case string(providers.RoleCoder):
		return o.cfg.Provider.OpenAI.Models.Coder
	case string(providers.RoleReviewer):
		return o.cfg.Provider.OpenAI.Models.Reviewer
	default:
		return o.cfg.Provider.OpenAI.Models.Planner
	}
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
