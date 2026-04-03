package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/furkanbeydemir/orch/internal/config"
	"github.com/furkanbeydemir/orch/internal/models"
)

const mcpToolTimeout = 30 * time.Second

// MCPTool implements the Tool interface for an external MCP server.
// Supports both HTTP endpoints (URL field) and stdio commands (Command field).
type MCPTool struct {
	cfg        config.MCPServerConfig
	httpClient *http.Client
}

func newMCPTool(cfg config.MCPServerConfig) *MCPTool {
	return &MCPTool{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: mcpToolTimeout},
	}
}

func (m *MCPTool) Name() string {
	return strings.ToLower(strings.TrimSpace(m.cfg.Name)) + "_query"
}

func (m *MCPTool) Description() string {
	return fmt.Sprintf("MCP tool for %s server", m.cfg.Name)
}

func (m *MCPTool) Execute(params map[string]string) (*models.ToolResult, error) {
	query, _ := params["query"]
	if query == "" {
		query, _ = params["input"]
	}
	if query == "" {
		return Failure(m.Name(), ErrCodeInvalidParams, "query or input parameter is required", ""), nil
	}

	var output string
	var err error

	if strings.TrimSpace(m.cfg.URL) != "" {
		output, err = m.callHTTP(query)
	} else if strings.TrimSpace(m.cfg.Command) != "" {
		output, err = m.callStdio(query)
	} else {
		return Failure(m.Name(), ErrCodeInvalidParams, "MCP server has neither URL nor Command configured", ""), nil
	}

	if err != nil {
		return Failure(m.Name(), ErrCodeExecution, err.Error(), ""), nil
	}

	return &models.ToolResult{
		ToolName: m.Name(),
		Success:  true,
		Output:   output,
	}, nil
}

// callHTTP sends a JSON-RPC 2.0 tools/call request to an HTTP MCP server.
func (m *MCPTool) callHTTP(query string) (string, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      m.cfg.Name,
			"arguments": map[string]string{"query": query},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal MCP request: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, m.cfg.URL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("build MCP HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("MCP HTTP call failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read MCP response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("MCP HTTP %d: %s", resp.StatusCode, string(body))
	}

	return extractMCPText(body), nil
}

// callStdio spawns the MCP command, sends a JSON-RPC request on stdin,
// and reads the response from stdout.
func (m *MCPTool) callStdio(query string) (string, error) {
	parts := strings.Fields(m.cfg.Command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty MCP command")
	}

	ctx, cancel := context.WithTimeout(context.Background(), mcpToolTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...) //nolint:gosec
	for k, v := range m.cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      m.cfg.Name,
			"arguments": map[string]string{"query": query},
		},
	}
	data, _ := json.Marshal(payload)

	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("MCP stdio command failed: %w", err)
	}

	return extractMCPText(out), nil
}

// extractMCPText pulls the text content out of a JSON-RPC result envelope.
func extractMCPText(data []byte) string {
	var envelope struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return strings.TrimSpace(string(data))
	}
	if envelope.Error != nil {
		return fmt.Sprintf("[MCP error: %s]", envelope.Error.Message)
	}
	var parts []string
	for _, c := range envelope.Result.Content {
		if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
			parts = append(parts, c.Text)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}
	return strings.TrimSpace(string(data))
}

// RegisterMCPTools reads the MCP config and registers an MCPTool for each server.
func RegisterMCPTools(registry *Registry, mcpCfg config.MCPConfig) {
	for _, srv := range mcpCfg.Servers {
		if strings.TrimSpace(srv.Name) == "" {
			continue
		}
		if strings.TrimSpace(srv.URL) == "" && strings.TrimSpace(srv.Command) == "" {
			continue
		}
		registry.Register(newMCPTool(srv))
	}
}
