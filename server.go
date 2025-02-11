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

		listener     net.Listener
		sessions     map[*Session]struct{}
		sessionsMu   sync.Mutex
		inShutdown   atomic.Bool
		sessionsDone chan struct{}
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
	// TODO: don't allow to add more listeners
	// or add support for multiple listeners
	s.listener = l
	for {
		conn, err := l.Accept()
		if s.shuttingDown() {
			return ErrServerClosed
		}
		if err != nil {
			return err
		}
		log.Printf("New connection from: %v", conn.RemoteAddr())
		session, err := NewSession(conn, s.mboxProvider, s.authorizer)
		if err != nil {
			conn.Close()
			continue
		}
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
// active connections. Shutdown works by first closing open
// listener and then waiting indefinitely for connections to return
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
	s.inShutdown.Store(true)

	var lnerr error
	if s.listener == nil {
		lnerr = s.listener.Close()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.sessionsDone:
		break
	}

	return lnerr
}

// Close immediately closes active net.Listener and any
// connections. For a graceful shutdown, use [Server.Shutdown].
//
// Close returns any error returned from closing the [Server]'s
// underlying Listener.
func (s *Server) Close() error {
	s.inShutdown.Store(true)

	var lnerr error
	if s.listener == nil {
		lnerr = s.listener.Close()
	}

	for session := range s.sessions {
		session.conn.Close()
	}

	return lnerr
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
