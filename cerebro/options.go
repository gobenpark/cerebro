package cerebro

import (
	"time"

	"github.com/gobenpark/trader/domain"
	"github.com/rs/zerolog"
)

type CerebroOption func(*Cerebro)

func WithBroker(broker domain.Broker) CerebroOption {
	return func(c *Cerebro) {
		c.broker = broker
	}
}

func WithStrategy(strategy ...domain.Strategy) CerebroOption {
	return func(c *Cerebro) {
		c.strategies = strategy
	}
}

func WithStore(stores ...domain.Store) CerebroOption {
	return func(c *Cerebro) {
		c.stores = append(c.stores, stores...)
	}
}

func WithFeed(feed ...domain.Feed) CerebroOption {
	return func(c *Cerebro) {
		//c.Feeds = feed
	}
}

func WithLogLevel(level zerolog.Level) CerebroOption {
	return func(c *Cerebro) {
		c.log.Level(level)
	}
}

func WithLive(isLive bool) CerebroOption {
	return func(c *Cerebro) {
		c.isLive = isLive
	}
}

func WithResample(code string, level time.Duration) CerebroOption {
	return func(c *Cerebro) {
		c.compress = append(c.compress, CompressInfo{
			code:  code,
			level: level,
		})
	}
}

func WithPreload(b bool) CerebroOption {
	return func(c *Cerebro) {
		c.preload = b
	}
}
