package pop3srv

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/textproto"
	"os"
	"strings"
	"time"
)

type (
	// Session represents one POP3 session.
	//
	// It is used internally by [Server] but you can use it
	// for building your own server.
	Session struct {
		// ConnectionTimeout is the amount of time allowed to read
		// client command.
		//
		// Value equal or less than zero means infinite timeout (default).
		ConnectionTimeout time.Duration

		conn            Conn
		authorizer      Authorizer
		mboxProvider    MailboxProvider
		timestampBanner string

		r *bufio.Reader

		state    sessionState
		user     string
		mailbox  Mailbox
		toDelete map[int]struct{}
		msgCount int
	}

	sessionState int
)

const (
	authorizationState sessionState = iota
	transactionState
	updateState
)

func NewSession(c Conn, mboxProvider MailboxProvider, authorizer Authorizer) (*Session, error) {
	s := &Session{
		conn:            c,
		authorizer:      authorizer,
		mboxProvider:    mboxProvider,
		r:               bufio.NewReader(c),
		state:           authorizationState,
		timestampBanner: generateTimestampBanner(),
		toDelete:        make(map[int]struct{}),
	}
	greetings := fmt.Sprintf("+OK POP3 server ready %s\r\n", s.timestampBanner)
	err := s.writeLine(greetings)
	return s, err
}

func generateTimestampBanner() string {
	hostName, err := os.Hostname()
	if err != nil {
		hostName = "localhost"
	}
	return fmt.Sprintf("<%d.%d@%s>", os.Getpid(), time.Now().UnixMicro(), hostName)
}

func (s *Session) readCommand() (cmd command, err error) {
	line, err := s.r.ReadString('\n')
	if err != nil {
		return
	}
	line = strings.TrimRight(line, "\r\n")
	log.Printf("S->C: %v", line)
	cmd.parse(line)
	return
}

// Serve is the main loop which read commands and write reponses.
//
// It returns non-nil error if there is any error on reading or writting
// data with connection. [MailboxProvider] and [Authorizer] errors are
// reported as -ERR response.
func (s *Session) Serve() error {
	for {
		cmd, err := timeoutCall(s.readCommand, 10*time.Second)
		if err != nil {
			return err
		}

		if !cmd.isValidInState(s.state) {
			if errSend := s.writeResponseLine("", ErrInvalidCommand); errSend != nil {
				return errSend
			}
			continue
		}

		switch s.state {
		case authorizationState:
			err = s.handleAuthorizationState(cmd)
		case transactionState:
			err = s.handleTransactionState(cmd)
		case updateState:
			err = s.handleUpdateState(cmd)
		}

		if err != nil {
			return err
		}
	}
}

