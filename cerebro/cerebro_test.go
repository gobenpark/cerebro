package cerebro

import (
	"testing"
	"time"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/store"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewCerebro(t *testing.T) {
	tests := []struct {
		name    string
		cerebro *Cerebro
		checker func(c *Cerebro, t *testing.T)
	}{
		{
			"insert broker",
			NewCerebro(WithBroker(broker.NewBroker(10, 10))),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.broker)
			},
		},
		{
			"not insert broker",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.Nil(t, c.broker)
			},
		},
		{
			"live true",
			NewCerebro(WithLive(true)),
			func(c *Cerebro, t *testing.T) {
				assert.True(t, c.isLive)
			},
		},
		{
			"live false",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.False(t, c.isLive)
			},
		},
		{
			"preload false",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.False(t, c.preload)
			},
		},
		{
			"preload true",
			NewCerebro(WithPreload(true)),
			func(c *Cerebro, t *testing.T) {
				assert.True(t, c.preload)
			},
		},
		{
			"resample",
			NewCerebro(WithResample("resample", 10*time.Second)),
			func(c *Cerebro, t *testing.T) {
				assert.Len(t, c.compress, 1)
				assert.Equal(t, "resample", c.compress[0].code)
				assert.Equal(t, 10*time.Second, c.compress[0].level)
			},
		},
		{
			"log level info",
			NewCerebro(WithLogLevel(zerolog.InfoLevel)),
			func(c *Cerebro, t *testing.T) {
				assert.Equal(t, zerolog.InfoLevel, c.log.GetLevel())
			},
		},
		{
			"store option",
			NewCerebro(WithStore(store.NewStore("test"))),
			func(c *Cerebro, t *testing.T) {
				assert.Len(t, c.stores, 1)
			},
		},
		{
			"cerebro event channel exist",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.event)
			},
		},
		{
			"cerebro order channel exist",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.order)
			},
		},
		{
			"cerebro data container exist",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.container)
			},
		},
		{
			"cerebro strategy engine exist",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.StrategyEngine)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.checker(test.cerebro, t)
		})
	}
}
