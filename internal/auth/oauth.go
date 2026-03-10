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
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	clientID                    = "app_EMoamEEZ73f0CkXaXp7hrann"
	authURL                     = "https://auth.openai.com/oauth/authorize"
	tokenURL                    = "https://auth.openai.com/oauth/token"
	deviceUserCodeURL           = "https://auth.openai.com/api/accounts/deviceauth/usercode"
	deviceTokenURL              = "https://auth.openai.com/api/accounts/deviceauth/token"
	deviceRedirectURI           = "https://auth.openai.com/deviceauth/callback"
	redirectURI                 = "http://localhost:1455/auth/callback"
	callbackHost                = "localhost:1455"
	defaultTokenExpirySeconds   = 3600
	oauthCallbackWaitTimeout    = 5 * time.Minute
	headlessPollingSafetyMargin = 3 * time.Second
)

type OAuthResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	AccountID    string
	Email        string
}

// RunOAuthFlow executes OpenAI OAuth login.
// Supported flows: auto, browser, headless.
func RunOAuthFlow(flow string) (OAuthResult, error) {
	normalized := strings.ToLower(strings.TrimSpace(flow))
	if normalized == "" {
		normalized = "auto"
	}

	switch normalized {
	case "browser":
		return runBrowserOAuthFlow()
	case "headless":
		return runHeadlessOAuthFlow()
	case "auto":
		browser, browserErr := runBrowserOAuthFlow()
		if browserErr == nil {
			return browser, nil
		}
		fmt.Printf("\nBrowser login failed: %v\n", browserErr)
		fmt.Println("Falling back to headless device login...")
		headless, headlessErr := runHeadlessOAuthFlow()
		if headlessErr != nil {
			return OAuthResult{}, fmt.Errorf("browser flow failed: %v; headless flow failed: %w", browserErr, headlessErr)
		}
		return headless, nil
	default:
		return OAuthResult{}, fmt.Errorf("unsupported oauth flow: %s", flow)
	}
}

func RefreshOAuthToken(refreshToken string) (OAuthResult, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return OAuthResult{}, fmt.Errorf("refresh token is required")
	}

	tokens, err := requestToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	})
	if err != nil {
		return OAuthResult{}, err
	}

	return parseOAuthResult(tokens)
}

func runBrowserOAuthFlow() (OAuthResult, error) {
	// Generate state and PKCE challenge
	state := generateRandomString(32)
	verifier := generateRandomString(64)
	challenge := generateCodeChallenge(verifier)

	// Build auth URL.
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

	// Identify CLI origin.
	params.Add("originator", "orch")

	loginURL := fmt.Sprintf("%s?%s", authURL, params.Encode())

	// Start local server to receive callback.
	addr := callbackHost
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		if errDesc := query.Get("error_description"); errDesc != "" {
			select {
			case errChan <- fmt.Errorf("auth error: %s", errDesc):
			default:
			}
			fmt.Fprintf(w, "Auth error: %s. You can close this window.", errDesc)
			return
		}

		if returnedState := query.Get("state"); returnedState != state {
			select {
			case errChan <- fmt.Errorf("state mismatch: expected %s, got %s", state, returnedState):
			default:
			}
			fmt.Fprintln(w, "State mismatch error. You can close this window.")
			return
		}

		code := query.Get("code")
		if code == "" {
			select {
			case errChan <- fmt.Errorf("no code returned"):
			default:
			}
			fmt.Fprintln(w, "No code provided. You can close this window.")
			return
		}

		// Success.
		fmt.Fprintln(w, "Login successful! You can close this window and return to Orch.")
		select {
		case codeChan <- code:
		default:
		}
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			select {
			case errChan <- fmt.Errorf("could not start local server on %s: %w", addr, err):
			default:
			}
		}
	}()

	fmt.Println("\nLogin to ChatGPT Plus/Pro/Codex Subscription")
	fmt.Printf("\n%s\n\n", loginURL)
	if err := openBrowser(loginURL); err != nil {
		fmt.Printf("Could not open browser automatically: %v\n", err)
	}
	fmt.Println("Ctrl+click to open if needed")
	fmt.Println("\nA browser window should open. Complete login to finish.")
	fmt.Println("\nWaiting for browser callback... (Press Ctrl+C to cancel)")

	var code string
	select {
	case c := <-codeChan:
		code = c
	case err := <-errChan:
		_ = srv.Shutdown(context.Background())
		return OAuthResult{}, err
	case <-time.After(oauthCallbackWaitTimeout):
		_ = srv.Shutdown(context.Background())
		return OAuthResult{}, fmt.Errorf("oauth callback timed out after %s", oauthCallbackWaitTimeout)
	}

	// Exchange code for token.
	tokens, err := exchangeCodeForToken(code, verifier, redirectURI)

	// Shut down server.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	if err != nil {
		return OAuthResult{}, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	return parseOAuthResult(tokens)
}

func runHeadlessOAuthFlow() (OAuthResult, error) {
	deviceAuth, err := requestDeviceAuth()
	if err != nil {
		return OAuthResult{}, err
	}

	fmt.Println("\nHeadless login to ChatGPT Plus/Pro/Codex Subscription")
	fmt.Println("Open: https://auth.openai.com/codex/device")
	fmt.Printf("Enter code: %s\n", deviceAuth.UserCode)
	fmt.Println("Waiting for authorization... (Press Ctrl+C to cancel)")

	code, verifier, err := pollForDeviceAuthorization(deviceAuth)
	if err != nil {
		return OAuthResult{}, err
	}

	tokens, err := exchangeCodeForToken(code, verifier, deviceRedirectURI)
	if err != nil {
		return OAuthResult{}, fmt.Errorf("failed to exchange headless authorization code: %w", err)
	}

	return parseOAuthResult(tokens)
}

