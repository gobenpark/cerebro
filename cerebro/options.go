package cerebro

import (
	"time"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/datacontainer"
	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/strategy"
	"github.com/rs/zerolog"
)

type Option func(*Cerebro)

func WithBroker(broker broker.Broker) Option {
	return func(c *Cerebro) {
		c.broker = broker
		c.broker.SetEventBroadCaster(c.eventEngine)
	}
}

func WithStrategy(strategy ...strategy.Strategy) Option {
	return func(c *Cerebro) {
		c.strategies = strategy
	}
}

func WithStore(store store.Store, codes ...string) Option {
	return func(c *Cerebro) {
		c.storengine.Stores[store.Uid()] = store
		c.storengine.Mapper[store.Uid()] = append(c.storengine.Mapper[store.Uid()], codes...)
	}
}

func WithLogLevel(level zerolog.Level) Option {
	return func(c *Cerebro) {
		c.log = c.log.Level(level)
	}
}

func WithLive(isLive bool) Option {
	return func(c *Cerebro) {
		c.isLive = isLive
	}
}

func WithResample(store domain.Store, level time.Duration, leftEdge bool) Option {
	return func(c *Cerebro) {
		c.compress[store.Uid()] = append(c.compress[store.Uid()], CompressInfo{level: level, LeftEdge: leftEdge})
		c.containers = append(c.containers, datacontainer.NewDataContainer(datacontainer.ContainerInfo{
			Code:             store.Code(),
			CompressionLevel: level,
		}))
	}
}

func WithPreload(b bool) Option {
	return func(c *Cerebro) {
		c.preload = b
	}
}
