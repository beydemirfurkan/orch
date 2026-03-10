package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
	_ "modernc.org/sqlite"
)

const (
	driverName         = "sqlite"
	activeSessionKeyNS = "active_session_id:"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionClosed   = errors.New("session is closed")
	ErrNameConflict    = errors.New("session name conflict")
	idCounter          uint64
)

type Store struct {
	repoRoot string
	db       *sql.DB
}

type Session struct {
	ID         string
	ProjectID  string
	Name       string
	Status     string
	Worktree   string
	CreatedAt  time.Time
	ClosedAt   *time.Time
	IsActive   bool
	SessionRef string
}

type RunRecord struct {
	ID          string
	SessionID   string
	Status      string
	Task        string
	StartedAt   time.Time
	CompletedAt *time.Time
	Error       string
}

type SessionMessage struct {
	ID           string
	SessionID    string
	Role         string
	ParentID     string
	ProviderID   string
	ModelID      string
	FinishReason string
	Error        string
	CreatedAt    time.Time
}

type SessionPart struct {
	ID        string
	MessageID string
	Type      string
	Payload   string
	Compacted bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SessionSummary struct {
	SessionID   string
	SummaryText string
	UpdatedAt   time.Time
}

type SessionMetrics struct {
	SessionID     string
	InputTokens   int
	OutputTokens  int
	TotalCost     float64
	TurnCount     int
	LastMessageID string
	UpdatedAt     time.Time
}

func Open(repoRoot string) (*Store, error) {
	if err := config.EnsureOrchDir(repoRoot); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(repoRoot, config.OrchDir, "orch.db")
	db, err := sql.Open(driverName, dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	stmts := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA busy_timeout = 5000;",
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("configure sqlite pragma: %w", err)
		}
	}

	store := &Store{repoRoot: repoRoot, db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) GetOrCreateProject() (string, error) {
	const selectSQL = `SELECT id FROM projects WHERE repo_root = ?`
	var projectID string
	if err := s.db.QueryRow(selectSQL, s.repoRoot).Scan(&projectID); err == nil {
		return projectID, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("query project: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	projectID = newID("proj")
	const insertSQL = `
		INSERT INTO projects(id, name, repo_root, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?)`
	if _, err := s.db.Exec(insertSQL, projectID, filepath.Base(s.repoRoot), s.repoRoot, now, now); err != nil {
		return "", fmt.Errorf("insert project: %w", err)
	}

	return projectID, nil
}

func (s *Store) EnsureDefaultSession(projectID string) (Session, error) {
	active, err := s.GetActiveSession(projectID)
	if err == nil {
		return active, nil
	}
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return Session{}, err
	}

	created, err := s.CreateSession(projectID, "default")
	if err != nil {
		if errors.Is(err, ErrNameConflict) {
			selected, selectErr := s.SelectSession(projectID, "default")
			if selectErr != nil {
				return Session{}, selectErr
			}
			return selected, nil
		}
		return Session{}, err
	}

	if err := s.SetActiveSession(projectID, created.ID); err != nil {
		return Session{}, err
	}
	created.IsActive = true
	return created, nil
}

func (s *Store) CreateSession(projectID, name string) (Session, error) {
	return s.CreateSessionWithWorktree(projectID, name, "")
}

func (s *Store) CreateSessionWithWorktree(projectID, name, worktreePath string) (Session, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Session{}, fmt.Errorf("session name is required")
	}

	const dupSQL = `SELECT id FROM sessions WHERE project_id = ? AND name = ?`
	var existingID string
	if err := s.db.QueryRow(dupSQL, projectID, name).Scan(&existingID); err == nil {
		return Session{}, ErrNameConflict
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Session{}, fmt.Errorf("check session name conflict: %w", err)
	}

	now := time.Now().UTC()
	session := Session{
		ID:        newID("sess"),
		ProjectID: projectID,
		Name:      name,
		Status:    "active",
		Worktree:  strings.TrimSpace(worktreePath),
		CreatedAt: now,
	}

	const insertSQL = `
		INSERT INTO sessions(id, project_id, name, status, worktree_path, created_at, closed_at)
		VALUES(?, ?, ?, ?, ?, ?, NULL)`
	if _, err := s.db.Exec(insertSQL, session.ID, session.ProjectID, session.Name, session.Status, session.Worktree, now.Format(time.RFC3339Nano)); err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}

	return session, nil
}