func (s *Session) handleAuthorizationState(cmd command) error {
	switch cmd.name {
	case userCmd:
		if s.user != "" {
			err := s.writeResponseLine("", ErrUserAlreadySpecified)
			if err != nil {
				return err
			}
			break
		}
		s.user = cmd.args[0]
		err := s.writeResponseLine("send PASS", nil)
		if err != nil {
			return err
		}

	case passCmd:
		if s.user == "" {
			err := s.writeResponseLine("", ErrUserNotSpecified)
			if err != nil {
				return err
			}
			break
		}
		err := s.authorizer.UserPass(s.user, cmd.args[0])
		if err != nil {
			return s.writeResponseLine("", err)
		}
		mailbox, err := s.mboxProvider.Provide(s.user)
		if err == nil {
			s.mailbox = mailbox
			s.state = transactionState // if user and password are correct
			s.msgCount, _, err = s.mailbox.Stat()
		}
		err = s.writeResponseLine("logged in", err)
		if err != nil {
			return err
		}

	case quitCmd:
		_, err := s.conn.Write([]byte("+OK POP3 server signing off\r\n"))
		if err != nil {
			return err
		}
		return s.Close()

	case apopCmd:
		if len(cmd.args) != 2 {
			if errSend := s.writeLine("-ERR invalid arguments\r\n"); errSend != nil {
				return errSend
			}
			break
		}
		user := cmd.args[0]
		err := s.authorizer.Apop(user, s.timestampBanner, cmd.args[1])
		if err != nil {
			return s.writeResponseLine("", err)
		}
		mailbox, err := s.mboxProvider.Provide(user)
		if err == nil {
			s.mailbox = mailbox
			s.state = transactionState // if user and password are correct
			s.msgCount, _, err = s.mailbox.Stat()
		}

		if errSend := s.writeResponseLine("logged in", err); errSend != nil {
			return errSend
		}

	case capaCmd:
		if err := s.handleCAPA(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) handleCAPA() error {
	err := s.writeResponseLine("Capability list follows", nil)
	if err != nil {
		return err
	}
	_, err = s.conn.Write([]byte(
		"USER\r\n" +
			"TOP\r\n" +
			"UIDL\r\n" +
			".\r\n",
	))
	return err
}

func (s *Session) handleTransactionState(cmd command) error {
	switch cmd.name {
	case statCmd:
		n, size, err := s.mailbox.Stat()
		if errSend := s.writeResponseLine(fmt.Sprintf("%d %d", n, size), err); errSend != nil {
			return errSend
		}

	case listCmd:
		if cmd.oneNumArg() {
			n := cmd.numArgs[0]
			if s.isMarketAsDelete(n) {
				if errSend := s.writeResponseLine("", ErrMessageMarkedAsDeleted); errSend != nil {
					return errSend
				}
			}
			size, err := s.mailbox.ListOne(n)
			if errSend := s.writeResponseLine(fmt.Sprintf("%d %d", n+1, size), err); errSend != nil {
				return errSend
			}
			break
		}

		list, err := s.mailbox.List()
		if errSend := s.writeResponseLine(fmt.Sprintf("%d messages in mailbox", len(list)), err); errSend != nil {
			return errSend
		}
		for i, size := range list {
			if errSend := s.writeLine(fmt.Sprintf("%d %d\r\n", i+1, size)); errSend != nil {
				return errSend
			}
		}
		if errSend := s.writeLine(".\r\n"); errSend != nil {
			return errSend
		}

	case retrCmd:
		if !cmd.oneNumArg() {
			err := s.writeLine("-ERR invalid arguments\r\n")
			if err != nil {
				return err
			}
			break
		}

		n := cmd.numArgs[0]
		if s.isMarketAsDelete(n) {
			if errSend := s.writeResponseLine("", ErrMessageMarkedAsDeleted); errSend != nil {
				return errSend
			}
		}

		r, err := s.mailbox.Message(n)
		if errSend := s.writeResponseLine(fmt.Sprintf("message body #%v", n+1), err); errSend != nil {
			return errSend
		}
		if err != nil {
			break
		}

		dotWriter := textproto.NewWriter(bufio.NewWriter(s.conn)).DotWriter()
		_, errCopy := io.Copy(dotWriter, r)
		errCloseR := r.Close()
		errCloseW := dotWriter.Close()
		errComposite := errors.Join(errCopy, errCloseR, errCloseW)
		if errComposite != nil {
			return errComposite
		}

	case deleCmd:
		if !cmd.oneNumArg() {
			if errSend := s.writeLine("-ERR invalid arguments\r\n"); errSend != nil {
				return errSend
			}
			break
		}

		n := cmd.numArgs[0]
		if s.isMarketAsDelete(n) {
			if errSend := s.writeResponseLine("", ErrMessageMarkedAsDeleted); errSend != nil {
				return errSend
			}
		}
		if n > s.msgCount {
			if errSend := s.writeLine("-ERR invalid arguments\r\n"); errSend != nil {
				return errSend
			}
			break
		}

		s.toDelete[n] = struct{}{}
		if errSend := s.writeResponseLine("message deleted", nil); errSend != nil {
			return errSend
		}

	case rsetCmd:
		clear(s.toDelete)
		if errSend := s.writeResponseLine("maildrop has been reset", nil); errSend != nil {
			return errSend
		}

	case noopCmd:
		if errSend := s.writeResponseLine("noop", nil); errSend != nil {
			return errSend
		}

	case topCmd:
		if !cmd.twoNumArgs() {
			if errSend := s.writeLine("-ERR invalid arguments\r\n"); errSend != nil {
				return errSend
			}
			break
		}

		n, nLines := cmd.numArgs[0], cmd.numArgs[1]
		if s.isMarketAsDelete(n) {
			if errSend := s.writeResponseLine("", ErrMessageMarkedAsDeleted); errSend != nil {
				return errSend
			}
		}

		r, err := s.mailbox.Message(n)
		if errSend := s.writeResponseLine("message body", err); errSend != nil {
			return errSend
		}
		if err != nil {
			break
		}

		if errSend := copyHeadersAndBody(s.conn, r, nLines); errSend != nil {
			return errSend
		}
		r.Close()

		if _, errSend := s.conn.Write([]byte(".\r\n")); errSend != nil {
			return errSend
		}

	case uidlCmd:
		if cmd.oneNumArg() {
			n := cmd.numArgs[0]
			if s.isMarketAsDelete(n) {
				if errSend := s.writeResponseLine("", ErrMessageMarkedAsDeleted); errSend != nil {
					return errSend
				}
			}
			uidl, err := s.mailbox.UidlOne(n)
			if errSend := s.writeResponseLine(fmt.Sprintf("%d %s", n+1, uidl), err); errSend != nil {
				return errSend
			}
			break
		}

		uidlList, err := s.mailbox.Uidl()
		if errSend := s.writeResponseLine(fmt.Sprintf("%d messages in mailbox", len(uidlList)), err); errSend != nil {
			return errSend
		}

		for i, uidl := range uidlList {
			if errSend := s.writeLine(fmt.Sprintf("%d %s\r\n", i+1, uidl)); errSend != nil {
				return errSend
			}
		}
		if errSend := s.writeLine(".\r\n"); errSend != nil {
			return errSend
		}

	case capaCmd:
		if err := s.handleCAPA(); err != nil {
			return err
		}

	case quitCmd:
		return s.Close()
	}

	return nil
}

func (s *Session) handleUpdateState(cmd command) error {
	panic("should not reach here")
}

func (s *Session) Close() error {
	defer s.conn.Close()

	var err error
	if s.mailbox != nil {
		for msg := range s.toDelete {
			if err = s.mailbox.Dele(msg); err != nil {
				break
			}
		}
		err = s.mailbox.Close()
	}

	return s.writeResponseLine("server signing off", err)
}

func (s *Session) writeLine(line string) error {
	log.Printf("C->S: %v", line)
	_, err := s.conn.Write([]byte(line))
	return err
}

func (s *Session) writeResponseLine(okResponse string, err error) error {
	var line string
	if err != nil {
		line = fmt.Sprintf("-ERR %s\r\n", err)
	} else {
		line = fmt.Sprintf("+OK %s\r\n", okResponse)
	}
	return s.writeLine(line)
}

func (s *Session) isMarketAsDelete(msg int) bool {
	_, ok := s.toDelete[msg]
	return ok
}

func timeoutCall[T any](fn func() (T, error), timeout time.Duration) (v T, err error) {
	if timeout <= 0 {
		return fn()
	}

	callDone := make(chan struct{})

	go func() {
		defer close(callDone)
		v, err = fn()
	}()

	select {
	case <-time.After(timeout):
		err = context.DeadlineExceeded
	case <-callDone:
	}

	return
}
