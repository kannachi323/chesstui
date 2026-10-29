package lichess

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxErrorBody = 4096
	// Lichess asks clients to pause for a full minute after any 429.
	minRateLimitPause = time.Minute
)

// UserAgent identifies ChessTUI to Lichess. main sets the version at startup.
var UserAgent = "ChessTUI/dev (+https://github.com/kannachi323/chesstui)"

// SetVersion stamps the release version into the User-Agent header.
func SetVersion(version string) {
	UserAgent = "ChessTUI/" + version + " (+https://github.com/kannachi323/chesstui)"
}

// retryDelays are the waits before each retry of an idempotent request that
// failed because of the network or a Lichess 5xx response.
var retryDelays = []time.Duration{time.Second, 3 * time.Second}

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// throttles is shared by every Client for a base URL. The session swaps
// clients when the user signs in or out, and a rate-limit pause or the
// one-request-at-a-time rule must survive that swap.
var (
	throttlesMu sync.Mutex
	throttles   = map[string]*throttle{}
)

type throttle struct {
	gate  sync.Mutex
	mu    sync.Mutex
	until time.Time
}

func throttleFor(baseURL string) *throttle {
	throttlesMu.Lock()
	defer throttlesMu.Unlock()
	t, ok := throttles[baseURL]
	if !ok {
		t = &throttle{}
		throttles[baseURL] = t
	}
	return t
}

func (t *throttle) pausedUntil() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.until
}

func (t *throttle) pause(wait time.Duration) time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	if until := time.Now().Add(wait); until.After(t.until) {
		t.until = until
	}
	return t.until
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("lichess API returned %d: %s", e.StatusCode, e.Message)
}

// RateLimitError reports that Lichess answered 429 and requests are paused.
type RateLimitError struct{ Until time.Time }

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("lichess rate limit; paused until %s", e.Until.Format(time.Kitchen))
}

// Wait returns how long remains before requests resume.
func (e *RateLimitError) Wait() time.Duration { return max(0, time.Until(e.Until)) }

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				MaxIdleConns:          10,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 15 * time.Second,
			},
		},
	}
}

func (c *Client) WithToken(token string) *Client {
	return NewClient(c.baseURL, token)
}

// HasToken reports whether requests are authenticated.
func (c *Client) HasToken() bool { return c.token != "" }

// do sends one request through the shared throttle. Idempotent GETs are
// retried on network errors and 5xx responses; nothing is retried after a 429.
func (c *Client) do(ctx context.Context, method, path string, form url.Values, accept string) (*http.Response, error) {
	t := throttleFor(c.baseURL)
	t.gate.Lock()
	defer t.gate.Unlock()

	for attempt := 0; ; attempt++ {
		if until := t.pausedUntil(); time.Now().Before(until) {
			return nil, &RateLimitError{Until: until}
		}
		var body io.Reader
		if form != nil {
			body = strings.NewReader(form.Encode())
		}
		req, err := c.newRequest(ctx, method, path, body)
		if err != nil {
			return nil, err
		}
		if form != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		req.Header.Set("Accept", accept)

		resp, err := c.http.Do(req)
		retryable := false
		switch {
		case err != nil:
			err = fmt.Errorf("request lichess: %w", err)
			retryable = ctx.Err() == nil
		case resp.StatusCode == http.StatusTooManyRequests:
			resp.Body.Close()
			return nil, &RateLimitError{Until: t.pause(retryAfter(resp))}
		case resp.StatusCode >= 500:
			err = decodeAPIError(resp)
			resp.Body.Close()
			retryable = true
		case resp.StatusCode < 200 || resp.StatusCode >= 300:
			err = decodeAPIError(resp)
			resp.Body.Close()
			return nil, err
		default:
			return resp, nil
		}
		if !retryable || method != http.MethodGet || attempt >= len(retryDelays) {
			return nil, err
		}
		select {
		case <-time.After(retryDelays[attempt]):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Client) doJSON(ctx context.Context, method, path string, form url.Values, out any) error {
	resp, err := c.do(ctx, method, path, form, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode lichess response: %w", err)
	}
	return nil
}

func (c *Client) doText(ctx context.Context, method, path, accept string) (string, error) {
	resp, err := c.do(ctx, method, path, nil, accept)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read lichess response: %w", err)
	}
	return string(data), nil
}

func (c *Client) streamNDJSON(ctx context.Context, path string, handle func(json.RawMessage) error) error {
	t := throttleFor(c.baseURL)
	if until := t.pausedUntil(); time.Now().Before(until) {
		return &RateLimitError{Until: until}
	}
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/x-ndjson")
	client := *c.http
	client.Timeout = 0
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("open lichess stream: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return &RateLimitError{Until: t.pause(retryAfter(resp))}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		payload := append(json.RawMessage(nil), line...)
		if err := handle(payload); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("read lichess stream: %w", err)
	}
	return ctx.Err()
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create lichess request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

func decodeAPIError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	message := strings.TrimSpace(string(data))
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &payload) == nil && payload.Error != "" {
		message = payload.Error
	}
	if message == "" || strings.HasPrefix(message, "<") {
		message = http.StatusText(resp.StatusCode)
	}
	return &APIError{StatusCode: resp.StatusCode, Message: message}
}

func retryAfter(resp *http.Response) time.Duration {
	wait := minRateLimitPause
	if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
		wait = max(wait, time.Duration(seconds)*time.Second)
	}
	return wait
}

// ErrStreamDone lets a stream handler end Reconnect on purpose, for example
// once a game is over.
var ErrStreamDone = errors.New("stream finished")

// Retryable reports whether a failed stream is worth reopening. Auth and
// not-found errors will fail the same way again.
func Retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, ErrStreamDone) {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500
	}
	return true
}
