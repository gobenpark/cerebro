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
		c.broker.SetEventBroadCaster(c.eventEngine)
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

func WithLogLevel(level zerolog.Level) CerebroOption {
	return func(c *Cerebro) {
		c.log = c.log.Level(level)
	}
}

func WithLive(isLive bool) CerebroOption {
	return func(c *Cerebro) {
		c.isLive = isLive
	}
}

func WithResample(store domain.Store, level time.Duration) CerebroOption {
	return func(c *Cerebro) {
		c.compress[store.Uid()] = CompressInfo{level: level}
	}
}

func WithPreload(b bool) CerebroOption {
	return func(c *Cerebro) {
		c.preload = b
	}
}
