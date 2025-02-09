package pop3srv

import (
	"errors"
	"io"
)

type (
	Conn interface {
		io.ReadWriteCloser
	}

	MailboxProvider interface {
		Provide(user string) (Mailbox, error)
	}

	Mailbox interface {
		Stat() (numberOfMessages int, totalSize int, err error)
		List() (messageSizes []int, err error)
		ListOne(msgNumber int) (size int, err error)
		Message(msgNumber int) (msgReader io.ReadCloser, err error)
		Dele(msgNumber int) error
		Uidl() (uidls []string, err error)
		UildOne(msgNumber int) (uidl string, err error)
		io.Closer
	}

	Authorizer interface {
		UserPass(user, pass string) error
		Apop(user, timestampBanner, digest string) error
	}
)

var (
	ErrUserNotSpecified     = errors.New("user not specified")
	ErrUserAlreadySpecified = errors.New("user already specified")
	ErrInvalidCommand       = errors.New("invalid command")
)
