package tools

import "testing"

func TestPolicyPlanModeBlocksDestructiveTools(t *testing.T) {
	policy := Policy{Mode: ModePlan}

	decision := policy.Decide("write_file", map[string]string{"path": "a.txt"})
	if decision.allowed {
		t.Fatalf("expected write_file to be blocked in plan mode")
	}

	decision = policy.Decide("apply_patch", map[string]string{"patch": "diff --git a/a b/a"})
	if decision.allowed {
		t.Fatalf("expected apply_patch to be blocked in plan mode")
	}

	decision = policy.Decide("run_command", map[string]string{"command": "rm -rf /tmp/x"})
	if decision.allowed {
		t.Fatalf("expected run_command to be blocked in plan mode")
	}
}

func TestPolicyRequiresApprovalForDestructiveTools(t *testing.T) {
	policy := Policy{Mode: ModeRun, RequireDestructiveApproval: true}

	decision := policy.Decide("write_file", map[string]string{"path": "a.txt"})
	if decision.allowed {
		t.Fatalf("expected write_file to require approval")
	}

	decision = policy.Decide("write_file", map[string]string{"path": "a.txt", "approved": "true"})
	if !decision.allowed {
		t.Fatalf("expected write_file to be allowed with approval")
	}
}
