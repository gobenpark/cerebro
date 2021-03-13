package cerebro

//go:generate mockgen -source=./cerebro.go -destination=./mock/mock_cerebro.go

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gobenpark/trader/datacontainer"
	"github.com/gobenpark/trader/domain"
	"github.com/gobenpark/trader/event"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/strategy"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

type Cerebro struct {
	//isLive flog of live trading
	isLive bool

	//Broker buy, sell and manage order
	broker domain.Broker `json:"broker" validate:"required"`

	//Ctx cerebro global context
	Ctx context.Context `json:"ctx" validate:"required"`

	//Cancel cerebro global context cancel
	Cancel context.CancelFunc `json:"cancel" validate:"required"`

	//strategies
	strategies []domain.Strategy `json:"strategis" validate:"gte=1,dive,required"`

	//stores external api, etc
	//store can inject into cerebro what external api or oher tick,candle buy,sell history data
	stores []domain.Store
	//Feeds
	datacontainer.DataContainer

	strategy.StrategyEngine

	log zerolog.Logger `json:"log" validate:"required"`
	//event channel of all event
	event chan event.Event

	order chan order.Order

	compress []CompressInfo

	preload bool
}

func NewCerebro(opts ...CerebroOption) *Cerebro {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().Timestamp().Str("logger", "cerebro").Logger()
	ctx, cancel := context.WithCancel(context.Background())

	c := &Cerebro{
		Ctx:            ctx,
		Cancel:         cancel,
		log:            logger,
		DataContainer:  datacontainer.DataContainer{},
		StrategyEngine: strategy.StrategyEngine{},
		event:          make(chan event.Event, 1),
		order:          make(chan order.Order, 1),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Cerebro) load() error {
	g, ctx := errgroup.WithContext(c.Ctx)
	if c.preload {
		for _, i := range c.stores {
			g.Go(func() error {
				candle, err := i.LoadHistory(ctx)
				if err != nil {
					return err
				}

				for _, i := range candle {
					cde := domain.Candle{
						Code:   i.Code,
						Low:    i.Low,
						High:   i.High,
						Open:   i.Open,
						Close:  i.Close,
						Volume: i.Volume,
						Date:   i.Date,
					}

					if err := c.DataContainer.Add(cde); err != nil {
						return err
					}
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			c.log.Err(err).Send()
			return err
		}
	}
	if c.isLive {
		var storeTicks []<-chan domain.Tick
		for _, i := range c.stores {
			tick, err := i.LoadTick(c.Ctx)
			if err != nil {
				return err
			}

			ch := make(chan domain.Tick, 1)
			storeTicks = append(storeTicks, ch)
			go func(t <-chan domain.Tick, tch chan domain.Tick) {
				defer close(tch)
				for j := range t {
					tch <- j
				}
			}(tick, ch)

		}

		//TODO: tick데이터도 저장?
		for _, i := range storeTicks {
			go func(ch <-chan domain.Tick) {
				for _, com := range c.compress {
					go func() {
						for j := range Compression(ch, com.level) {
							if err := c.DataContainer.Add(j); err != nil {
								c.log.Err(err).Send()
							}
							c.log.Info().Str("code", j.Code).Interface("candle", j).Int("container length", c.DataContainer.Size()).Send()
						}
					}()
				}
			}(i)
		}

		<-c.Ctx.Done()

		//Compression(ch,time.Minute * 3)
	}
	return nil
}

func (c *Cerebro) Start() error {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGKILL)

	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		c.log.Err(err).Send()
		return err
	}
	c.log.Info().Msg("Cerebro start...")

	c.log.Info().Msg("startload")
	if err := c.load(); err != nil {
		return err
	}
	c.log.Info().Msg("end load")

	c.StrategyEngine.Broker = c.broker
	c.StrategyEngine.Start(c.Ctx)
	c.broker.SetEventCh(c.event)
	<-ch
	return nil
}

func (c *Cerebro) Stop() error {
	c.Cancel()
	return nil
}
