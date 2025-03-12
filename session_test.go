package pop3srv_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/pkierski/pop3srv"
	"github.com/pkierski/pop3srv/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ConnectionTestSuite struct {
	suite.Suite

	conn           *mocks.ConnMock
	provider       *mocks.MailboxProvider
	mockAuthorizer *mocks.Authorizer
	authorizer     pop3srv.Authorizer
	session        *pop3srv.Session
}

func (suite *ConnectionTestSuite) SetupTest() {
	suite.conn = mocks.NewConnMock()
	suite.provider = mocks.NewMailboxProvider(suite.T())

	suite.mockAuthorizer = mocks.NewAuthorizer(suite.T())
	//
	suite.mockAuthorizer.On("Apop", "", "", "").Return(nil)
	suite.mockAuthorizer.On("UserPass", "", "").Return(nil)

	suite.authorizer = suite.mockAuthorizer
	suite.session = pop3srv.NewSession(suite.conn, suite.provider, suite.authorizer)
}

func (suite *ConnectionTestSuite) TearDownTest() {
	mock.AssertExpectationsForObjects(suite.T(), suite.provider, suite.mockAuthorizer)
}

func TestAddTaskTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionTestSuite))
}

func (suite *ConnectionTestSuite) TestSessionConnectQuit() {
	// GIVEN
	suite.conn.LinesToRead = []string{"QUIT\r\n"}

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)

	// has APOP banner
	assert.Regexp(suite.T(), `\+OK .+ \<\d+\.\d+@.+\>`, suite.conn.NextWrittenLine())
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK "))
	assert.True(suite.T(), suite.conn.Closed)
}

func (suite *ConnectionTestSuite) TestSessionConnectInvalidCommand() {
	// GIVEN
	suite.conn.LinesToRead = []string{"foobar\r\n"}

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.ErrorIs(suite.T(), err, io.EOF)

	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK "))
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "-ERR "))
	assert.False(suite.T(), suite.conn.Closed) // don't enter in update state, don't close connection as far as io.EOF was encoutered
}

func (suite *ConnectionTestSuite) TestSessionConnectErrorRead() {
	// GIVEN
	expectedErr := errors.New("foobar")
	suite.conn.Err = expectedErr

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.ErrorIs(suite.T(), err, expectedErr)

	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK "))
	assert.Empty(suite.T(), suite.conn.NextWrittenLine())
	assert.False(suite.T(), suite.conn.Closed) // don't enter in update state, don't close connection as far as io.EOF was encoutered
}

func (suite *ConnectionTestSuite) TestSessionUserPassSuccess() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("Close").Return(nil).Once()         // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionApopSuccess() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"APOP testuser digestvalue\r\n",
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("Close").Return(nil).Once()         // Called during QUIT
	suite.mockAuthorizer.On("Apop", "testuser", mock.AnythingOfType("string"), "digestvalue").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	banner := suite.conn.NextWrittenLine()
	assert.True(suite.T(), strings.HasPrefix(banner, "+OK"))
	assert.True(suite.T(), strings.Contains(banner, "<"))                          // Contains APOP banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // APOP response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionListAllMessages() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"LIST\r\n",
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("List").Return([]int{500, 524}, nil)
	mailbox.On("Close").Return(nil).Once() // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // LIST response
	assert.Equal(suite.T(), "1 500\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), "2 524\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), ".\r\n", suite.conn.NextWrittenLine())
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionListSingleMessageSuccess() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"LIST 1\r\n",
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("ListOne", 0).Return(500, nil)      // Internal index is 0-based for message #1
	mailbox.On("Close").Return(nil).Once()         // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // PASS response
	assert.Equal(suite.T(), "+OK 1 500\r\n", suite.conn.NextWrittenLine())         // LIST response with size
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionListInvalidMessageNumber() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"LIST 999\r\n", // Message number beyond mailbox size
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("Close").Return(nil).Once()         // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "-ERR")) // LIST response with error
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionCloseError() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"QUIT\r\n",
	}
	expectedErr := errors.New("close error")
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("Close").Return(expectedErr).Once() // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)                                                  // Server continues despite Close error
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "-ERR")) // QUIT response with error
}

