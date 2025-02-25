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

	// Mailbox represents a backend interface for a single mailbox.
	//
	// All msgNumber arguments are 0-based indices.
	// Any method that takes msgNumber as an argument will be called
	// with values in the range [0..numberOfMessages-1] (inclusive).
	// Ensuring msgNumber is within this range is the responsibility
	// of the [Session].
	Mailbox interface {
		// Stat returns the total number of messages
		// in the mailbox and the total size of all messages.
		//
		// This is used for the STAT command.
		// numberOfMessages is also used for validation of arguments
		// for commands required message number as argument.
		Stat() (numberOfMessages int, totalSize int, err error)

		// List returns the sizes of all messages in the mailbox.
		//
		// This is used for the LIST command without parameters.
		List() (messageSizes []int, err error)

		// ListOne returns the size of a specific message
		// identified by msgNumber.
		//
		// This is used for the LIST command with a message number argument.
		ListOne(msgNumber int) (size int, err error)

		// Message returns an io.ReadCloser to access
		// the content of a specific message.
		//
		// This is used for the RETR and TOP commands.
		Message(msgNumber int) (msgReader io.ReadCloser, err error)

		// Dele marks a specific message for deletion from the mailbox.
		//
		// This method is called in the UPDATE state (after
		// the client issues the QUIT command) for all messages
		// that were marked as deleted.
		Dele(msgNumber int) error

		// Uidl returns a list of unique identifiers for
		// all messages in the mailbox.
		//
		// This is used for the UIDL command without parameters.
		Uidl() (uidls []string, err error)

		// UidlOne returns the unique identifier of a specific
		// message identified by msgNumber.
		//
		// This is used for the UIDL command with a message number argument.
		UidlOne(msgNumber int) (uidl string, err error)

		// Close is called at the end of a session in the UPDATE state
		// after all required Dele calls are completed.
		//
		// This follows the io.Closer interface.
		io.Closer
	}

	// Authorizer is authorization interface
	// as merge of [UserPassAuthorizer] and [ApopAuthorizer].
	//
	// Implementation can indicate lack of support particular
	// authorization method by returning [ErrNotSupportedAuthMethod].
	// All methods are called with empty parameters on creating [Session].
	Authorizer interface {
		UserPassAuthorizer
		ApopAuthorizer
	}

	// UserPassAuthorizer defines an interface for user authentication using
	// a username and password.
	//
	// [UserPass] is called on creating session with empty parameters to determine
	// if [Authorizer] supports USER/PASS authorization.
	UserPassAuthorizer interface {
		// UserPass authenticates a user based on the provided username and password.
		//
		// Returns nil if authentication is successful. Returns an error if authentication
		// fails due to invalid credentials or other issues.
		//
		// A special case is when [ErrNotSupportedAuthMethod] is returned,
		// indicating that this authorizer does not support USER/PASS authentication
		// at all.
		// If an authorizer returns this specific error, the USER command will be
		// removed from the server's capability list (response for CAPA command).
		UserPass(user, pass string) error
	}

	// ApopAuthorizer defines an interface for user authentication using the APOP
	// mechanism.
	//
	// [Apop] is called on session creation with empty parameters to determine if the
	// authorizer supports APOP authentication.
	ApopAuthorizer interface {
		// Apop authenticates a user based on the provided username, timestamp banner, and digest.
		//
		// The timestampBanner is the unique challenge string sent by the server during the
		// session initialization. The digest is an MD5 hash of the timestamp concatenated
		// with the user's secret (password).
		//
		// Returns nil if authentication is successful. Returns an error if authentication
		// fails due to invalid credentials or other issues.
		//
		// A special case is when [ErrNotSupportedAuthMethod] is returned, indicating that
		// this authorizer does not support APOP authentication at all.
		// If an authorizer returns this specific error the [Session] won't send
		// challenge in welcome message, which indicates lack of support of APOP command.
		Apop(user, timestampBanner, digest string) error
	}
	apopDisabler struct {
		UserPassAuthorizer
	}

	userPassDisabler struct {
		ApopAuthorizer
	}
)

var (
	ErrUserNotSpecified       = errors.New("user not specified")
	ErrUserAlreadySpecified   = errors.New("user already specified")
	ErrInvalidCommand         = errors.New("invalid command")
	ErrInvalidArgument        = errors.New("invalid argument")
	ErrMessageMarkedAsDeleted = errors.New("message marked as deleted")
	ErrNotSupportedAuthMethod = errors.New("not suported authorization method")
)

var (
	_ Authorizer = (*apopDisabler)(nil)
	_ Authorizer = (*userPassDisabler)(nil)
)

// DisableApop wraps a [UserPassAuthorizer] and explicitly signals
// that APOP authentication is not supported.
//
// This function allows the implementation of an [Authorizer] using
// only a [UserPassAuthorizer], ensuring that the timestamp banner
// for APOP command is removed from the server's greetings message.
func DisableApop(a UserPassAuthorizer) apopDisabler {
	return apopDisabler{
		UserPassAuthorizer: a,
	}
}

func (apopDisabler) Apop(user, timestampBanner, digest string) error {
	return ErrNotSupportedAuthMethod
}

// This function allows the implementation of an [Authorizer] using
// only a [ApopAuthorizer], ensuring that the USER command is
// removed from the server's capability list.
func DisableUserPass(a ApopAuthorizer) userPassDisabler {
	return userPassDisabler{
		ApopAuthorizer: a,
	}
}

func (userPassDisabler) UserPass(user, pass string) error {
	return ErrNotSupportedAuthMethod
}
