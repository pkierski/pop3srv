package pop3srv_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/pkierski/pop3srv"
	"github.com/pkierski/pop3srv/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ConnectionTestSuite struct {
	suite.Suite

	conn       *connMock
	provider   *mocks.MailboxProvider
	authorizer *mocks.Authorizer
}

func (suite *ConnectionTestSuite) SetupTest() {
	suite.conn = newConnMock()
	suite.provider = mocks.NewMailboxProvider(suite.T())
	suite.authorizer = mocks.NewAuthorizer(suite.T())
}

func (suite *ConnectionTestSuite) TearDownTest() {
	mock.AssertExpectationsForObjects(suite.T(), suite.authorizer, suite.provider)
}

func TestAddTaskTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionTestSuite))
}

func (suite *ConnectionTestSuite) TestSessionConnectQuit() {
	// GIVEN
	suite.conn.linesToRead = []string{"QUIT\r\n"}

	// WHEN
	session, err := pop3srv.NewSession(suite.conn, suite.provider, suite.authorizer)

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.nextWrittenLine(), "+OK "))

	// WHEN
	err = session.Serve()
	// THEN
	assert.NoError(suite.T(), err)

	assert.True(suite.T(), strings.HasPrefix(suite.conn.nextWrittenLine(), "+OK "))
	assert.True(suite.T(), suite.conn.closed)
}

func (suite *ConnectionTestSuite) TestSessionConnectInvalidCommand() {
	// GIVEN
	suite.conn.linesToRead = []string{"foobar\r\n"}

	// WHEN
	session, err := pop3srv.NewSession(suite.conn, suite.provider, suite.authorizer)

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.nextWrittenLine(), "+OK "))

	// WHEN
	err = session.Serve()
	assert.ErrorIs(suite.T(), err, io.EOF)

	assert.True(suite.T(), strings.HasPrefix(suite.conn.nextWrittenLine(), "-ERR "))
	assert.False(suite.T(), suite.conn.closed) // don't enter in update state, don't close connection as far as io.EOF was encoutered
}

func (suite *ConnectionTestSuite) TestSessionConnectErrorRead() {
	// GIVEN
	expectedErr := errors.New("foobar")
	suite.conn.err = expectedErr

	// WHEN
	session, err := pop3srv.NewSession(suite.conn, suite.provider, suite.authorizer)

	// THEN
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), strings.HasPrefix(suite.conn.nextWrittenLine(), "+OK "))

	// WHEN
	err = session.Serve()
	// THEN
	assert.ErrorIs(suite.T(), err, expectedErr)

	assert.Empty(suite.T(), suite.conn.nextWrittenLine())
	assert.False(suite.T(), suite.conn.closed) // don't enter in update state, don't close connection as far as io.EOF was encoutered
}
