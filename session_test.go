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
