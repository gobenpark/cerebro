package order

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOrderMarshall(t *testing.T) {
	o := NewOrder("005930", Buy, Market, 100, 1000)
	bt, err := json.Marshal(o)
	require.NoError(t, err)
	fmt.Println(string(bt))
}
