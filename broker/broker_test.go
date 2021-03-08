package broker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBroker(t *testing.T) {
	b := NewBroker(1, 1)
	require.NotNil(t, b)
}