func (s *Store) ListSessions(projectID string) ([]Session, error) {
	activeID, _ := s.getMeta(activeSessionKey(projectID))

	const querySQL = `
		SELECT id, project_id, name, status, worktree_path, created_at, closed_at
		FROM sessions WHERE project_id = ? ORDER BY created_at DESC`
	rows, err := s.db.Query(querySQL, projectID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	result := make([]Session, 0)
	for rows.Next() {
		var sRow Session
		var createdAt string
		var closedAt sql.NullString
		if err := rows.Scan(&sRow.ID, &sRow.ProjectID, &sRow.Name, &sRow.Status, &sRow.Worktree, &createdAt, &closedAt); err != nil {
			return nil, fmt.Errorf("scan session row: %w", err)
		}
		parsedCreated, _ := time.Parse(time.RFC3339Nano, createdAt)
		sRow.CreatedAt = parsedCreated
		if closedAt.Valid {
			parsedClosed, parseErr := time.Parse(time.RFC3339Nano, closedAt.String)
			if parseErr == nil {
				sRow.ClosedAt = &parsedClosed
			}
		}
		sRow.IsActive = sRow.ID == activeID
		sRow.SessionRef = fmt.Sprintf("%s (%s)", sRow.Name, sRow.ID)
		result = append(result, sRow)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	return result, nil
}

func (s *Store) SelectSession(projectID, nameOrID string) (Session, error) {
	session, err := s.findSession(projectID, nameOrID)
	if err != nil {
		return Session{}, err
	}
	if session.Status == "closed" {
		return Session{}, ErrSessionClosed
	}

	if err := s.SetActiveSession(projectID, session.ID); err != nil {
		return Session{}, err
	}
	session.IsActive = true
	return session, nil
}

func (s *Store) GetSession(projectID, nameOrID string) (Session, error) {
	session, err := s.findSession(projectID, nameOrID)
	if err != nil {
		return Session{}, err
	}
	activeID, _ := s.getMeta(activeSessionKey(projectID))
	session.IsActive = session.ID == activeID
	return session, nil
}

func (s *Store) CloseSession(projectID, nameOrID string) error {
	session, err := s.findSession(projectID, nameOrID)
	if err != nil {
		return err
	}
	if session.Status == "closed" {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	const updateSQL = `UPDATE sessions SET status='closed', closed_at=? WHERE id=?`
	if _, err := s.db.Exec(updateSQL, now, session.ID); err != nil {
		return fmt.Errorf("close session: %w", err)
	}

	activeID, _ := s.getMeta(activeSessionKey(projectID))
	if activeID == session.ID {
		if session.Name != "default" {
			if fallback, selErr := s.SelectSession(projectID, "default"); selErr == nil {
				_ = s.SetActiveSession(projectID, fallback.ID)
				return nil
			}
		}
		if err := s.setMeta(activeSessionKey(projectID), ""); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) GetActiveSession(projectID string) (Session, error) {
	activeID, err := s.getMeta(activeSessionKey(projectID))
	if err != nil {
		return Session{}, err
	}
	activeID = strings.TrimSpace(activeID)
	if activeID == "" {
		return Session{}, ErrSessionNotFound
	}

	session, err := s.findSession(projectID, activeID)
	if err != nil {
		return Session{}, err
	}
	if session.Status == "closed" {
		return Session{}, ErrSessionClosed
	}
	session.IsActive = true
	return session, nil
}

func (s *Store) SetActiveSession(projectID, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return s.setMeta(activeSessionKey(projectID), "")
	}

	session, err := s.findSession(projectID, sessionID)
	if err != nil {
		return err
	}
	if session.Status == "closed" {
		return ErrSessionClosed
	}
	return s.setMeta(activeSessionKey(projectID), session.ID)
}

func (s *Store) SaveRunState(state *models.RunState) error {
	if state == nil {
		return fmt.Errorf("run state cannot be nil")
	}
	if strings.TrimSpace(state.ProjectID) == "" || strings.TrimSpace(state.SessionID) == "" {
		return fmt.Errorf("run state missing project/session metadata")
	}

	taskJSON, _ := json.Marshal(state.Task)
	taskBriefJSON, _ := json.Marshal(state.TaskBrief)
	planJSON, _ := json.Marshal(state.Plan)
	executionContractJSON, _ := json.Marshal(state.ExecutionContract)
	patchJSON, _ := json.Marshal(state.Patch)
	validationResultsJSON, _ := json.Marshal(state.ValidationResults)
	retryDirectiveJSON, _ := json.Marshal(state.RetryDirective)
	reviewJSON, _ := json.Marshal(state.Review)
	reviewScorecardJSON, _ := json.Marshal(state.ReviewScorecard)
	confidenceJSON, _ := json.Marshal(state.Confidence)
	testFailuresJSON, _ := json.Marshal(state.TestFailures)
	retriesJSON, _ := json.Marshal(state.Retries)
	unresolvedJSON, _ := json.Marshal(state.UnresolvedFailures)

	completedAt := sql.NullString{}
	if state.CompletedAt != nil {
		completedAt = sql.NullString{String: state.CompletedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	const upsertRun = `
		INSERT INTO runs(
			id, project_id, session_id, task_json, task_brief_json, status, plan_json, execution_contract_json,
			patch_json, validation_results_json, retry_directive_json, review_json, review_scorecard_json, confidence_json, test_failures_json, test_results, retries_json,
			unresolved_failures_json, best_patch_summary, error, started_at, completed_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			task_brief_json=excluded.task_brief_json,
			status=excluded.status,
			plan_json=excluded.plan_json,
			execution_contract_json=excluded.execution_contract_json,
			patch_json=excluded.patch_json,
			validation_results_json=excluded.validation_results_json,
			retry_directive_json=excluded.retry_directive_json,
			review_json=excluded.review_json,
			review_scorecard_json=excluded.review_scorecard_json,
			confidence_json=excluded.confidence_json,
			test_failures_json=excluded.test_failures_json,
			test_results=excluded.test_results,
			retries_json=excluded.retries_json,
			unresolved_failures_json=excluded.unresolved_failures_json,
			best_patch_summary=excluded.best_patch_summary,
			error=excluded.error,
			completed_at=excluded.completed_at`

	if _, err := s.db.Exec(upsertRun,
		state.ID,
		state.ProjectID,
		state.SessionID,
		string(taskJSON),
		nullJSON(taskBriefJSON),
		string(state.Status),
		nullJSON(planJSON),
		nullJSON(executionContractJSON),
		nullJSON(patchJSON),
		nullJSON(validationResultsJSON),
		nullJSON(retryDirectiveJSON),
		nullJSON(reviewJSON),
		nullJSON(reviewScorecardJSON),
		nullJSON(confidenceJSON),
		nullJSON(testFailuresJSON),
		nullString(state.TestResults),
		string(retriesJSON),
		nullJSON(unresolvedJSON),
		nullString(state.BestPatchSummary),
		nullString(state.Error),
		state.StartedAt.UTC().Format(time.RFC3339Nano),
		nullStringFromNull(completedAt),
	); err != nil {
		return fmt.Errorf("upsert run: %w", err)
	}

	if _, err := s.db.Exec(`DELETE FROM run_logs WHERE run_id = ?`, state.ID); err != nil {
		return fmt.Errorf("clear run logs: %w", err)
	}

	const insertLog = `INSERT INTO run_logs(run_id, timestamp, actor, step, message) VALUES(?, ?, ?, ?, ?)`
	for _, entry := range state.Logs {
		if _, err := s.db.Exec(insertLog, state.ID, entry.Timestamp.UTC().Format(time.RFC3339Nano), entry.Actor, entry.Step, entry.Message); err != nil {
			return fmt.Errorf("insert run log: %w", err)
		}
	}

	return nil
}

func (s *Store) ListRunsBySession(sessionID string, limit int) ([]RunRecord, error) {
	return s.ListRunsBySessionFiltered(sessionID, limit, "", "")
}

func (s *Store) ListRunsBySessionFiltered(sessionID string, limit int, statusFilter, containsFilter string) ([]RunRecord, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session id is required")
	}
	if limit <= 0 {
		limit = 20
	}

	q := `
		SELECT id, session_id, status, task_json, started_at, completed_at, error
		FROM runs
		WHERE session_id = ?`
	args := []any{sessionID}

	if strings.TrimSpace(statusFilter) != "" {
		q += ` AND status = ?`
		args = append(args, strings.TrimSpace(statusFilter))
	}

	q += `
		ORDER BY started_at DESC
		LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list runs by session: %w", err)
	}
	defer rows.Close()

	result := make([]RunRecord, 0)
	for rows.Next() {
		var rec RunRecord
		var taskJSON string
		var startedAt string
		var completedAt sql.NullString
		if err := rows.Scan(&rec.ID, &rec.SessionID, &rec.Status, &taskJSON, &startedAt, &completedAt, &rec.Error); err != nil {
			return nil, fmt.Errorf("scan run row: %w", err)
		}

		var task models.Task
		if err := json.Unmarshal([]byte(taskJSON), &task); err == nil {
			rec.Task = task.Description
		}
		if rec.Task == "" {
			rec.Task = "(unknown task)"
		}

		if ts, parseErr := time.Parse(time.RFC3339Nano, startedAt); parseErr == nil {
			rec.StartedAt = ts
		}
		if completedAt.Valid {
			if ts, parseErr := time.Parse(time.RFC3339Nano, completedAt.String); parseErr == nil {
				rec.CompletedAt = &ts
			}
		}

		if strings.TrimSpace(containsFilter) != "" {
			needle := strings.ToLower(strings.TrimSpace(containsFilter))
			if !strings.Contains(strings.ToLower(rec.Task), needle) {
				continue
			}
		}

		result = append(result, rec)
		if len(result) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runs by session: %w", err)
	}

	return result, nil
}

func (s *Store) GetLatestRunStateBySession(sessionID string) (*models.RunState, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session id is required")
	}

	row := s.db.QueryRow(
		`SELECT id, project_id, session_id, task_json, task_brief_json, status, plan_json, execution_contract_json,
		        patch_json, validation_results_json, retry_directive_json, review_json, review_scorecard_json,
		        confidence_json, test_failures_json, test_results, retries_json, unresolved_failures_json,
		        best_patch_summary, error, started_at, completed_at
		 FROM runs
		 WHERE session_id = ?
		 ORDER BY started_at DESC
		 LIMIT 1`,
		sessionID,
	)

	state, err := scanRunState(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("run state not found")
		}
		return nil, err
	}
	logs, logErr := s.listRunLogs(state.ID)
	if logErr != nil {
		return nil, logErr
	}
	state.Logs = logs
	return state, nil
}

func (s *Store) GetRunState(projectID, runID string) (*models.RunState, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("run id is required")
	}

	row := s.db.QueryRow(
		`SELECT id, project_id, session_id, task_json, task_brief_json, status, plan_json, execution_contract_json,
		        patch_json, validation_results_json, retry_directive_json, review_json, review_scorecard_json,
		        confidence_json, test_failures_json, test_results, retries_json, unresolved_failures_json,
		        best_patch_summary, error, started_at, completed_at
		 FROM runs
		 WHERE project_id = ? AND id = ?
		 LIMIT 1`,
		projectID,
		runID,
	)

	state, err := scanRunState(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("run state not found")
		}
		return nil, err
	}
	logs, logErr := s.listRunLogs(state.ID)
	if logErr != nil {
		return nil, logErr
	}
	state.Logs = logs
	return state, nil
}

func (s *Store) ListRunStatesByProject(projectID string, limit int) ([]*models.RunState, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project id is required")
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(
		`SELECT id, project_id, session_id, task_json, task_brief_json, status, plan_json, execution_contract_json,
		        patch_json, validation_results_json, retry_directive_json, review_json, review_scorecard_json,
		        confidence_json, test_failures_json, test_results, retries_json, unresolved_failures_json,
		        best_patch_summary, error, started_at, completed_at
		 FROM runs
		 WHERE project_id = ?
		 ORDER BY started_at DESC
		 LIMIT ?`,
		projectID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list run states: %w", err)
	}
	defer rows.Close()

	states := make([]*models.RunState, 0)
	for rows.Next() {
		state, scanErr := scanRunState(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		logs, logErr := s.listRunLogs(state.ID)
		if logErr != nil {
			return nil, logErr
		}
		state.Logs = logs
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run states: %w", err)
	}

	return states, nil
}

func (s *Store) LoadLatestPatchBySession(sessionID string) (string, error) {
	state, err := s.GetLatestRunStateBySession(sessionID)
	if err != nil {
		return "", err
	}
	if state == nil || state.Patch == nil || strings.TrimSpace(state.Patch.RawDiff) == "" {
		return "", fmt.Errorf("latest patch not found")
	}
	return state.Patch.RawDiff, nil
}

func (s *Store) listRunLogs(runID string) ([]models.LogEntry, error) {
	rows, err := s.db.Query(
		`SELECT timestamp, actor, step, message FROM run_logs WHERE run_id = ? ORDER BY id ASC`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("list run logs: %w", err)
	}
	defer rows.Close()

	logs := make([]models.LogEntry, 0)
	for rows.Next() {
		var ts string
		var entry models.LogEntry
		if err := rows.Scan(&ts, &entry.Actor, &entry.Step, &entry.Message); err != nil {
			return nil, fmt.Errorf("scan run log: %w", err)
		}
		if parsed, parseErr := time.Parse(time.RFC3339Nano, ts); parseErr == nil {
			entry.Timestamp = parsed
		}
		logs = append(logs, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run logs: %w", err)
	}
	return logs, nil
}

type runStateScanner interface {
	Scan(dest ...any) error
}

func scanRunState(scanner runStateScanner) (*models.RunState, error) {
	var (
		id                    string
		projectID             string
		sessionID             string
		taskJSON              string
		taskBriefJSON         sql.NullString
		status                string
		planJSON              sql.NullString
		executionContractJSON sql.NullString
		patchJSON             sql.NullString
		validationJSON        sql.NullString
		retryDirectiveJSON    sql.NullString
		reviewJSON            sql.NullString
		reviewScorecardJSON   sql.NullString
		confidenceJSON        sql.NullString
		testFailuresJSON      sql.NullString
		testResults           sql.NullString
		retriesJSON           string
		unresolvedJSON        sql.NullString
		bestPatchSummary      sql.NullString
		errorText             sql.NullString
		startedAt             string
		completedAt           sql.NullString
	)

	if err := scanner.Scan(
		&id,
		&projectID,
		&sessionID,
		&taskJSON,
		&taskBriefJSON,
		&status,
		&planJSON,
		&executionContractJSON,
		&patchJSON,
		&validationJSON,
		&retryDirectiveJSON,
		&reviewJSON,
		&reviewScorecardJSON,
		&confidenceJSON,
		&testFailuresJSON,
		&testResults,
		&retriesJSON,
		&unresolvedJSON,
		&bestPatchSummary,
		&errorText,
		&startedAt,
		&completedAt,
	); err != nil {
		return nil, err
	}

	state := &models.RunState{
		ID:        id,
		ProjectID: projectID,
		SessionID: sessionID,
		Status:    models.RunStatus(status),
		Logs:      []models.LogEntry{},
	}

	if parsed, parseErr := time.Parse(time.RFC3339Nano, startedAt); parseErr == nil {
		state.StartedAt = parsed
	}
	if completedAt.Valid {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, completedAt.String); parseErr == nil {
			state.CompletedAt = &parsed
		}
	}
	state.Error = strings.TrimSpace(errorText.String)
	state.TestResults = strings.TrimSpace(testResults.String)
	state.BestPatchSummary = strings.TrimSpace(bestPatchSummary.String)

	if err := json.Unmarshal([]byte(taskJSON), &state.Task); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}

	if strings.TrimSpace(taskBriefJSON.String) != "" {
		state.TaskBrief = &models.TaskBrief{}
		if err := json.Unmarshal([]byte(taskBriefJSON.String), state.TaskBrief); err != nil {
			return nil, fmt.Errorf("unmarshal task brief: %w", err)
		}
	}
	if strings.TrimSpace(planJSON.String) != "" {
		state.Plan = &models.Plan{}
		if err := json.Unmarshal([]byte(planJSON.String), state.Plan); err != nil {
			return nil, fmt.Errorf("unmarshal plan: %w", err)
		}
	}
	if strings.TrimSpace(executionContractJSON.String) != "" {
		state.ExecutionContract = &models.ExecutionContract{}
		if err := json.Unmarshal([]byte(executionContractJSON.String), state.ExecutionContract); err != nil {
			return nil, fmt.Errorf("unmarshal execution contract: %w", err)
		}
	}
	if strings.TrimSpace(patchJSON.String) != "" {
		state.Patch = &models.Patch{}
		if err := json.Unmarshal([]byte(patchJSON.String), state.Patch); err != nil {
			return nil, fmt.Errorf("unmarshal patch: %w", err)
		}
	}
	if strings.TrimSpace(validationJSON.String) != "" {
		if err := json.Unmarshal([]byte(validationJSON.String), &state.ValidationResults); err != nil {
			return nil, fmt.Errorf("unmarshal validation results: %w", err)
		}
	}
	if strings.TrimSpace(retryDirectiveJSON.String) != "" {
		state.RetryDirective = &models.RetryDirective{}
		if err := json.Unmarshal([]byte(retryDirectiveJSON.String), state.RetryDirective); err != nil {
			return nil, fmt.Errorf("unmarshal retry directive: %w", err)
		}
	}
	if strings.TrimSpace(reviewJSON.String) != "" {
		state.Review = &models.ReviewResult{}
		if err := json.Unmarshal([]byte(reviewJSON.String), state.Review); err != nil {
			return nil, fmt.Errorf("unmarshal review: %w", err)
		}
	}
	if strings.TrimSpace(reviewScorecardJSON.String) != "" {
		state.ReviewScorecard = &models.ReviewScorecard{}
		if err := json.Unmarshal([]byte(reviewScorecardJSON.String), state.ReviewScorecard); err != nil {
			return nil, fmt.Errorf("unmarshal review scorecard: %w", err)
		}
	}
	if strings.TrimSpace(confidenceJSON.String) != "" {
		state.Confidence = &models.ConfidenceReport{}
		if err := json.Unmarshal([]byte(confidenceJSON.String), state.Confidence); err != nil {
			return nil, fmt.Errorf("unmarshal confidence: %w", err)
		}
	}
	if strings.TrimSpace(testFailuresJSON.String) != "" {
		if err := json.Unmarshal([]byte(testFailuresJSON.String), &state.TestFailures); err != nil {
			return nil, fmt.Errorf("unmarshal test failures: %w", err)
		}
	}
	if strings.TrimSpace(retriesJSON) != "" {
		if err := json.Unmarshal([]byte(retriesJSON), &state.Retries); err != nil {
			return nil, fmt.Errorf("unmarshal retries: %w", err)
		}
	}
	if strings.TrimSpace(unresolvedJSON.String) != "" {
		if err := json.Unmarshal([]byte(unresolvedJSON.String), &state.UnresolvedFailures); err != nil {
			return nil, fmt.Errorf("unmarshal unresolved failures: %w", err)
		}
	}

	return state, nil
}

func (s *Store) CreateMessageWithParts(message SessionMessage, parts []SessionPart) (SessionMessage, []SessionPart, error) {
	if strings.TrimSpace(message.SessionID) == "" {
		return SessionMessage{}, nil, fmt.Errorf("session id is required")
	}
	message.Role = strings.ToLower(strings.TrimSpace(message.Role))
	if message.Role != "user" && message.Role != "assistant" && message.Role != "system" {
		return SessionMessage{}, nil, fmt.Errorf("invalid message role: %s", message.Role)
	}
	if strings.TrimSpace(message.ID) == "" {
		message.ID = newID("msg")
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	} else {
		message.CreatedAt = message.CreatedAt.UTC()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return SessionMessage{}, nil, fmt.Errorf("begin message transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	parentID := strings.TrimSpace(message.ParentID)
	if _, err := tx.Exec(
		`INSERT INTO session_messages(id, session_id, role, parent_id, provider_id, model_id, finish_reason, error, created_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		message.ID,
		message.SessionID,
		message.Role,
		nullString(parentID),
		nullString(message.ProviderID),
		nullString(message.ModelID),
		nullString(message.FinishReason),
		nullString(message.Error),
		message.CreatedAt.Format(time.RFC3339Nano),
	); err != nil {
		return SessionMessage{}, nil, fmt.Errorf("insert session message: %w", err)
	}

	inserted := make([]SessionPart, 0, len(parts))
	for _, part := range parts {
		partType := strings.ToLower(strings.TrimSpace(part.Type))
		if partType == "" {
			return SessionMessage{}, nil, fmt.Errorf("part type is required")
		}
		if strings.TrimSpace(part.ID) == "" {
			part.ID = newID("part")
		}
		part.MessageID = message.ID
		now := time.Now().UTC()
		if part.CreatedAt.IsZero() {
			part.CreatedAt = now
		} else {
			part.CreatedAt = part.CreatedAt.UTC()
		}
		if part.UpdatedAt.IsZero() {
			part.UpdatedAt = part.CreatedAt
		} else {
			part.UpdatedAt = part.UpdatedAt.UTC()
		}
		part.Type = partType

		compacted := 0
		if part.Compacted {
			compacted = 1
		}

		if _, err := tx.Exec(
			`INSERT INTO session_parts(id, message_id, type, payload_json, compacted, created_at, updated_at)
			 VALUES(?, ?, ?, ?, ?, ?, ?)`,
			part.ID,
			part.MessageID,
			part.Type,
			nullString(part.Payload),
			compacted,
			part.CreatedAt.Format(time.RFC3339Nano),
			part.UpdatedAt.Format(time.RFC3339Nano),
		); err != nil {
			return SessionMessage{}, nil, fmt.Errorf("insert session part: %w", err)
		}

		inserted = append(inserted, part)
	}

	if err := tx.Commit(); err != nil {
		return SessionMessage{}, nil, fmt.Errorf("commit message transaction: %w", err)
	}

	return message, inserted, nil
}

func (s *Store) ListSessionMessages(sessionID string, limit int) ([]SessionMessage, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session id is required")
	}
	if limit <= 0 {
		limit = 200
	}

	rows, err := s.db.Query(
		`SELECT id, session_id, role, parent_id, provider_id, model_id, finish_reason, error, created_at
		 FROM session_messages
		 WHERE session_id = ?
		 ORDER BY created_at ASC, id ASC
		 LIMIT ?`,
		sessionID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list session messages: %w", err)
	}
	defer rows.Close()

	messages := make([]SessionMessage, 0)
	for rows.Next() {
		var item SessionMessage
		var parentID sql.NullString
		var providerID sql.NullString
		var modelID sql.NullString
		var finishReason sql.NullString
		var errText sql.NullString
		var createdAt string
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Role, &parentID, &providerID, &modelID, &finishReason, &errText, &createdAt); err != nil {
			return nil, fmt.Errorf("scan session message: %w", err)
		}
		item.ParentID = strings.TrimSpace(parentID.String)
		item.ProviderID = strings.TrimSpace(providerID.String)
		item.ModelID = strings.TrimSpace(modelID.String)
		item.FinishReason = strings.TrimSpace(finishReason.String)
		item.Error = strings.TrimSpace(errText.String)
		if ts, parseErr := time.Parse(time.RFC3339Nano, createdAt); parseErr == nil {
			item.CreatedAt = ts
		}
		messages = append(messages, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session messages: %w", err)
	}

	return messages, nil
}

func (s *Store) ListSessionParts(messageID string) ([]SessionPart, error) {
	if strings.TrimSpace(messageID) == "" {
		return nil, fmt.Errorf("message id is required")
	}

	rows, err := s.db.Query(
		`SELECT id, message_id, type, payload_json, compacted, created_at, updated_at
		 FROM session_parts
		 WHERE message_id = ?
		 ORDER BY created_at ASC, id ASC`,
		messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("list session parts: %w", err)
	}
	defer rows.Close()

	parts := make([]SessionPart, 0)
	for rows.Next() {
		var item SessionPart
		var compacted int
		var payload sql.NullString
		var createdAt string
		var updatedAt string
		if err := rows.Scan(&item.ID, &item.MessageID, &item.Type, &payload, &compacted, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan session part: %w", err)
		}
		item.Payload = payload.String
		item.Compacted = compacted != 0
		if ts, parseErr := time.Parse(time.RFC3339Nano, createdAt); parseErr == nil {
			item.CreatedAt = ts
		}
		if ts, parseErr := time.Parse(time.RFC3339Nano, updatedAt); parseErr == nil {
			item.UpdatedAt = ts
		}
		parts = append(parts, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session parts: %w", err)
	}

	return parts, nil
}

func (s *Store) UpsertSessionSummary(sessionID, summaryText string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(
		`INSERT INTO session_summaries(session_id, summary_text, updated_at)
		 VALUES(?, ?, ?)
		 ON CONFLICT(session_id) DO UPDATE SET summary_text=excluded.summary_text, updated_at=excluded.updated_at`,
		sessionID,
		nullString(summaryText),
		now,
	)
	if err != nil {
		return fmt.Errorf("upsert session summary: %w", err)
	}
	return nil
}

func (s *Store) GetSessionSummary(sessionID string) (*SessionSummary, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session id is required")
	}
	var summary SessionSummary
	var updatedAt string
	if err := s.db.QueryRow(
		`SELECT session_id, summary_text, updated_at FROM session_summaries WHERE session_id = ?`,
		sessionID,
	).Scan(&summary.SessionID, &summary.SummaryText, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query session summary: %w", err)
	}
	if ts, parseErr := time.Parse(time.RFC3339Nano, updatedAt); parseErr == nil {
		summary.UpdatedAt = ts
	}
	return &summary, nil
}

func (s *Store) UpsertSessionMetrics(metrics SessionMetrics) error {
	if strings.TrimSpace(metrics.SessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	metrics.UpdatedAt = time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO session_metrics(session_id, input_tokens, output_tokens, total_cost, turn_count, last_message_id, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(session_id) DO UPDATE SET
		   input_tokens=excluded.input_tokens,
		   output_tokens=excluded.output_tokens,
		   total_cost=excluded.total_cost,
		   turn_count=excluded.turn_count,
		   last_message_id=excluded.last_message_id,
		   updated_at=excluded.updated_at`,
		metrics.SessionID,
		metrics.InputTokens,
		metrics.OutputTokens,
		metrics.TotalCost,
		metrics.TurnCount,
		nullString(metrics.LastMessageID),
		metrics.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert session metrics: %w", err)
	}
	return nil
}

func (s *Store) GetSessionMetrics(sessionID string) (*SessionMetrics, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session id is required")
	}
	var metrics SessionMetrics
	var lastMessageID sql.NullString
	var updatedAt string
	if err := s.db.QueryRow(
		`SELECT session_id, input_tokens, output_tokens, total_cost, turn_count, last_message_id, updated_at
		 FROM session_metrics WHERE session_id = ?`,
		sessionID,
	).Scan(
		&metrics.SessionID,
		&metrics.InputTokens,
		&metrics.OutputTokens,
		&metrics.TotalCost,
		&metrics.TurnCount,
		&lastMessageID,
		&updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query session metrics: %w", err)
	}
	metrics.LastMessageID = strings.TrimSpace(lastMessageID.String)
	if ts, parseErr := time.Parse(time.RFC3339Nano, updatedAt); parseErr == nil {
		metrics.UpdatedAt = ts
	}
	return &metrics, nil
}

func (s *Store) CompactSessionParts(sessionID string, keepLastMessages int) (int64, error) {
	if strings.TrimSpace(sessionID) == "" {
		return 0, fmt.Errorf("session id is required")
	}
	if keepLastMessages <= 0 {
		keepLastMessages = 12
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.Exec(
		`WITH keep AS (
			SELECT id
			FROM session_messages
			WHERE session_id = ?
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		 )
		 UPDATE session_parts
		 SET compacted = 1,
		     payload_json = '[Old content compacted]',
		     updated_at = ?
		 WHERE compacted = 0
		   AND message_id IN (
			 SELECT id
			 FROM session_messages
			 WHERE session_id = ?
			   AND id NOT IN (SELECT id FROM keep)
		 )`,
		sessionID,
		keepLastMessages,
		now,
		sessionID,
	)
	if err != nil {
		return 0, fmt.Errorf("compact session parts: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read compacted rows: %w", err)
	}
	return affected, nil
}

func (s *Store) migrate() error {
	const createMigrations = `
		CREATE TABLE IF NOT EXISTS schema_migrations(
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		);`
	if _, err := s.db.Exec(createMigrations); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = 1`).Scan(&count); err != nil {
		return fmt.Errorf("check schema version: %w", err)
	}
	if count > 0 {
		if err := s.ensureRunColumns(); err != nil {
			return err
		}
		return s.ensureSessionStoreTables()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS projects(
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			repo_root TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS sessions(
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id),
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			worktree_path TEXT,
			created_at TEXT NOT NULL,
			closed_at TEXT,
			UNIQUE(project_id, name)
		);`,
		`CREATE TABLE IF NOT EXISTS runs(
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id),
			session_id TEXT NOT NULL REFERENCES sessions(id),
			task_json TEXT NOT NULL,
			task_brief_json TEXT,
			status TEXT NOT NULL,
			plan_json TEXT,
			execution_contract_json TEXT,
			patch_json TEXT,
			validation_results_json TEXT,
			retry_directive_json TEXT,
			review_json TEXT,
			review_scorecard_json TEXT,
			confidence_json TEXT,
			test_failures_json TEXT,
			test_results TEXT,
			retries_json TEXT,
			unresolved_failures_json TEXT,
			best_patch_summary TEXT,
			error TEXT,
			started_at TEXT NOT NULL,
			completed_at TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS run_logs(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL REFERENCES runs(id),
			timestamp TEXT NOT NULL,
			actor TEXT NOT NULL,
			step TEXT NOT NULL,
			message TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS meta(
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS session_messages(
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			parent_id TEXT,
			provider_id TEXT,
			model_id TEXT,
			finish_reason TEXT,
			error TEXT,
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_session_messages_session_created
			ON session_messages(session_id, created_at, id);`,
		`CREATE TABLE IF NOT EXISTS session_parts(
			id TEXT PRIMARY KEY,
			message_id TEXT NOT NULL REFERENCES session_messages(id) ON DELETE CASCADE,
			type TEXT NOT NULL,
			payload_json TEXT,
			compacted INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_session_parts_message_created
			ON session_parts(message_id, created_at, id);`,
		`CREATE TABLE IF NOT EXISTS session_summaries(
			session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
			summary_text TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS session_metrics(
			session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			total_cost REAL NOT NULL DEFAULT 0,
			turn_count INTEGER NOT NULL DEFAULT 0,
			last_message_id TEXT,
			updated_at TEXT NOT NULL
		);`,
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("apply migration v1: %w", err)
		}
	}

	if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(1, ?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("record migration version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}
	if err := s.ensureRunColumns(); err != nil {
		return err
	}
	return s.ensureSessionStoreTables()
}

func (s *Store) ensureSessionStoreTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS session_messages(
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			parent_id TEXT,
			provider_id TEXT,
			model_id TEXT,
			finish_reason TEXT,
			error TEXT,
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_session_messages_session_created
			ON session_messages(session_id, created_at, id);`,
		`CREATE TABLE IF NOT EXISTS session_parts(
			id TEXT PRIMARY KEY,
			message_id TEXT NOT NULL REFERENCES session_messages(id) ON DELETE CASCADE,
			type TEXT NOT NULL,
			payload_json TEXT,
			compacted INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_session_parts_message_created
			ON session_parts(message_id, created_at, id);`,
		`CREATE TABLE IF NOT EXISTS session_summaries(
			session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
			summary_text TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS session_metrics(
			session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			total_cost REAL NOT NULL DEFAULT 0,
			turn_count INTEGER NOT NULL DEFAULT 0,
			last_message_id TEXT,
			updated_at TEXT NOT NULL
		);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("ensure session store tables: %w", err)
		}
	}

	return nil
}

func (s *Store) ensureRunColumns() error {
	existing, err := s.runColumns()
	if err != nil {
		return err
	}

	required := map[string]string{
		"task_brief_json":         "TEXT",
		"execution_contract_json": "TEXT",
		"validation_results_json": "TEXT",
		"retry_directive_json":    "TEXT",
		"review_scorecard_json":   "TEXT",
		"confidence_json":         "TEXT",
		"test_failures_json":      "TEXT",
	}

	for column, definition := range required {
		if _, ok := existing[column]; ok {
			continue
		}
		statement := fmt.Sprintf("ALTER TABLE runs ADD COLUMN %s %s", column, definition)
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("add runs column %s: %w", column, err)
		}
	}

	return nil
}

func (s *Store) runColumns() (map[string]struct{}, error) {
	rows, err := s.db.Query("PRAGMA table_info(runs)")
	if err != nil {
		return nil, fmt.Errorf("query runs table info: %w", err)
	}
	defer rows.Close()

	columns := make(map[string]struct{})
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("scan runs table info: %w", err)
		}
		columns[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runs table info: %w", err)
	}
	return columns, nil
}

func (s *Store) findSession(projectID, nameOrID string) (Session, error) {
	nameOrID = strings.TrimSpace(nameOrID)
	const q = `
		SELECT id, project_id, name, status, worktree_path, created_at, closed_at
		FROM sessions
		WHERE project_id = ? AND (id = ? OR name = ?)
		LIMIT 1`

	var sess Session
	var createdAt string
	var closedAt sql.NullString
	if err := s.db.QueryRow(q, projectID, nameOrID, nameOrID).Scan(
		&sess.ID,
		&sess.ProjectID,
		&sess.Name,
		&sess.Status,
		&sess.Worktree,
		&createdAt,
		&closedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, fmt.Errorf("query session: %w", err)
	}

	created, parseErr := time.Parse(time.RFC3339Nano, createdAt)
	if parseErr == nil {
		sess.CreatedAt = created
	}
	if closedAt.Valid {
		if closed, cErr := time.Parse(time.RFC3339Nano, closedAt.String); cErr == nil {
			sess.ClosedAt = &closed
		}
	}
	return sess, nil
}

func (s *Store) getMeta(key string) (string, error) {
	var value string
	if err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrSessionNotFound
		}
		return "", fmt.Errorf("query meta: %w", err)
	}
	return value, nil
}

func (s *Store) setMeta(key, value string) error {
	const upsert = `
		INSERT INTO meta(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`
	if _, err := s.db.Exec(upsert, key, value); err != nil {
		return fmt.Errorf("upsert meta: %w", err)
	}
	return nil
}

func activeSessionKey(projectID string) string {
	return activeSessionKeyNS + projectID
}

func newID(prefix string) string {
	seq := atomic.AddUint64(&idCounter, 1)
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), seq)
}

func nullString(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return ""
	}
	return v
}

func nullJSON(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	if string(data) == "null" {
		return ""
	}
	return string(data)
}

func nullStringFromNull(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}
