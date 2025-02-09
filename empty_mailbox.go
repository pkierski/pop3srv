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

func (EmptyMailbox) UildOne(_ int) (string, error) {
	return "", errors.New("no such message")
}

func (EmptyMailbox) Close() error {
	return nil
}

type EmptyMailboxProvider struct{}

func (EmptyMailboxProvider) Provide(user string) (Mailbox, error) {
	return EmptyMailbox{}, nil
}

type AllowAllAuthorizer struct{}

func (AllowAllAuthorizer) UserPass(user, pass string) error {
	return nil
}

func (AllowAllAuthorizer) Apop(user, timestampBanner, digest string) error {
	return nil
}
