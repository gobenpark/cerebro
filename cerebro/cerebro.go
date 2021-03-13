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

	container domain.Container

	eventEngine *event.EventEngine

	//strategy.StrategyEngine embedding property for managing user strategy
	strategyEngine *strategy.StrategyEngine

	//log in cerebro global logger
	log zerolog.Logger `json:"log" validate:"required"`

	//event channel of all event
	event chan event.Event

	order chan order.Order

	compress []CompressInfo

	preload bool

	data chan domain.Container

	events []chan event.Event
}

//NewCerebro generate new cerebro with cerebro option
func NewCerebro(opts ...CerebroOption) *Cerebro {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().Timestamp().Str("logger", "cerebro").Logger()
	ctx, cancel := context.WithCancel(context.Background())

	c := &Cerebro{
		Ctx:    ctx,
		Cancel: cancel,
		log:    logger,
		container: &datacontainer.DataContainer{
			CandleData: make(map[string][]domain.Candle),
		},
		strategyEngine: &strategy.StrategyEngine{},
		event:          make(chan event.Event, 1),
		order:          make(chan order.Order, 1),
		data:           make(chan domain.Container, 1),
		eventEngine:    event.NewEventEngine(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

//load initializing data from injected store interface
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

					c.container.Add(cde)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			c.log.Err(err).Send()
			return err
		}
	}
	c.log.Info().Msg("start load live data ")
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
							c.container.Add(j)
							c.data <- c.container
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
func (c *Cerebro) registEvent() {
	c.eventEngine.Register <- c.strategyEngine
}

//Start run cerebro
// first check cerebro validation
// second load from store data
// third other engine setup
func (c *Cerebro) Start() error {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGKILL)

	validate := validator.New()
	if err := validate.Struct(c); err != nil {
		c.log.Err(err).Send()
		return err
	}

	c.eventEngine.Start(c.Ctx)
	c.registEvent()
	c.log.Info().Msg("Cerebro start...")
	c.strategyEngine.Broker = c.broker
	c.strategyEngine.Start(c.Ctx, c.data, c.strategies)

	c.log.Info().Msg("startload")
	if err := c.load(); err != nil {
		return err
	}
	c.log.Info().Msg("end load")

	<-ch
	return nil
}

//Stop all cerebro goroutine and finish
func (c *Cerebro) Stop() error {
	c.Cancel()
	return nil
}
