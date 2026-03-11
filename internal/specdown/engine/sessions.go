package engine

import (
	"fmt"

	"github.com/corca-ai/specdown/internal/specdown/adapterhost"
	"github.com/corca-ai/specdown/internal/specdown/config"
)

type sessionManager struct {
	host     adapterhost.Host
	sessions map[string]*adapterhost.Session
}

func newSessionManager(host adapterhost.Host) *sessionManager {
	return &sessionManager{
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
	if adapter.BuiltinShell {
		session, err = m.host.StartBuiltinShellSession(adapter)
	} else {
		session, err = m.host.StartSession(adapter)
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