func (suite *ConnectionTestSuite) TestSessionStat() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"STAT\r\n",
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called for STAT command
	mailbox.On("Close").Return(nil).Once()
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // PASS response
	assert.Equal(suite.T(), "+OK 2 1024\r\n", suite.conn.NextWrittenLine())        // STAT response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionTop() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"TOP 1 2\r\n",
		"QUIT\r\n",
	}
	messageContent := "Subject: Test\r\n\r\nLine1\r\nLine2\r\nLine3\r\n"
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once()
	mailbox.On("Message", 0).Return(io.NopCloser(strings.NewReader(messageContent)), nil)
	mailbox.On("Close").Return(nil).Once()
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // TOP response
	assert.Equal(suite.T(), "Subject: Test\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), "\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), "Line1\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), "Line2\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), ".\r\n", suite.conn.NextWrittenLine())
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionUidl() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"UIDL\r\n",
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once()
	mailbox.On("Uidl").Return([]string{"uid1", "uid2"}, nil)
	mailbox.On("Close").Return(nil).Once()
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // UIDL response
	assert.Equal(suite.T(), "1 uid1\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), "2 uid2\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), ".\r\n", suite.conn.NextWrittenLine())
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionUidlSingleMessage() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"UIDL 1\r\n",
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("UidlOne", 0).Return("uid1", nil)   // Internal index is 0-based for message #1
	mailbox.On("Close").Return(nil).Once()         // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // PASS response
	assert.Equal(suite.T(), "+OK 1 uid1\r\n", suite.conn.NextWrittenLine())        // UIDL response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionUidlInvalidMessageNumber() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"UIDL 999\r\n", // Message number beyond mailbox size
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("Close").Return(nil).Once()         // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "-ERR")) // UIDL response with error
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionUidlDeletedMessage() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"DELE 1\r\n",
		"UIDL 1\r\n",
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("Dele", 0).Return(nil).Once()       // Called during QUIT
	mailbox.On("Close").Return(nil).Once()         // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // DELE response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "-ERR")) // UIDL response (message deleted)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionNoop() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"NOOP\r\n",
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once()
	mailbox.On("Close").Return(nil).Once()
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // NOOP response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionRetrMessage() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"RETR 1\r\n",
		"QUIT\r\n",
	}
	messageContent := "From: sender@example.com\r\nTo: recipient@example.com\r\n\r\nTest message body\r\n"
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once()                                        // Called during auth
	mailbox.On("Message", 0).Return(io.NopCloser(strings.NewReader(messageContent)), nil) // Internal index is 0-based
	mailbox.On("Close").Return(nil).Once()                                                // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // RETR response
	assert.Equal(suite.T(), "From: sender@example.com\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), "To: recipient@example.com\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), "\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), "Test message body\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), ".\r\n", suite.conn.NextWrittenLine())
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionRetrInvalidMessageNumber() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"RETR 999\r\n", // Message number beyond mailbox size
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("Close").Return(nil).Once()         // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "-ERR")) // RETR response with error
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionRetrDeletedMessage() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"DELE 1\r\n",
		"RETR 1\r\n",
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("Close").Return(nil).Once()         // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)
	mailbox.On("Dele", 0).Return(nil).Once() // Called during QUIT

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // DELE response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "-ERR")) // RETR response (message deleted)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionRetrMessageError() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"RETR 1\r\n",
		"QUIT\r\n",
	}
	expectedErr := errors.New("message access error")
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once()    // Called during auth
	mailbox.On("Message", 0).Return(nil, expectedErr) // Message access fails
	mailbox.On("Close").Return(nil).Once()            // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "-ERR")) // RETR response with error
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionRset() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"DELE 1\r\n",
		"RSET\r\n",
		"LIST 1\r\n",
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once() // Called during auth
	mailbox.On("ListOne", 0).Return(500, nil)      // Called after RSET
	mailbox.On("Close").Return(nil).Once()         // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // DELE response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // RSET response
	assert.Equal(suite.T(), "+OK 1 500\r\n", suite.conn.NextWrittenLine())         // LIST response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionRsetBeforeAuth() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"RSET\r\n",
		"QUIT\r\n",
	}

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "-ERR")) // RSET response (not authenticated)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK"))  // QUIT response
}

func (suite *ConnectionTestSuite) TestSessionRsetMultipleMessages() {
	// GIVEN
	suite.conn.LinesToRead = []string{
		"USER testuser\r\n",
		"PASS testpass\r\n",
		"DELE 1\r\n",
		"DELE 2\r\n",
		"LIST\r\n",
		"RSET\r\n",
		"LIST\r\n",
		"QUIT\r\n",
	}
	mailbox := mocks.NewMailbox(suite.T())
	mailbox.On("Stat").Return(2, 1024, nil).Once()         // Called during auth
	mailbox.On("List").Return(nil, nil).Once()             // Empty list when messages are deleted
	mailbox.On("List").Return([]int{500, 524}, nil).Once() // List after RSET
	mailbox.On("Close").Return(nil).Once()                 // Called during QUIT
	suite.mockAuthorizer.On("UserPass", "testuser", "testpass").Return(nil)
	suite.provider.On("Provide", "testuser").Return(mailbox, nil)

	// WHEN
	err := suite.session.Serve()

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // Banner
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // USER response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // PASS response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // DELE #1 response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // DELE #2 response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // First LIST response
	assert.Equal(suite.T(), ".\r\n", suite.conn.NextWrittenLine())                 // Empty list termination
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // RSET response
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // Second LIST response
	assert.Equal(suite.T(), "1 500\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), "2 524\r\n", suite.conn.NextWrittenLine())
	assert.Equal(suite.T(), ".\r\n", suite.conn.NextWrittenLine())
	assert.True(suite.T(), strings.HasPrefix(suite.conn.NextWrittenLine(), "+OK")) // QUIT response
}
