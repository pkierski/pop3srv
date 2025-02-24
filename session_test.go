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
