package cerebro

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/domain"
	mock_domain "github.com/gobenpark/trader/domain/mock"
	"github.com/gobenpark/trader/store"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewCerebro(t *testing.T) {
	tests := []struct {
		name    string
		cerebro *Cerebro
		checker func(c *Cerebro, t *testing.T)
	}{
		{
			"insert broker",
			NewCerebro(WithBroker(broker.NewBroker(10, 10))),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.broker)
			},
		},
		{
			"not insert broker",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.Nil(t, c.broker)
			},
		},
		{
			"live true",
			NewCerebro(WithLive(true)),
			func(c *Cerebro, t *testing.T) {
				assert.True(t, c.isLive)
			},
		},
		{
			"live false",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.False(t, c.isLive)
			},
		},
		{
			"preload false",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.False(t, c.preload)
			},
		},
		{
			"preload true",
			NewCerebro(WithPreload(true)),
			func(c *Cerebro, t *testing.T) {
				assert.True(t, c.preload)
			},
		},
		{
			"resample",
			NewCerebro(WithResample("resample", 10*time.Second)),
			func(c *Cerebro, t *testing.T) {
				assert.Len(t, c.compress, 1)
				assert.Equal(t, "resample", c.compress[0].code)
				assert.Equal(t, 10*time.Second, c.compress[0].level)
			},
		},
		{
			"log level info",
			NewCerebro(WithLogLevel(zerolog.InfoLevel)),
			func(c *Cerebro, t *testing.T) {
				assert.Equal(t, zerolog.InfoLevel, c.log.GetLevel())
			},
		},
		{
			"store option",
			NewCerebro(WithStore(store.NewStore("test"))),
			func(c *Cerebro, t *testing.T) {
				assert.Len(t, c.stores, 1)
			},
		},
		{
			"cerebro event channel exist",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.event)
			},
		},
		{
			"cerebro order channel exist",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.order)
			},
		},
		{
			"cerebro data container exist",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.container)
			},
		},
		{
			"cerebro strategy engine exist",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.strategyEngine)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.checker(test.cerebro, t)
		})
	}
}

func TestCerebro_Stop(t *testing.T) {
	c := NewCerebro()
	err := c.Stop()
	assert.NoError(t, err)
	assert.Equal(t, "context canceled", c.Ctx.Err().Error())
}

func TestCerebro_load(t *testing.T) {
	ctrl := gomock.NewController(t)
	con := mock_domain.NewMockContainer(ctrl)
	store := mock_domain.NewMockStore(ctrl)
	input := []domain.Candle{
		{
			Code:   "test",
			Low:    1,
			High:   3,
			Open:   3,
			Close:  3,
			Volume: 3,
			Date:   time.Now(),
		},
		{
			Code:   "test2",
			Low:    1,
			High:   3,
			Open:   3,
			Close:  3,
			Volume: 3,
			Date:   time.Now(),
		},
	}

	store.EXPECT().LoadHistory(gomock.Any()).DoAndReturn(func(ctx context.Context) ([]domain.Candle, error) {
		return input, nil
	})
	con.EXPECT().Add(input[0])
	con.EXPECT().Add(input[1])

	c := NewCerebro(WithPreload(true), WithStore(store))
	c.container = con
	err := c.load()
	assert.NoError(t, err)

	t.Run("load error", func(t *testing.T) {
		store.EXPECT().LoadHistory(gomock.Any()).Return(nil, errors.New("error"))
		err := c.load()
		assert.Error(t, err)
	})
}

func TestCerebro_Start(t *testing.T) {
	c := NewCerebro()
	go func() {
		<-time.After(1 * time.Second)
		c.Stop()
	}()
	err := c.Start()
	assert.NoError(t, err)
}
