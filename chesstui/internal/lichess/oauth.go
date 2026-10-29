package lichess

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const oauthClientID = "chesstui"

var DefaultScopes = []string{
	"board:play",
	"challenge:read",
	"challenge:write",
	"study:read",
	"puzzle:read",
	"preference:read",
}

const callbackPage = `<!doctype html><meta charset="utf-8"><title>ChessTUI</title>
<style>body{font:16px system-ui;display:grid;place-items:center;height:100vh;margin:0;background:#101114;color:#e9e6df}</style>
<p>%s</p>`

type oauthCallback struct {
	code string
	err  error
}

// Login is one in-flight PKCE authorization. The caller shows or opens URL,
// then calls Wait. Close releases the loopback listener.
type Login struct {
	URL string

	client      *Client
	verifier    string
	redirectURI string
	server      *http.Server
	result      chan oauthCallback
	closeOnce   sync.Once
}

// BeginLogin starts Lichess's public-client Authorization Code flow with PKCE
// (S256) on a 127.0.0.1 loopback redirect. No client secret is involved.
func (c *Client) BeginLogin(scopes []string) (*Login, error) {
	verifier, err := randomURLToken(64)
	if err != nil {
		return nil, err
	}
	state, err := randomURLToken(32)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start OAuth callback listener: %w", err)
	}
	login := &Login{
		client:      NewClient(c.baseURL, ""),
		verifier:    verifier,
		redirectURI: "http://" + listener.Addr().String() + "/oauth/callback",
		result:      make(chan oauthCallback, 1),
	}
	var once sync.Once
	deliver := func(value oauthCallback) { once.Do(func() { login.result <- value }) }
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// A request without our state did not come from this login. Ignore
		// it instead of failing, so a stray local request cannot cancel it.
		if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(state)) != 1 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, callbackPage, "This sign-in link is not the one ChessTUI is waiting for.")
			return
		}
		if code := query.Get("error"); code != "" {
			deliver(oauthCallback{err: fmt.Errorf("Lichess sign-in was not completed (%s)", code)})
			fmt.Fprintf(w, callbackPage, "Sign-in cancelled. You can close this tab.")
			return
		}
		code := query.Get("code")
		if code == "" {
			deliver(oauthCallback{err: errors.New("OAuth callback contained no authorization code")})
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, callbackPage, "The sign-in response was incomplete.")
			return
		}
		deliver(oauthCallback{code: code})
		fmt.Fprintf(w, callbackPage, "Connected. You can return to the terminal.")
	})
	login.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second}
	go func() { _ = login.server.Serve(listener) }()

	challenge := sha256.Sum256([]byte(verifier))
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {oauthClientID},
		"redirect_uri":          {login.redirectURI},
		"code_challenge_method": {"S256"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"state":                 {state},
		"scope":                 {strings.Join(scopes, " ")},
	}
	login.URL = c.baseURL + "/oauth?" + query.Encode()
	return login, nil
}

// Wait blocks until the browser returns to the loopback listener, then
// exchanges the code for an access token.
func (l *Login) Wait(ctx context.Context) (string, error) {
	defer l.Close()
	var callback oauthCallback
	select {
	case callback = <-l.result:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if callback.err != nil {
		return "", callback.err
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {callback.code},
		"code_verifier": {l.verifier},
		"redirect_uri":  {l.redirectURI},
		"client_id":     {oauthClientID},
	}
	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := l.client.doJSON(ctx, http.MethodPost, "/api/token", form, &tokenResponse); err != nil {
		return "", fmt.Errorf("exchange OAuth code: %w", err)
	}
	if tokenResponse.AccessToken == "" {
		return "", errors.New("Lichess returned an empty access token")
	}
	return tokenResponse.AccessToken, nil
}

// Close stops the loopback listener. It is safe to call more than once.
func (l *Login) Close() {
	l.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = l.server.Shutdown(ctx)
	})
}

// RevokeToken deletes the client's access token on Lichess.
func (c *Client) RevokeToken(ctx context.Context) error {
	if c.token == "" {
		return nil
	}
	return c.doJSON(ctx, http.MethodDelete, "/api/token", nil, nil)
}

func randomURLToken(bytes int) (string, error) {
	data := make([]byte, bytes)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate OAuth secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
