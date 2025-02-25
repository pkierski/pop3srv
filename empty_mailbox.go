package pop3srv

import (
	"errors"
	"io"
)

var (
	// sanity check if intefaces are properly implemented
	_ Mailbox         = (*EmptyMailbox)(nil)
	_ MailboxProvider = (*EmptyMailboxProvider)(nil)
	_ Authorizer      = (*AllowAllAuthorizer)(nil)
)

// EmptyMailbox is a trivial implementation of [Mailbox]
// which represents empty mailbox.
type EmptyMailbox struct{}

func (EmptyMailbox) Stat() (int, int, error) {
	return 0, 0, nil
}

func (EmptyMailbox) List() ([]int, error) {
	return nil, nil
}

func (EmptyMailbox) ListOne(_ int) (int, error) {
	return 0, nil
}

func (EmptyMailbox) Message(_ int) (io.ReadCloser, error) {
	return nil, errors.New("no such message")
}

func (EmptyMailbox) Dele(_ int) error {
	return nil
}

func (EmptyMailbox) Uidl() ([]string, error) {
	return nil, nil
}

func (EmptyMailbox) UidlOne(_ int) (string, error) {
	return "", errors.New("no such message")
}

func (EmptyMailbox) Close() error {
	return nil
}

// EmptyMailboxProvider is a trivial implementation
// of [MailboxProvider] which returns empty mailbox
// for all users.
type EmptyMailboxProvider struct{}

func (EmptyMailboxProvider) Provide(user string) (Mailbox, error) {
	return EmptyMailbox{}, nil
}

// AllowAllAuthorizer is a trivial implementation of [Authorizer]
// which allows any user with any credentials.
type AllowAllAuthorizer struct{}

func (AllowAllAuthorizer) UserPass(user, pass string) error {
	return nil
}

func (AllowAllAuthorizer) Apop(user, timestampBanner, digest string) error {
	return nil
}
