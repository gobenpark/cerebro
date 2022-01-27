package container

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuffer(t *testing.T) {
	buf := bytes.NewBuffer([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	require.Equal(t, buf.Len(), 9)
	fmt.Println(buf.Cap())

	fmt.Println(buf.Next(1))
	fmt.Println(buf.Next(1))
	fmt.Println(buf.Cap())
	fmt.Println(buf.Len())
}
