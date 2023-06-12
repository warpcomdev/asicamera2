package jpeg

import (
	"context"
	"sync"

	"github.com/warpcomdev/asicamera2/internal/driver/servicelog"
)

type errString string

// Error implements error
func (err errString) Error() string {
	return string(err)
}

var ErrManagerCancelled errString = "manager has been cancelled"

// ResumableSource is an extended type of Source
// that can be started and stopped.
// It is guaranteed that `Next` will never be called
// before `Start` or after `Stop`.
type ResumableSource interface {
	Source
	Start(servicelog.Logger) error
	Stop()
}

// SessionManager manages several sessions sharing a single ResumableSource.
// It starts and stops the source on demand.
type SessionManager struct {
	// Construction-time parameters
	source   ResumableSource
	pipeline *Pipeline
	// Current session, if any
	session    *Session
	cancelFunc func()
	// Cancellation management
	cancelled bool
	users     int
	cond      sync.Cond
	mutex     sync.Mutex
}

// Manage builds a manager for the given source
func (pipeline *Pipeline) Manage(source ResumableSource) *SessionManager {
	sm := &SessionManager{
		source:   source,
		pipeline: pipeline,
	}
	sm.cond.L = &sm.mutex
	return sm
}

// Acquire a Session for the source, making sure it is Started
func (m *SessionManager) Acquire(logger servicelog.Logger) (*Session, error) {
	m.cond.L.Lock()
	defer m.cond.L.Unlock()
	if m.cancelled {
		return nil, ErrManagerCancelled
	}
	if m.session == nil {
		if err := m.source.Start(logger); err != nil {
			return nil, err
		}
		ctx, cancelFunc := context.WithCancel(context.Background())
		m.cancelFunc = cancelFunc
		m.session = m.pipeline.session(ctx, logger, m.source)
	}
	m.users += 1
	return m.session, nil
}

// Release the session, stop the source if no sessions left
func (m *SessionManager) Done() {
	m.cond.L.Lock()
	defer m.cond.L.Unlock()
	if m.users -= 1; m.users <= 0 {
		m.cancelFunc()
		m.session.Join()
		m.source.Stop()
		m.session = nil
		m.cond.Broadcast()
	}
}

// Join marks the manager as done and waits until there are no more sessions
func (m *SessionManager) Join() {
	m.cond.L.Lock()
	defer m.cond.L.Unlock()
	m.cancelled = true
	for m.users > 0 {
		m.cond.Wait()
	}
}
