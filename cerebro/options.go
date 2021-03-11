package cerebro

import "github.com/gobenpark/trader/domain"

func WithBroker(broker domain.Broker) CerebroOption {
	return func(c *Cerebro) {
		c.Broker = broker
	}
}

func WithStrategy(strategy ...domain.Strategy) CerebroOption {
	return func(c *Cerebro) {
		c.Strategies = strategy
	}
}

func WithStore(store domain.Store) CerebroOption {
	return func(c *Cerebro) {
		c.Store = store
	}
}

func WithFeed(feed ...domain.Feed) CerebroOption {
	return func(c *Cerebro) {
		c.Feeds = feed
	}
}
