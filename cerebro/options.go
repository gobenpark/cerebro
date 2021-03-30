package cerebro

import (
	"time"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/strategy"
)

type Option func(*Cerebro)

func WithBroker(b broker.Broker) Option {
	return func(c *Cerebro) {
		c.broker = b
	}
}

func WithStrategy(s ...strategy.Strategy) Option {
	return func(c *Cerebro) {
		c.strategyEngine.Sts = s
	}
}

func WithStore(s store.Store, initCodes ...string) Option {
	return func(c *Cerebro) {
		c.storeEngine.Store = s
		c.storeEngine.Codes = initCodes
	}
}

func WithLive(isLive bool) Option {
	return func(c *Cerebro) {
		c.isLive = isLive
	}
}

func WithResample(code string, level time.Duration, leftEdge bool) Option {
	return func(c *Cerebro) {
		c.compress[code] = append(c.compress[code], CompressInfo{level: level, LeftEdge: leftEdge})
	}
}

func WithPreload(b bool) Option {
	return func(c *Cerebro) {
		c.preload = b
	}
}
