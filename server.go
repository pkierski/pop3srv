package pop3srv

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultConnectionsLimit = 100
)

type (
	// Server is a POP3 server instance.
	Server struct {
		// ConnectionsLimit defines maximum concurrent connections.
		ConnectionsLimit int

		// ConnectionTimeout is the amount of time allowed to read
		// client command.
		//
		// Value equal or less than zero means infinite timeout (default).
		ConnectionTimeout time.Duration

		authorizer   Authorizer
		mboxProvider MailboxProvider

		inShutdown     atomic.Bool
		listeners      map[*net.Listener]struct{}
		listenersMu    sync.Mutex
		listenersGroup sync.WaitGroup
		sessions       map[*Session]struct{}
		sessionsMu     sync.Mutex
		sessionsDone   chan struct{}
	}
)

// ErrServerClosed is returned by the [Server.Serve] and [ListenAndServe],
// methods after a call to [Server.Shutdown] or [Server.Close].
var (
	ErrServerClosed       = errors.New("pop3: server closed")
	ErrTooManyConnections = errors.New("pop3: too many connections")
)

func NewServer(authorizer Authorizer, mboxProvider MailboxProvider) *Server {
	return &Server{
		ConnectionsLimit: DefaultConnectionsLimit,
		authorizer:       authorizer,
		mboxProvider:     mboxProvider,
		listeners:        make(map[*net.Listener]struct{}),
		sessions:         make(map[*Session]struct{}),
		sessionsDone:     make(chan struct{}),
	}
}

// Serve accepts incoming connections on the Listener l.
//
// Serve always returns a non-nil error and closes l.
// After [Server.Shutdown] or [Server.Close], the returned error
// is [ErrServerClosed].
func (s *Server) Serve(l net.Listener) error {
	l = &onceCloseListener{Listener: l}
	defer l.Close()

	// addListener checks if the server is in shutdown state
	if !s.addListener(&l) {
		return ErrServerClosed
	}
	defer s.removeListener(&l)

	for {
		conn, err := l.Accept()
		if s.shuttingDown() {
			return ErrServerClosed
		}
		if err != nil {
			return err
		}
		log.Printf("New connection from: %v on: %v", conn.RemoteAddr(), conn.LocalAddr())
		session := NewSession(conn, s.mboxProvider, s.authorizer)
		session.ConnectionTimeout = s.ConnectionTimeout

		if s.addSession(session) != nil {
			session.writeResponseLine("", err)
			continue
		}

		go func() {
			session.Serve()
			s.deleteSession(session)
			// set singnal if we in shutting down state and the last session is finished
			if s.inShutdown.Load() && !s.hasActiveSessions() {
				close(s.sessionsDone)
			}
			log.Printf("Connection from: %v on: %v closed", conn.RemoteAddr(), conn.LocalAddr())
		}()
	}
}

// ListenAndServe listens on the TCP network address addr and then
// calls Serve to handle requests on incoming connections.
//
// If srv.Addr is blank, ":pop3" is used.
//
// Serve always returns a non-nil error and closes l.
// After [Server.Shutdown] or [Server.Close], the returned error
// is [ErrServerClosed].
func (s *Server) ListenAndServe(addr string) error {
	if addr == "" {
		addr = ":pop3"
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Shutdown gracefully shuts down the server without interrupting any
// active connections. Shutdown works by first closing all open
// listeners and then waiting indefinitely for connections to return
// to idle and then shut down.
// If the provided context expires before the shutdown is complete,
// Shutdown returns the context's error, otherwise it returns any
// error returned from closing the [Server]'s underlying Listener.
//
// When Shutdown is called, [Serve] and [ListenAndServe]
// immediately return [ErrServerClosed]. Make sure the
// program doesn't exit and waits instead for Shutdown to return.
//
// Once Shutdown has been called on a server, it may not be reused;
// future calls to methods such as Serve will return ErrServerClosed.
func (s *Server) Shutdown(ctx context.Context) error {
	if !s.inShutdown.CompareAndSwap(false, true) {
		return ErrServerClosed
	}

	s.listenersMu.Lock()
	lnerr := s.closeListenersLocked()
	s.listenersMu.Unlock()
	s.listenersGroup.Wait()

	s.sessionsMu.Lock()
	if len(s.sessions) == 0 {
		close(s.sessionsDone)
	}
	s.sessionsMu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.sessionsDone:
		break
	}

	s.forceCloseAllSessions()

	return lnerr
}

// Close immediately closes all active net.Listener and any
// connections. For a graceful shutdown, use [Server.Shutdown].
//
// Close returns any error returned from closing the [Server]'s
// underlying Listener.
func (s *Server) Close() error {
	if !s.inShutdown.CompareAndSwap(false, true) {
		return ErrServerClosed
	}

	s.listenersMu.Lock()
	lnerr := s.closeListenersLocked()
	s.listenersMu.Unlock()
	s.listenersGroup.Wait()

	s.forceCloseAllSessions()

	return lnerr
}

func (s *Server) forceCloseAllSessions() {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	for session := range s.sessions {
		session.conn.Close()
	}
}

func (s *Server) shuttingDown() bool {
	return s.inShutdown.Load()
}

func (s *Server) addSession(session *Session) error {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	if len(s.sessions) >= s.ConnectionsLimit {
		return ErrTooManyConnections
	}

	s.sessions[session] = struct{}{}
	return nil
}

func (s *Server) deleteSession(session *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	delete(s.sessions, session)
}

func (s *Server) hasActiveSessions() bool {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	return len(s.sessions) > 0
}

// onceCloseListener wraps a net.Listener, protecting it from
// multiple Close calls.
type onceCloseListener struct {
	net.Listener
	once     sync.Once
	closeErr error
}

func (oc *onceCloseListener) Close() error {
	oc.once.Do(oc.close)
	return oc.closeErr
}

func (oc *onceCloseListener) close() { oc.closeErr = oc.Listener.Close() }

func (s *Server) closeListenersLocked() error {
	var err error
	for ln := range s.listeners {
		if cerr := (*ln).Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

func (s *Server) addListener(ln *net.Listener) bool {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	if s.inShutdown.Load() {
		return false
	}
	s.listeners[ln] = struct{}{}
	s.listenersGroup.Add(1)
	return true
}

func (s *Server) removeListener(ln *net.Listener) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	delete(s.listeners, ln)
	s.listenersGroup.Done()
}
