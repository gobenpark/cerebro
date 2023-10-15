package strategy

import (
	"context"
	"errors"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/gobenpark/cerebro/indicators"
	"github.com/gobenpark/cerebro/store"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Container interface {
	Candle(du time.Duration) (indicators.Candles, error)
	Tick() indicators.Tick
	UpdateTick(tick indicators.Tick)
}

type container struct {
	Code string
	store.Store
	cache *badger.DB
	tick  indicators.Tick
}

func (c *container) UpdateTick(tick indicators.Tick) {
	c.tick = tick
}

func (c *container) Candle(du time.Duration) (indicators.Candles, error) {
	var candles indicators.Candles
	if err := c.cache.Update(func(txn *badger.Txn) error {
		it, err := txn.Get([]byte(CandleKey(c.Code, du.String())))
		if err != nil && errors.Is(err, badger.ErrKeyNotFound) {
			candles, err := c.Store.Candles(context.Background(), c.Code, du)
			if err != nil {
				return err
			}
			bt, err := json.Marshal(candles)
			if err != nil {
				return err
			}

			entry := badger.NewEntry([]byte(CandleKey(c.Code, du.String())), bt)
			entry.WithTTL(time.Minute)
			return txn.SetEntry(entry)
		}
		return it.Value(func(val []byte) error {
			return json.Unmarshal(val, &candles)
		})
	}); err != nil {
		return nil, err
	}

	return candles, nil
}

func (c *container) Tick() indicators.Tick {
	return c.tick
}
