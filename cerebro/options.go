package cerebro

import (
	"time"

	"github.com/gobenpark/trader/domain"
	"github.com/rs/zerolog"
)

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

func WithStore(store domain.Store) CerebroOption {
	return func(c *Cerebro) {
		c.store = store
	}
}

func WithFeed(feed ...domain.Feed) CerebroOption {
	return func(c *Cerebro) {
		c.Feeds = feed
	}
}

func WithLogLevel(level zerolog.Level) CerebroOption {
	return func(c *Cerebro) {
		c.log.Level(level)
	}
}

func WithResample(level time.Duration) CerebroOption {
	return func(c *Cerebro) {
		c.compress = level
	}
}