func exchangeCodeForToken(code, verifier, redirect string) (*tokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", clientID)
	data.Set("code", code)
	data.Set("redirect_uri", redirect)
	data.Set("code_verifier", verifier)
	return requestToken(data)
}

func requestToken(data url.Values) (*tokenResponse, error) {

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result tokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	if result.AccessToken == "" {
		return nil, fmt.Errorf("no access token in response: %s", string(body))
	}

	return &result, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func parseOAuthResult(tokens *tokenResponse) (OAuthResult, error) {
	if tokens == nil {
		return OAuthResult{}, fmt.Errorf("empty oauth token response")
	}
	if strings.TrimSpace(tokens.AccessToken) == "" {
		return OAuthResult{}, fmt.Errorf("oauth access token is empty")
	}

	expiresIn := tokens.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = defaultTokenExpirySeconds
	}

	result := OAuthResult{
		AccessToken:  strings.TrimSpace(tokens.AccessToken),
		RefreshToken: strings.TrimSpace(tokens.RefreshToken),
		ExpiresAt:    time.Now().UTC().Add(time.Duration(expiresIn) * time.Second),
	}

	if idClaims := decodeJWTClaims(tokens.IDToken); idClaims != nil {
		result.AccountID = extractAccountID(idClaims)
		result.Email = extractEmail(idClaims)
	}
	if result.AccountID == "" || result.Email == "" {
		if accessClaims := decodeJWTClaims(tokens.AccessToken); accessClaims != nil {
			if result.AccountID == "" {
				result.AccountID = extractAccountID(accessClaims)
			}
			if result.Email == "" {
				result.Email = extractEmail(accessClaims)
			}
		}
	}

	return result, nil
}

type deviceAuthResponse struct {
	DeviceAuthID string
	UserCode     string
	Interval     time.Duration
}

func requestDeviceAuth() (*deviceAuthResponse, error) {
	body, _ := json.Marshal(map[string]string{"client_id": clientID})
	req, err := http.NewRequest(http.MethodPost, deviceUserCodeURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("device auth init failed (status %d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var payload struct {
		DeviceAuthID string `json:"device_auth_id"`
		UserCode     string `json:"user_code"`
		Interval     string `json:"interval"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse device auth response: %w", err)
	}
	if strings.TrimSpace(payload.DeviceAuthID) == "" || strings.TrimSpace(payload.UserCode) == "" {
		return nil, fmt.Errorf("device auth response missing required fields")
	}

	seconds := 5
	if parsed, convErr := strconv.Atoi(strings.TrimSpace(payload.Interval)); convErr == nil && parsed > 0 {
		seconds = parsed
	}

	return &deviceAuthResponse{
		DeviceAuthID: strings.TrimSpace(payload.DeviceAuthID),
		UserCode:     strings.TrimSpace(payload.UserCode),
		Interval:     time.Duration(seconds) * time.Second,
	}, nil
}

func pollForDeviceAuthorization(device *deviceAuthResponse) (string, string, error) {
	if device == nil {
		return "", "", fmt.Errorf("device auth context is nil")
	}

	client := &http.Client{Timeout: 20 * time.Second}
	ticker := time.NewTicker(device.Interval + headlessPollingSafetyMargin)
	defer ticker.Stop()
	timeout := time.After(10 * time.Minute)

	bodyPayload, _ := json.Marshal(map[string]string{
		"device_auth_id": device.DeviceAuthID,
		"user_code":      device.UserCode,
	})

	for {
		select {
		case <-timeout:
			return "", "", fmt.Errorf("headless oauth timed out")
		case <-ticker.C:
			req, err := http.NewRequest(http.MethodPost, deviceTokenURL, strings.NewReader(string(bodyPayload)))
			if err != nil {
				return "", "", err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return "", "", err
			}

			data, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				return "", "", readErr
			}

			if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
				continue
			}

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return "", "", fmt.Errorf("device auth polling failed (status %d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
			}

			var payload struct {
				AuthorizationCode string `json:"authorization_code"`
				CodeVerifier      string `json:"code_verifier"`
			}
			if err := json.Unmarshal(data, &payload); err != nil {
				return "", "", fmt.Errorf("failed to parse device auth poll response: %w", err)
			}

			code := strings.TrimSpace(payload.AuthorizationCode)
			verifier := strings.TrimSpace(payload.CodeVerifier)
			if code == "" || verifier == "" {
				return "", "", fmt.Errorf("device auth poll response missing authorization code")
			}
			return code, verifier, nil
		}
	}
}

func openBrowser(targetURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	case "darwin":
		cmd = exec.Command("open", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	return cmd.Start()
}

func generateRandomString(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:length]
}

func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func decodeJWTClaims(token string) map[string]any {
	token = strings.TrimSpace(token)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}

	payload := parts[1]
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(payload)
		if err != nil {
			return nil
		}
	}

	claims := map[string]any{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil
	}
	return claims
}

func extractAccountID(claims map[string]any) string {
	if claims == nil {
		return ""
	}

	if raw, ok := claims["chatgpt_account_id"].(string); ok {
		return strings.TrimSpace(raw)
	}

	if nested, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
		if raw, ok := nested["chatgpt_account_id"].(string); ok {
			return strings.TrimSpace(raw)
		}
	}

	if organizations, ok := claims["organizations"].([]any); ok && len(organizations) > 0 {
		if org, ok := organizations[0].(map[string]any); ok {
			if raw, ok := org["id"].(string); ok {
				return strings.TrimSpace(raw)
			}
		}
	}

	return ""
}

func extractEmail(claims map[string]any) string {
	if claims == nil {
		return ""
	}
	if raw, ok := claims["email"].(string); ok {
		return strings.TrimSpace(raw)
	}
	return ""
}
