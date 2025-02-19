package pop3srv_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

type connMock struct {
	w  *bytes.Buffer
	bw *bufio.Reader

	linesToRead []string
	closed      bool
	err         error
}

func (m *connMock) Read(p []byte) (n int, err error) {
	if m.closed {
		return 0, io.EOF
	}
	if len(m.linesToRead) == 0 {
		return 0, m.err
	}
	if len(p) >= len(m.linesToRead[0]) {
		copy(p, []byte(m.linesToRead[0]))
		m.linesToRead = m.linesToRead[1:]
	} else {
		copy(p, []byte(m.linesToRead[0][:len(p)]))
		m.linesToRead[0] = m.linesToRead[0][len(p):]
	}

	return len(p), nil
}

func (m *connMock) Write(p []byte) (n int, err error) {
	return m.w.Write(p)
}

func (m *connMock) Close() error {
	if m.closed {
		return fmt.Errorf("already closed")
	}
	m.closed = true
	return nil
}

func newConnMock() *connMock {
	writeBuffer := bytes.NewBuffer(nil)
	return &connMock{
		w:   writeBuffer,
		bw:  bufio.NewReader(writeBuffer),
		err: io.EOF,
	}
}

func (m *connMock) nextWrittenLine() string {
	line := &strings.Builder{}
	for {
		lineFragment, err := m.bw.ReadString('\n')
		line.WriteString(lineFragment)
		if len(lineFragment) > 0 && lineFragment[len(lineFragment)-1] == '\n' {
			break
		}
		if err != nil || err != bufio.ErrBufferFull {
			break
		}
	}
	return line.String()
}
