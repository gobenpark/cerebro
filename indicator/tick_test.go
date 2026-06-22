package indicator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gobenpark/cerebro/indicator"
)

func TestTicks_Mean(t *testing.T) {
	is := assert.New(t)

	ticks := indicator.Ticks{{Price: dec(10)}, {Price: dec(20)}, {Price: dec(30)}}
	is.InDelta(20.0, ticks.Mean(), 1e-9)
}

func TestTicks_MeanEmptyIsZero(t *testing.T) {
	is := assert.New(t)
	is.InDelta(0.0, indicator.Ticks{}.Mean(), 1e-9)
}

func TestTicks_StandardDeviation(t *testing.T) {
	is := assert.New(t)

	// [10,20,30]: mean 20, sample variance (100+0+100)/(3-1)=100, sd=10.
	ticks := indicator.Ticks{{Price: dec(10)}, {Price: dec(20)}, {Price: dec(30)}}
	is.InDelta(10.0, ticks.StandardDeviation(), 1e-9)
}
