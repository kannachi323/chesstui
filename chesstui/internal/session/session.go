package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/kannachi323/chesstui/internal/auth"
	"github.com/kannachi323/chesstui/internal/config"
	"github.com/kannachi323/chesstui/internal/engine"
	"github.com/kannachi323/chesstui/internal/lichess"
	"github.com/kannachi323/chesstui/internal/storage"
)

// Session owns long-lived services shared by otherwise independent UI models.
type Session struct {
	Config     config.Config
	Paths      config.Paths
	Store      *storage.Store
	TokenStore auth.TokenStore

	mu      sync.RWMutex
	client  *lichess.Client
	account *lichess.Account

	engineMu   sync.Mutex
	engine     *engine.Engine
	engineOpts engine.Options
}

// Engine returns the shared UCI engine, starting over when the engine
// settings changed. It fails with engine.ErrNotFound when none is installed.
func (s *Session) Engine() (*engine.Engine, error) {
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	opts := engine.Options{Path: s.Config.EnginePath, Threads: s.Config.EngineThreads, HashMB: s.Config.EngineHashMB}
	if s.engine != nil && opts == s.engineOpts {
		return s.engine, nil
	}
	if s.engine != nil {
		go s.engine.Close()
		s.engine = nil
	}
	created, err := engine.New(opts)
	if err != nil {
		return nil, err
	}
	s.engine, s.engineOpts = created, opts
	return created, nil
}

// Close stops background processes such as the engine.
func (s *Session) Close() {
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	if s.engine != nil {
		s.engine.Close()
		s.engine = nil
	}
}

func New(cfg config.Config, paths config.Paths, store *storage.Store, tokens auth.TokenStore) *Session {
	token, _ := tokens.Load()
	return &Session{
		Config: cfg, Paths: paths, Store: store, TokenStore: tokens,
		client: lichess.NewClient(cfg.LichessURL, token),
	}
}

func (s *Session) Client() *lichess.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

func (s *Session) Account() (lichess.Account, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.account == nil {
		return lichess.Account{}, false
	}
	return *s.account, true
}

func (s *Session) Connect(ctx context.Context, token string) (lichess.Account, error) {
	client := lichess.NewClient(s.Config.LichessURL, token)
	account, err := client.Account(ctx)
	if err != nil {
		return lichess.Account{}, err
	}
	if err := s.TokenStore.Save(token); err != nil {
		return lichess.Account{}, err
	}
	s.mu.Lock()
	s.client = client
	s.account = &account
	s.mu.Unlock()
	return account, nil
}

func (s *Session) RefreshAccount(ctx context.Context) (lichess.Account, error) {
	if !s.HasToken() {
		return lichess.Account{}, auth.ErrTokenNotFound
	}
	account, err := s.Client().Account(ctx)
	if err != nil {
		return lichess.Account{}, err
	}
	s.mu.Lock()
	s.account = &account
	s.mu.Unlock()
	return account, nil
}

// HasToken reports whether a Lichess token is loaded, so screens can skip
// authenticated requests that would only return 401.
func (s *Session) HasToken() bool { return s.Client().HasToken() }

// Disconnect revokes the token on Lichess (best effort, so it works offline)
// and removes it from local storage.
func (s *Session) Disconnect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.Client().RevokeToken(ctx); err != nil {
		slog.Warn("revoke Lichess token", "error", err)
	}
	if err := s.TokenStore.Delete(); err != nil {
		return err
	}
	s.mu.Lock()
	s.client = lichess.NewClient(s.Config.LichessURL, "")
	s.account = nil
	s.mu.Unlock()
	return nil
}
