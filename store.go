package cerebro

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/gobenpark/cerebro/container"
	"github.com/gobenpark/cerebro/internal/pkg"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
	"github.com/gobenpark/kinvest-go"
)

type st struct {
	*kv.Kinvest
}

func (s *st) MarketItems(ctx context.Context) []item.Item {
	//TODO implement me
	panic("implement me")
}

func (s *st) Candles(ctx context.Context, code string, level time.Duration) (container.Candles, error) {
	ss := time.Now().Add(-(24 * 365 * time.Hour))
	p, err := s.DailyChartPrice(ctx, ss, time.Now(), kv.Stock, kv.Day, code, true)
	if err != nil {
		return nil, err
	}

	var candles container.Candles
	for _, i := range p.Output2 {
		o, err := strconv.ParseInt(i.StckOprc, 10, 64)
		if err != nil {
			return nil, err
		}

		h, err := strconv.ParseInt(i.StckHgpr, 10, 64)
		if err != nil {
			return nil, err
		}
		l, err := strconv.ParseInt(i.StckLwpr, 10, 64)
		if err != nil {
			return nil, err
		}

		c, err := strconv.ParseInt(i.StckClpr, 10, 64)
		if err != nil {
			return nil, err
		}

		v, err := strconv.ParseInt(i.AcmlVol, 10, 64)
		if err != nil {
			return nil, err
		}

		ti, err := time.Parse("20060102", i.StckBsopDate)
		if err != nil {
			return nil, err
		}

		candles = append(candles, container.Candle{
			Type:   container.Day,
			Code:   code,
			Open:   o,
			High:   h,
			Low:    l,
			Close:  c,
			Volume: v,
			Date:   ti,
		})
	}
	return candles, nil
}

func (s *st) Tick(ctx context.Context, codes ...string) (<-chan container.Tick, error) {
	var once sync.Once
	approvalkey, err := s.ApprovalKey(ctx)
	if err != nil {
		return nil, err
	}

	ch := make(chan container.Tick, 1)

	for i := range codes {
		go func(code string) {

			res, err := s.RealtimeContract(ctx, approvalkey, code)
			if err != nil {
				return
			}
			for v := range pkg.OrDone(ctx, res) {
				for _, j := range v.Datas {
					n := time.Now()
					lo, _ := time.LoadLocation("Asia/Seoul")
					timein := time.Date(n.Year(), n.Month(), n.Day(), j.GetContractHour().Hour(), j.GetContractHour().Minute(), j.GetContractHour().Second(), 0, lo)

					ch <- container.Tick{
						Code:   j.GetCode(),
						Date:   timein,
						Price:  int64(j.GetPrice()),
						Volume: int64(j.GetContractVolume()),
					}
				}
			}
			once.Do(func() {
				close(ch)
			})
		}(codes[i])
	}
	return ch, nil
}
func (s *st) TradeCommits(ctx context.Context, code string) ([]container.TradeHistory, error) {
	//TODO implement me
	panic("implement me")
}

func (s *st) Order(ctx context.Context, o order.Order) error {
	//TODO implement me
	panic("implement me")
}

func (s *st) Cancel(id string) error {
	//TODO implement me
	panic("implement me")
}

func (s *st) Uid() string {
	//TODO implement me
	panic("implement me")
}

func (s *st) Cash() int64 {
	//TODO implement me
	panic("implement me")
}

func (s *st) Commission() float64 {
	//TODO implement me
	panic("implement me")
}

func (s *st) Positions() map[string]position.Position {
	//TODO implement me
	panic("implement me")
}

func (s *st) OrderInfo(id string) (order.Order, error) {
	//TODO implement me
	panic("implement me")
}
