package engine

import (
	"context"
	"fmt"

	"github.com/corca-ai/specdown/internal/specdown/adapterhost"
	"github.com/corca-ai/specdown/internal/specdown/config"
)

type sessionManager struct {
	ctx      context.Context
	host     adapterhost.Host
	sessions map[string]*adapterhost.Session
}

type sessionProvider interface {
	For(config.AdapterConfig) (*adapterhost.Session, error)
}

type cleanupSessionProvider struct {
	primary  *sessionManager
	fallback *sessionManager
}

func (p cleanupSessionProvider) For(adapter config.AdapterConfig) (*adapterhost.Session, error) {
	if session, ok := p.primary.sessions[adapter.Name]; ok && session.Usable() {
		return session, nil
	}
	return p.fallback.For(adapter)
}

func newSessionManager(ctx context.Context, host adapterhost.Host) *sessionManager {
	return &sessionManager{
		ctx:      ctx,
		host:     host,
		sessions: make(map[string]*adapterhost.Session),
	}
}

// For returns an existing session for the adapter or starts a new one.
func (m *sessionManager) For(adapter config.AdapterConfig) (*adapterhost.Session, error) {
	if session, ok := m.sessions[adapter.Name]; ok {
		return session, nil
	}
	var session *adapterhost.Session
	var err error
	switch {
	case adapter.BuiltinShell:
		session, err = m.host.StartBuiltinShellSession(adapter)
	case adapter.BuiltinJQ:
		session, err = m.host.StartBuiltinJQSession(adapter)
	default:
		session, err = m.host.StartSessionContext(m.ctx, adapter)
	}
	if err != nil {
		return nil, err
	}
	m.sessions[adapter.Name] = session
	return session, nil
}

// CloseAll closes all open sessions and returns the first error encountered.
func (m *sessionManager) CloseAll() error {
	var firstErr error
	for name, session := range m.sessions {
		if session == nil {
			continue
		}
		if err := session.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close adapter session %q: %w", name, err)
		}
		delete(m.sessions, name)
	}
	return firstErr
}
