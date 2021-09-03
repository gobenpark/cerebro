package broker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBroker(t *testing.T) {
	b := NewBroker()
	require.NotNil(t, b)
}
