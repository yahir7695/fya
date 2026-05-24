//go:build !windows

package ptyrun

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKillProcessGroupNil(t *testing.T) {
	assert.NoError(t, killProcessGroup(nil))
}
