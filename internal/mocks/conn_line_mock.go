package mocks

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

type ConnMock struct {
	w  *bytes.Buffer
	bw *bufio.Reader

	LinesToRead []string
	Closed      bool
	Err         error
}

func (m *ConnMock) Read(p []byte) (n int, err error) {
	if m.Closed {
		return 0, io.EOF
	}
	if len(m.LinesToRead) == 0 {
		return 0, m.Err
	}
	if len(p) >= len(m.LinesToRead[0]) {
		copy(p, []byte(m.LinesToRead[0]))
		m.LinesToRead = m.LinesToRead[1:]
	} else {
		copy(p, []byte(m.LinesToRead[0][:len(p)]))
		m.LinesToRead[0] = m.LinesToRead[0][len(p):]
	}

	return len(p), nil
}

func (m *ConnMock) Write(p []byte) (n int, err error) {
	return m.w.Write(p)
}

func (m *ConnMock) Close() error {
	if m.Closed {
		return fmt.Errorf("already closed")
	}
	m.Closed = true
	return nil
}

func NewConnMock() *ConnMock {
	writeBuffer := bytes.NewBuffer(nil)
	return &ConnMock{
		w:   writeBuffer,
		bw:  bufio.NewReader(writeBuffer),
		Err: io.EOF,
	}
}

func (m *ConnMock) NextWrittenLine() string {
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
