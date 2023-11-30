package indicator

import (
	"fmt"
	"testing"

	_ "embed"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

//go:embed "data.json"
var data []byte

func ParseCandles(t *testing.T) Candles {
	var candles Candles
	err := json.Unmarshal(data, &candles)
	require.NoError(t, err)
	return candles
}

func TestStandardDeviation(t *testing.T) {
	candles := ParseCandles(t)
	fmt.Println(candles)
}

func TestBollingerBand(t *testing.T) {
	candles := ParseCandles(t)
	candles.BollingerBand(20)
}
