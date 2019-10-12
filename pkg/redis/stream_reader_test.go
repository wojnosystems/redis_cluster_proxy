package redis

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCommands(t *testing.T) {
	cases := []struct {
		input         string
		expected      Componenter
		expectedError error
	}{
		{
			input: "*2\r\n$3\r\nGET\r\n$3\r\nKEY\r\n",
			expected: &Array{
				NewBulkStringFromString("GET"),
				NewBulkStringFromString("KEY"),
			},
		},
	}

	buffer := make([]byte, BufferSizeBytes)
	for caseName, c := range cases {
		stream := bytes.NewBufferString(c.input)
		actual, _, err := ComponentFromReader(stream, buffer)
		assert.Equal(t, c.expectedError, err, caseName)
		assert.Equal(t, c.expected, actual, caseName)
	}
}
