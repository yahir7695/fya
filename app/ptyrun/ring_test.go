package ptyrun

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTailBufferLimit(t *testing.T) {
	r := &tailBuffer{data: make([]byte, 0, 5), limit: 5}

	r.Write([]byte("hello"))
	r.Write([]byte(" world"))

	assert.Equal(t, "world", r.String())
}

// repeated over-limit writes must keep the visible length bounded at the limit;
// the backing array may grow once during append's amortized growth but the
// trimmed slice length should stay at limit, not balloon.
func TestTailBufferStaysBoundedLength(t *testing.T) {
	r := &tailBuffer{data: make([]byte, 0, 8), limit: 8}

	for range 32 {
		r.Write([]byte("abcd"))
	}

	assert.Len(t, r.data, 8)
	assert.Equal(t, "abcdabcd", r.String())
}
