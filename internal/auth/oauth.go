package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	clientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	authURL      = "https://auth.openai.com/oauth/authorize"
	tokenURL     = "https://auth.openai.com/oauth/token"
	redirectURI  = "http://localhost:1455/auth/callback"
	callbackHost = "localhost:1455"
)

// RunOAuthFlow starts a local server on port 1455, prints an authorization URL,
// waits for the OAuth callback, exchanges the code for a token, and returns it.
func RunOAuthFlow() (string, error) {
	// Generate state and PKCE challange
	state := generateRandomString(32)
	verifier := generateRandomString(64)
	challenge := generateCodeChallenge(verifier)

	// Build Auth URL
	params := url.Values{}
	params.Add("response_type", "code")
	params.Add("client_id", clientID)
	params.Add("redirect_uri", redirectURI)
	params.Add("scope", "openid profile email offline_access")
	params.Add("state", state)
	params.Add("code_challenge", challenge)
	params.Add("code_challenge_method", "S256")
	params.Add("id_token_add_organizations", "true")
	params.Add("codex_cli_simplified_flow", "true")

	// Usually we provide originator or other params
	params.Add("originator", "orch")

	loginURL := fmt.Sprintf("%s?%s", authURL, params.Encode())

	// Start local server to receive callback
	addr := callbackHost
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Channels to communicate state
	codeChan := make(chan string)
	errChan := make(chan error)

	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		if errDesc := query.Get("error_description"); errDesc != "" {
			errChan <- fmt.Errorf("auth error: %s", errDesc)
			fmt.Fprintf(w, "Auth error: %s. You can close this window.", errDesc)
			return
		}

		if returnedState := query.Get("state"); returnedState != state {
			errChan <- fmt.Errorf("state mismatch: expected %s, got %s", state, returnedState)
			fmt.Fprintln(w, "State mismatch error. You can close this window.")
			return
		}

		code := query.Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code returned")
			fmt.Fprintln(w, "No code provided. You can close this window.")
			return
		}

		// Success
		fmt.Fprintln(w, "Login successful! You can close this window and return to Orch.")
		codeChan <- code
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("could not start local server on %s: %w", addr, err)
		}
	}()

	fmt.Println("\nLogin to ChatGPT Plus/Pro/Codex Subscription")
	fmt.Printf("\n%s\n\n", loginURL)
	fmt.Println("Ctrl+click to open")
	fmt.Println("\nA browser window should open. Complete login to finish.")
	fmt.Println("\nWaiting for browser callback... (Press Ctrl+C to cancel)")

	var code string
	select {
	case c := <-codeChan:
		code = c
	case err := <-errChan:
		srv.Shutdown(context.Background())
		return "", err
	}

	// Exchange code for token
	token, err := exchangeCodeForToken(code, verifier)
	
	// Shut down server
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	if err != nil {
		return "", fmt.Errorf("failed to exchange code for token: %w", err)
	}

	return token, nil
}

func exchangeCodeForToken(code, verifier string) (string, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", clientID)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("code_verifier", verifier)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if result.AccessToken == "" {
		return "", fmt.Errorf("no access token in response: %s", string(body))
	}

	return result.AccessToken, nil
}

func generateRandomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:length]
}

func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
