package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

func TestAccountSessionFailsOverToNextCredential(t *testing.T) {
	repoRoot := t.TempDir()
	if err := Set(repoRoot, "openai", Credential{Type: "oauth", AccessToken: testSessionAccountToken("acc-1"), RefreshToken: "refresh-1", AccountID: "acc-1", Email: "one@example.com"}); err != nil {
		t.Fatalf("set first account: %v", err)
	}
	if err := Set(repoRoot, "openai", Credential{Type: "oauth", AccessToken: testSessionAccountToken("acc-2"), RefreshToken: "refresh-2", AccountID: "acc-2", Email: "two@example.com"}); err != nil {
		t.Fatalf("set second account: %v", err)
	}
	if err := SetActive(repoRoot, "openai", "acc-1"); err != nil {
		t.Fatalf("set active: %v", err)
	}

	session := NewAccountSession(repoRoot, "openai")
	token, err := session.ResolveToken(context.Background())
	if err != nil {
		t.Fatalf("resolve token: %v", err)
	}
	if token != testSessionAccountToken("acc-1") {
		t.Fatalf("expected first token, got %q", token)
	}

	nextToken, switched, err := session.Failover(context.Background(), time.Minute, "rate limited")
	if err != nil {
		t.Fatalf("failover: %v", err)
	}
	if !switched {
		t.Fatalf("expected failover switch")
	}
	if nextToken != testSessionAccountToken("acc-2") {
		t.Fatalf("expected second token, got %q", nextToken)
	}

	active, err := Get(repoRoot, "openai")
	if err != nil {
		t.Fatalf("get active credential: %v", err)
	}
	if active == nil || active.ID != "acc-2" {
		t.Fatalf("expected acc-2 active after failover, got %#v", active)
	}
	credentials, _, err := List(repoRoot, "openai")
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}
	var first *Credential
	for i := range credentials {
		if credentials[i].ID == "acc-1" {
			first = &credentials[i]
			break
		}
	}
	if first == nil || first.CooldownUntil.IsZero() {
		t.Fatalf("expected first credential to have cooldown set, got %#v", first)
	}
}

func testSessionAccountToken(accountID string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := fmt.Sprintf(`{"https://api.openai.com/auth":{"chatgpt_account_id":"%s"}}`, accountID)
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return header + "." + body + ".sig"
}
