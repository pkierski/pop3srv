package pop3srv

import (
	"bufio"
	"io"
)

var (
	crlf = []byte("\r\n")
)

// copyHeadersAndBody copies email headers and a limited number of body lines
// from an io.Reader to an io.Writer. Additionally converts all line endings to CRLF.
// The last line is always terminated with CRLF.
func copyHeadersAndBody(w io.Writer, r io.Reader, lineLimit int) error {
	scanner := bufio.NewScanner(r)

	headersDone := false
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Bytes()

		if !headersDone {
			// Check if we've reached the end of the headers.
			if len(line) == 0 {
				headersDone = true
			}
		} else {
			lineCount++
		}

		if lineCount > lineLimit {
			break
		}

		if _, err := w.Write(line); err != nil {
			return err
		}
		if _, err := w.Write(crlf); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
