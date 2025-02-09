package pop3srv

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUidlLimiter(t *testing.T) {
	type testCase struct {
		name   string
		input  string
		limit  int
		output string
	}

	for _, c := range []testCase{
		{
			name: "empty",
		},
		{
			name: "header only",
			input: "field1: foo\n" +
				"field2: bar \r\n" +
				"\r\n" +
				"line1\r\n" +
				"line2\r\n",
			output: "field1: foo\r\n" +
				"field2: bar \r\n" +
				"\r\n",
			limit: 0,
		},
		{
			name: "one line",
			input: "field1: foo\n" +
				"field2: bar \r\n" +
				"\r\n" +
				"line1\r\n" +
				"line2\r\n",
			output: "field1: foo\r\n" +
				"field2: bar \r\n" +
				"\r\n" +
				"line1\r\n",
			limit: 1,
		},
		{
			name: "all lines",
			input: "field1: foo\n" +
				"field2: bar \r\n" +
				"\r\n" +
				"line1\r\n" +
				"line2\r\n",
			output: "field1: foo\r\n" +
				"field2: bar \r\n" +
				"\r\n" +
				"line1\r\n" +
				"line2\r\n",
			limit: 2,
		},
		{
			name: "all lines (limit greater than lines count)",
			input: "field1: foo\n" +
				"field2: bar \r\n" +
				"\r\n" +
				"line1\r\n" +
				"line2\r\n",
			output: "field1: foo\r\n" +
				"field2: bar \r\n" +
				"\r\n" +
				"line1\r\n" +
				"line2\r\n",
			limit: 3,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := &strings.Builder{}
			err := copyHeadersAndBody(w, strings.NewReader(c.input), c.limit)
			assert.NoError(t, err)
			assert.Equal(t, c.output, w.String())
		})
	}

}
