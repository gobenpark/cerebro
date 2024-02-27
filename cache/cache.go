package cache

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/internal/pkg"
	"github.com/gobenpark/cerebro/position"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type (
	Key func(ticker string) []byte
)

const (
	positionPrefix = "position"
	counterPrefix  = "counter"
)

var (
	Position Key = func(ticker string) []byte { return pkg.StringToBytes(fmt.Sprintf("%s:%s", positionPrefix, ticker)) }
	Counter  Key = func(ticker string) []byte { return pkg.StringToBytes(fmt.Sprintf("%s:%s", counterPrefix, ticker)) }
)

type Cache struct {
	cache *badger.DB
}

func NewCache(cache *badger.DB) *Cache {
	return &Cache{cache: cache}
}

func (c *Cache) GetPosition(ticker string) (position.Position, error) {
	var p position.Position
	err := c.cache.View(func(txn *badger.Txn) error {
		item, err := txn.Get(Position(ticker))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &p)
		})
	})
	return p, err
}

func (c *Cache) InitializePosition(p []position.Position) error {
	if err := c.cache.DropPrefix(pkg.StringToBytes(positionPrefix)); err != nil {
		return err
	}

	return c.cache.Update(func(txn *badger.Txn) error {
		for i := range p {
			bt, err := json.Marshal(p[i])
			if err != nil {
				return err
			}
			entry := badger.NewEntry(Position(p[i].Item.Code), bt)
			if err := txn.SetEntry(entry); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *Cache) UpdatePosition(p position.Position) error {
	bt, err := json.Marshal(p)
	if err != nil {
		return err
	}

	return c.cache.Update(func(txn *badger.Txn) error {
		return txn.SetEntry(badger.NewEntry(Position(p.Item.Code), bt))
	})
}

func (c *Cache) Positions() []position.Position {
	var positions []position.Position
	c.cache.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(pkg.StringToBytes("position")); it.ValidForPrefix(pkg.StringToBytes("position")); it.Next() {
			item := it.Item()
			var p position.Position
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &p)
			})
			if err != nil {
				return err
			}
			positions = append(positions, p)
		}
		return nil
	})
	return positions
}

func (c *Cache) AddCounter(tk indicator.Tick) {
}
