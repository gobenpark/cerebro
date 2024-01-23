package cache

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/position"
	"github.com/stretchr/testify/require"
)

func CreateCache(t *testing.T) *Cache {
	t.Helper()
	db, err := badger.Open(badger.DefaultOptions("").WithLoggingLevel(badger.ERROR).WithInMemory(true))
	require.NoError(t, err)
	return NewCache(db)
}

func TestCache_UpdatePosition(t *testing.T) {
	cache := CreateCache(t)
	input := position.Position{
		Item: item.Item{
			Code: "005930",
			Name: "삼성전자",
			Type: item.KOSPI,
		},
		Size:  100,
		Price: 51240,
	}
	err := cache.UpdatePosition(input)
	require.NoError(t, err)

	p, err := cache.GetPosition("005930")
	require.NoError(t, err)
	require.Equal(t, input.Item.Code, p.Item.Code, p)
	require.Equal(t, input.Item.Name, p.Item.Name, p)
	require.Equal(t, input.Size, p.Size, p)
	require.Equal(t, input.Price, p.Price, p)

	t.Run("get positions", func(t *testing.T) {
		positions := cache.Positions()
		require.Len(t, positions, 1)
		for _, input := range positions {
			require.Equal(t, input.Item.Code, p.Item.Code, p)
			require.Equal(t, input.Item.Name, p.Item.Name, p)
			require.Equal(t, input.Size, p.Size, p)
			require.Equal(t, input.Price, p.Price, p)
		}
	})

	t.Run("init position", func(t *testing.T) {
		input := position.Position{
			Item: item.Item{
				Code: "005930",
				Name: "삼성전자",
				Type: item.KOSPI,
			},
			Size:  100,
			Price: 10,
		}

		err := cache.InitializePosition([]position.Position{input})
		require.NoError(t, err)

		p, err := cache.GetPosition("005930")
		require.NoError(t, err)
		fmt.Println(p)
	})

}
