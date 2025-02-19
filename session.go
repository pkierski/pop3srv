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

// #region Exported methods

// NewSession creates new [Session] with [MailboxProvider] and [Authorizer].
//
// Connection parameter is used as read-write channel, usually TCP connection.
// But it can be constructed with any [Conn] type (at this moment alias for
// [io.ReadWriteCloser] but it can change in the future).
//
// After construction greetings message (with APOP banner) is sent
// to the connection. Error is the error returned by Write operation on
// the connection.
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

// Serve is the main loop which read commands and write reponses.
//
// It returns non-nil error if there is any error on reading or writting
// data with connection. [MailboxProvider] and [Authorizer] errors are
// reported as -ERR response.
func (s *Session) Serve() error {
	for s.state != updateState {
		cmd, err := timeoutCall(s.readCommand, 10*time.Second)
		if err != nil {
			return err
		}

		if err = s.handleState(starteDispatch[s.state], cmd); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the session: it deletes messages marked as deleted from
// mailbox (if the mailbox was created as a result of successful authorization),
// then sent farewell status line (+OK or -ERR depending on messages' deletion result)
// and finally closes the connection.
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

// #endregion

// #region Dispatcher
type (
	handlerMethod func(s *Session, cmd command) error
	handlersMap   map[string]handlerMethod
)

var (
	authorizationStateDispatch = handlersMap{
		userCmd: (*Session).handleUser,
		passCmd: (*Session).handlePass,
		quitCmd: (*Session).handleQuit,
		apopCmd: (*Session).handleApop,
		capaCmd: (*Session).handleCapa,
	}
	transactionStateDispatch = handlersMap{
		quitCmd: (*Session).handleQuit,
		capaCmd: (*Session).handleCapa,
		statCmd: (*Session).handleStat,
		listCmd: (*Session).handleList,
		retrCmd: (*Session).handleRetr,
		deleCmd: (*Session).handleDele,
		rsetCmd: (*Session).handleRset,
		noopCmd: (*Session).handleNoop,
		topCmd:  (*Session).handleTop,
		uidlCmd: (*Session).handleUidl,
	}

	starteDispatch = map[sessionState]handlersMap{
		authorizationState: authorizationStateDispatch,
		transactionState:   transactionStateDispatch,
	}
)

func (s *Session) handleState(dispatcher handlersMap, cmd command) error {
	handler, found := dispatcher[cmd.name]
	if found {
		return handler(s, cmd)
	}
	return s.writeResponseLine("", ErrInvalidCommand)
}

// #endregion

// #region Command handlers
func (s *Session) handleUser(cmd command) error {
	if s.user != "" {
		return s.writeResponseLine("", ErrUserAlreadySpecified)
	}
	s.user = cmd.args[0]
	return s.writeResponseLine("send PASS", nil)
}

func (s *Session) handlePass(cmd command) error {
	if s.user == "" {
		return s.writeResponseLine("", ErrUserNotSpecified)
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
	return s.writeResponseLine("logged in", err)
}

func (s *Session) handleApop(cmd command) error {
	if len(cmd.args) != 2 {
		return s.writeLine("-ERR invalid arguments\r\n")
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

	return s.writeResponseLine("logged in", err)
}

func (s *Session) handleCapa(_ command) error {
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

func (s *Session) handleQuit(_ command) error {
	s.state = updateState
	return s.Close()
}

func (s *Session) handleUidl(cmd command) error {
	if cmd.oneNumArg() {
		n := cmd.numArgs[0]
		if s.isMarkedAsDeleted(n) {
			return s.writeResponseLine("", ErrMessageMarkedAsDeleted)
		}
		uidl, err := s.mailbox.UidlOne(n)
		return s.writeResponseLine(fmt.Sprintf("%d %s", n+1, uidl), err)
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
	return s.writeLine(".\r\n")
}

func (s *Session) handleTop(cmd command) error {
	if !cmd.twoNumArgs() {
		return s.writeLine("-ERR invalid arguments\r\n")
	}

	n, nLines := cmd.numArgs[0], cmd.numArgs[1]
	if s.isMarkedAsDeleted(n) {
		return s.writeResponseLine("", ErrMessageMarkedAsDeleted)
	}

	r, err := s.mailbox.Message(n)
	if errSend := s.writeResponseLine("message body", err); errSend != nil {
		return errSend
	}
	if err != nil {
		return nil
	}

	if errSend := copyHeadersAndBody(s.conn, r, nLines); errSend != nil {
		return errSend
	}
	r.Close()

	return s.writeLine(".\r\n")
}

func (s *Session) handleNoop(_ command) error {
	return s.writeResponseLine("noop", nil)
}

func (s *Session) handleRset(_ command) error {
	clear(s.toDelete)
	return s.writeResponseLine("maildrop has been reset", nil)
}

func (s *Session) handleDele(cmd command) error {
	if !cmd.oneNumArg() {
		return s.writeLine("-ERR invalid arguments\r\n")
	}

	n := cmd.numArgs[0]
	if s.isMarkedAsDeleted(n) {
		return s.writeResponseLine("", ErrMessageMarkedAsDeleted)
	}
	if n > s.msgCount {
		return s.writeLine("-ERR invalid arguments\r\n")
	}

	s.toDelete[n] = struct{}{}
	return s.writeResponseLine("message deleted", nil)
}

func (s *Session) handleRetr(cmd command) error {
	if !cmd.oneNumArg() {
		return s.writeLine("-ERR invalid arguments\r\n")
	}

	n := cmd.numArgs[0]
	if s.isMarkedAsDeleted(n) {
		return s.writeResponseLine("", ErrMessageMarkedAsDeleted)
	}

	r, err := s.mailbox.Message(n)
	if errSend := s.writeResponseLine(fmt.Sprintf("message body #%v", n+1), err); errSend != nil {
		return errSend
	}
	if err != nil {
		return nil
	}

	dotWriter := textproto.NewWriter(bufio.NewWriter(s.conn)).DotWriter()
	_, errCopy := io.Copy(dotWriter, r)
	errCloseR := r.Close()
	errCloseW := dotWriter.Close()
	return errors.Join(errCopy, errCloseR, errCloseW)
}

func (s *Session) handleStat(_ command) error {
	n, size, err := s.mailbox.Stat()
	return s.writeResponseLine(fmt.Sprintf("%d %d", n, size), err)
}

func (s *Session) handleList(cmd command) error {
	if cmd.oneNumArg() {
		n := cmd.numArgs[0]
		if s.isMarkedAsDeleted(n) {
			if errSend := s.writeResponseLine("", ErrMessageMarkedAsDeleted); errSend != nil {
				return errSend
			}
		}
		size, err := s.mailbox.ListOne(n)
		return s.writeResponseLine(fmt.Sprintf("%d %d", n+1, size), err)
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
	return s.writeLine(".\r\n")
}

// #endregion

// #region Helpers
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

func (s *Session) isMarkedAsDeleted(msg int) bool {
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

// #endregion
