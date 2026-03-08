package providers

import "fmt"

const (
	ErrAuthError        = "provider_auth_error"
	ErrRateLimited      = "provider_rate_limited"
	ErrTimeout          = "provider_timeout"
	ErrModelUnavailable = "provider_model_unavailable"
	ErrTransient        = "provider_transient_error"
	ErrInvalidResponse  = "provider_invalid_response"
)

type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return "provider error"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
