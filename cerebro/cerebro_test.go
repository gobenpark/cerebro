package cerebro

import (
	"testing"
	"time"

	"github.com/gobenpark/trader/broker"
	mock_domain "github.com/gobenpark/trader/domain/mock"
	"github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/strategy"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	uuid "github.com/satori/go.uuid"
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
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				store := store.NewStore("upbit", "codes")
				WithResample(store, 3*time.Minute)(c)
				assert.Equal(t, 3*time.Minute, c.compress[store.Uid()].level)
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
			NewCerebro(WithStore(store.NewStore("test", "code"))),
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
				assert.NotNil(t, c.containers)
			},
		},
		{
			"cerebro strategy engine exist",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.strategyEngine)
			},
		},
		{
			"add user strategy",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				WithStrategy(&strategy.Bighands{})(c)
				assert.NotNil(t, c.strategies)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.checker(test.cerebro, t)
		})
	}
}

func TestCerebro_load(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := mock_domain.NewMockStore(ctrl)
	store.EXPECT().Uid().Return(uuid.NewV4().String()).AnyTimes()
	store.EXPECT().LoadHistory(gomock.Any(), 0*time.Second)
	store.EXPECT().LoadTick(gomock.Any())
	c := NewCerebro(WithLive(true), WithPreload(true), WithStore(store))
	go func() {
		<-time.After(time.Second)
		c.Stop()
	}()
	err := c.load()
	assert.NoError(t, err)

	t.Run("store not exist", func(t *testing.T) {
		c := NewCerebro(WithLive(true))
		go func() {
			<-time.After(time.Second)
			c.Stop()
		}()
		err := c.load()
		assert.Error(t, err)
	})

}

func TestCerebro_Stop(t *testing.T) {
	c := NewCerebro()
	err := c.Stop()
	assert.NoError(t, err)
	assert.Equal(t, "context canceled", c.Ctx.Err().Error())
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
