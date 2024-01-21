package cache

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"github.com/gobenpark/cerebro/internal/pkg"
	"github.com/gobenpark/cerebro/position"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Key string

const (
	Position Key = "position"
)

func positionKey(ticker string) []byte {
	return pkg.StringToBytes(fmt.Sprintf("%s:%s", Position, ticker))
}

type Cache struct {
	cache *badger.DB
}

func NewCache(cache *badger.DB) *Cache {
	return &Cache{cache: cache}
}

func (c *Cache) GetPosition(ticker string) (position.Position, error) {
	var p position.Position
	err := c.cache.View(func(txn *badger.Txn) error {
		item, err := txn.Get(positionKey(ticker))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &p)
		})
	})
	return p, err
}

func (c *Cache) UpdatePosition(p position.Position) error {
	bt, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return c.cache.Update(func(txn *badger.Txn) error {
		return txn.SetEntry(badger.NewEntry(positionKey(p.Item.Code), bt))
	})
}

func (c *Cache) Positions() []position.Position {
	var positions []position.Position
	c.cache.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(pkg.StringToBytes(string(Position))); it.ValidForPrefix(pkg.StringToBytes(string(Position))); it.Next() {
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
