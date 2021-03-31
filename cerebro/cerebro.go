package cerebro

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gobenpark/trader/broker"
	"github.com/gobenpark/trader/container"
	error2 "github.com/gobenpark/trader/error"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/internal/pkg/retry"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/store"
	"github.com/gobenpark/trader/strategy"
)

// Cerebro head of trading system
// make all dependency manage
type Cerebro struct {
	//isLive flog of live trading
	isLive bool

	//Broker buy, sell and manage order
	broker *broker.Broker `validate:"required"`

	store store.Store

	codes []string

	//Ctx cerebro global context
	Ctx context.Context `json:"ctx" validate:"required"`

	//Cancel cerebro global context cancel
	Cancel context.CancelFunc `json:"cancel" validate:"required"`

	//strategies
	strategies []strategy.Strategy `validate:"gte=1,dive,required"`

	//compress compress info map for codes
	compress map[string][]CompressInfo

	// containers list of all container
	containers []container.Container

	//strategy.StrategyEngine embedding property for managing user strategy
	strategyEngine *strategy.Engine

	//log in cerebro global logger
	Logger Logger `validate:"required"`

	//event channel of all event
	order chan order.Order

	// eventEngine engine of management all event
	eventEngine *event.Engine

	// preload bool value, decide use candle history
	preload bool

	// dataCh all data container channel
	dataCh chan container.Container
}

//NewCerebro generate new cerebro with cerebro option
func NewCerebro(opts ...Option) *Cerebro {
	ctx, cancel := context.WithCancel(context.Background())

	c := &Cerebro{
		Ctx:            ctx,
		Cancel:         cancel,
		compress:       make(map[string][]CompressInfo),
		strategyEngine: &strategy.Engine{},
		order:          make(chan order.Order, 1),
		dataCh:         make(chan container.Container, 1),
		eventEngine:    event.NewEventEngine(),
		broker:         broker.NewBroker(),
	}

	for _, opt := range opts {
		opt(c)
	}
	if c.Logger == nil {
		c.Logger = getLogger()
	}

	return c
}

func (c *Cerebro) getContainer(code string, level time.Duration) container.Container {
	for k, v := range c.containers {
		if v.Code() == code && v.Level() == level {
			return c.containers[k]
		}
	}
	return nil
}

//load initializing data from injected store interface
func (c *Cerebro) load() error {
	//gocyclo:ignore
	if c.preload {
		for _, code := range c.codes {
			for _, comp := range c.compress[code] {
				if err := retry.Retry(10, func() error {
					candles, err := c.store.LoadHistory(c.Ctx, code, comp.level)
					if err != nil {
						c.Logger.Error(err)
						return err
					}
					con := c.getContainer(code, comp.level)
					for _, candle := range candles {
						con.Add(candle)
					}
					return nil
				}); err != nil {
					return err
				}
			}
		}
	}

	if c.isLive {
		c.Logger.Info("start load live data")
		if c.store == nil {
			return error2.ErrStoreNotExists
		}

		for _, i := range c.codes {
			var tick <-chan container.Tick
			if err := retry.Retry(10, func() error {
				var err error
				tick, err = c.store.LoadTick(c.Ctx, i)
				if err != nil {
					return err
				}
				return nil
			}); err != nil {
				return err
			}

			for _, com := range c.compress[i] {
				if con := c.getContainer(i, com.level); con != nil {
					go func(t <-chan container.Tick, con container.Container, level time.Duration, isLeftEdge bool) {
						for j := range Compression(t, level, isLeftEdge) {
							con.Add(j)
							c.dataCh <- con
						}
					}(tick, con, com.level, com.LeftEdge)
				}
			}
		}
	}

	return nil
}

func (c *Cerebro) registerEvent() {
	c.eventEngine.Register <- c.strategyEngine
}

func (c *Cerebro) createContainer() {
	for _, i := range c.codes {
		for _, j := range c.compress[i] {
			c.containers = append(c.containers, container.NewDataContainer(container.Info{
				Code:             i,
				CompressionLevel: j.level,
			}))
		}
	}
}

//Start run cerebro
// first check cerebro validation
// second load from store data
// third other engine setup
func (c *Cerebro) Start() error {
	done := make(chan os.Signal)
	signal.Notify(done, syscall.SIGTERM)

	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		c.Logger.Error(err)
		return err
	}

	c.createContainer()

	c.eventEngine.Start(c.Ctx)
	c.registerEvent()
	c.Logger.Info("Cerebro start ...")
	c.broker.Store = c.store
	c.strategyEngine.Broker = c.broker
	c.strategyEngine.Start(c.Ctx, c.dataCh)

	c.broker.SetEventBroadCaster(c.eventEngine)

	c.Logger.Info("loading...")
	if err := c.load(); err != nil {
		return err
	}

	select {
	case <-c.Ctx.Done():
		break
	case <-done:
		break
	}
	return nil
}

//Stop all cerebro goroutine and finish
func (c *Cerebro) Stop() error {
	c.Cancel()
	return nil
}
