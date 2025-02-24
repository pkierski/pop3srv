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

	// Mailbox is a interface for single mailbox backend.
	//
	// All msgNumber arguments are 0-based indices.
	// Any of methods with msgNumber will be called with
	// values range [0..numberOfMessages-1] inclusive (range
	// check is the Session responsiblility).
	Mailbox interface {
		Stat() (numberOfMessages int, totalSize int, err error)
		List() (messageSizes []int, err error)
		ListOne(msgNumber int) (size int, err error)
		Message(msgNumber int) (msgReader io.ReadCloser, err error)
		Dele(msgNumber int) error
		Uidl() (uidls []string, err error)
		UidlOne(msgNumber int) (uidl string, err error)
		io.Closer
	}

	Authorizer interface {
		UserPassAuthorizer
		ApopAuthorizer
	}

	UserPassAuthorizer interface {
		UserPass(user, pass string) error
	}

	ApopAuthorizer interface {
		Apop(user, timestampBanner, digest string) error
	}

	apopDisabler struct {
		UserPassAuthorizer
		ApopAuthorizer
	}

	userPassDisabler struct {
		ApopAuthorizer
		UserPassAuthorizer
	}
)

var (
	ErrUserNotSpecified       = errors.New("user not specified")
	ErrUserAlreadySpecified   = errors.New("user already specified")
	ErrInvalidCommand         = errors.New("invalid command")
	ErrMessageMarkedAsDeleted = errors.New("message marked as deleted")
	ErrNotSupportedAuthMethod = errors.New("not suported authorization method")
)

var (
	_ Authorizer = (*apopDisabler)(nil)
	_ Authorizer = (*userPassDisabler)(nil)
)

func DisableApop(a UserPassAuthorizer) apopDisabler {
	return apopDisabler{
		ApopAuthorizer:     apopDisabler{},
		UserPassAuthorizer: a,
	}
}

func (apopDisabler) Apop(user, timestampBanner, digest string) error {
	return ErrNotSupportedAuthMethod
}

func DisableUserPass(a ApopAuthorizer) userPassDisabler {
	return userPassDisabler{
		UserPassAuthorizer: userPassDisabler{},
		ApopAuthorizer:     a,
	}
}

func (userPassDisabler) UserPass(user, pass string) error {
	return ErrNotSupportedAuthMethod
}
