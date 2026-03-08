package cmd

import (
	"strings"
	"testing"

	"github.com/furkanbeydemir/orch/internal/config"
)

func TestProviderAndModelCommands(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}
	if err := config.Save(repoRoot, config.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := runProviderSet(nil, []string{"openai"}); err != nil {
		t.Fatalf("provider set: %v", err)
	}

	if err := runModelSet(nil, []string{"coder", "gpt-5.3-codex"}); err != nil {
		t.Fatalf("model set: %v", err)
	}

	if err := runProviderShow(nil, nil); err != nil {
		t.Fatalf("provider show: %v", err)
	}
	if err := runModelShow(nil, nil); err != nil {
		t.Fatalf("model show: %v", err)
	}
}

func TestDoctorFailsWithoutAPIKey(t *testing.T) {
	repoRoot := t.TempDir()
	t.Chdir(repoRoot)

	if err := config.EnsureOrchDir(repoRoot); err != nil {
		t.Fatalf("ensure orch dir: %v", err)
	}
	if err := config.Save(repoRoot, config.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	t.Setenv("OPENAI_API_KEY", "")
	err := runDoctor(nil, nil)
	if err == nil {
		t.Fatalf("expected doctor failure without API key")
	}
	if !strings.Contains(err.Error(), "doctor failed") {
		t.Fatalf("unexpected doctor error: %v", err)
	}
}
