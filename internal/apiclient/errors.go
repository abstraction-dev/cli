package apiclient

import (
	"fmt"
	"net/http"
)

// APIError is a non-2xx HTTP response (or a streamed `error` frame, with
// Status 0). Message carries the server's plain-text body.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Status == 0 {
		return e.Message
	}
	return fmt.Sprintf("%s (HTTP %d)", e.Message, e.Status)
}

// IsAuth reports whether the failure is an authentication/authorization error,
// so the caller can prompt the user to re-run login.
func (e *APIError) IsAuth() bool {
	return e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden
}
