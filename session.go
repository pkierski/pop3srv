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
	log.Printf("S->C: %v", greetings)
	_, err := c.Write([]byte(greetings))
	if err != nil {
		return nil, err
	}
	return s, nil
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
			err = s.writeResponseLine("", ErrInvalidCommand)
			if err != nil {
				return err
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
			err := s.writeLine("-ERR invalid arguments\r\n")
			if err != nil {
				return err
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
		err = s.writeResponseLine("logged in", err)
		if err != nil {
			return err
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
		err = s.writeResponseLine(fmt.Sprintf("%d %d", n, size), err)
		if err != nil {
			return err
		}

	case listCmd:
		if cmd.oneNumArg() {
			n := cmd.numArgs[0]
			size, err := s.mailbox.ListOne(n - 1)
			err = s.writeResponseLine(fmt.Sprintf("%d %d", n, size), err)
			if err != nil {
				return err
			}
			break
		}

		list, err := s.mailbox.List()
		err = s.writeResponseLine(fmt.Sprintf("%d messages in mailbox", len(list)), err)
		if err != nil {
			return err
		}

		for i, size := range list {
			err = s.writeLine(fmt.Sprintf("%d %d\r\n", i+1, size))
			if err != nil {
				return err
			}
		}
		err = s.writeLine(".\r\n")
		if err != nil {
			return err
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
		r, err := s.mailbox.Message(n - 1)
		errSend := s.writeResponseLine(fmt.Sprintf("message body #%v", n), err)
		if errSend != nil {
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
			err := s.writeLine("-ERR invalid arguments\r\n")
			if err != nil {
				return err
			}
			break
		}

		n := cmd.numArgs[0]
		if n > s.msgCount {
			err := s.writeLine("-ERR invalid arguments\r\n")
			if err != nil {
				return err
			}
			break
		}
		s.toDelete[n] = struct{}{}
		err := s.writeResponseLine("message deleted", nil)
		if err != nil {
			return err
		}

	case rsetCmd:
		clear(s.toDelete)
		err := s.writeResponseLine("maildrop has been reset", nil)
		if err != nil {
			return err
		}

	case noopCmd:
		err := s.writeResponseLine("noop", nil)
		if err != nil {
			return err
		}

	case topCmd:
		if !cmd.twoNumArgs() {
			err := s.writeLine("-ERR invalid arguments\r\n")
			if err != nil {
				return err
			}
			break
		}

		n, nLines := cmd.numArgs[0], cmd.numArgs[1]
		r, err := s.mailbox.Message(n - 1)
		errSend := s.writeResponseLine("message body", err)
		if errSend != nil {
			return errSend
		}
		if err != nil {
			break
		}

		err = copyHeadersAndBody(s.conn, r, nLines)
		if err != nil {
			return err
		}
		r.Close()

		_, err = s.conn.Write([]byte(".\r\n"))
		if err != nil {
			return err
		}

	case uidlCmd:
		if cmd.oneNumArg() {
			n := cmd.numArgs[0]
			uidl, err := s.mailbox.UidlOne(n - 1)
			err = s.writeResponseLine(uidl, err)
			if err != nil {
				return err
			}
			break
		}

		uidlList, err := s.mailbox.Uidl()
		err = s.writeResponseLine(fmt.Sprintf("%d messages in mailbox", len(uidlList)), err)
		if err != nil {
			return err
		}

		for i, uidl := range uidlList {
			err = s.writeLine(fmt.Sprintf("%d %s\r\n", i+1, uidl))
			if err != nil {
				return err
			}
		}
		err = s.writeLine(".\r\n")
		if err != nil {
			return err
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
		return
	case <-callDone:
		return
	}
}
