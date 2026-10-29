package lichess

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Describe turns client errors into short, human sentences for the UI.
func Describe(err error) string {
	if err == nil {
		return ""
	}
	var rate *RateLimitError
	var apiErr *APIError
	var netErr net.Error
	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &rate):
		return fmt.Sprintf("Lichess asked for a pause · retry in %s", rate.Wait().Round(time.Second))
	case errors.Is(err, context.Canceled):
		return "Cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "Lichess took too long to answer · try again"
	case errors.As(err, &apiErr):
		switch {
		case apiErr.StatusCode == http.StatusUnauthorized:
			return "Lichess sign-in expired · reconnect from Play"
		case apiErr.StatusCode == http.StatusForbidden:
			return "Lichess refused that · " + apiErr.Message
		case apiErr.StatusCode == http.StatusNotFound:
			return "Not found on Lichess"
		case apiErr.StatusCode >= 500:
			return "Lichess is having trouble · try again shortly"
		default:
			return apiErr.Message
		}
	case errors.As(err, &dnsErr), errors.As(err, &netErr):
		return "Offline · check your connection"
	}
	return err.Error()
}
