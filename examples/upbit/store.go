package main

import (
	"context"
	"sort"
	"time"

	"github.com/gobenpark/proto/stock"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/order"
	"github.com/gogo/protobuf/types"
	"github.com/rs/zerolog/log"
	uuid "github.com/satori/go.uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type store struct {
	name string
	uid  string
	cli  stock.StockClient
}

func NewStore(name string) *store {
	cli, err := stock.NewSocketClient(context.Background(), "localhost:50051")
	if err != nil {
		panic(err)
	}

	return &store{name, uuid.NewV4().String(), cli}
}

func (s *store) LoadHistory(ctx context.Context, code string, du time.Duration) ([]container.Candle, error) {

	r, err := s.cli.Chart(context.Background(), &stock.ChartRequest{
		Code: code,
		To:   nil,
	})
	if err != nil {
		return nil, err
	}
	var d []container.Candle
	for _, i := range r.GetData() {
		ti, err := types.TimestampFromProto(i.GetDate())
		if err != nil {
			log.Err(err).Send()
		}
		d = append(d, container.Candle{
			Code:   code,
			Low:    i.GetLow(),
			High:   i.GetHigh(),
			Open:   i.GetOpen(),
			Close:  i.GetClose(),
			Volume: i.GetVolume(),
			Date:   ti,
		})
	}

	sort.Slice(d, func(i, j int) bool {
		return d[i].Date.Before(d[j].Date)
	})

	return d, nil
}

func (s *store) LoadTick(ctx context.Context, code string) (<-chan container.Tick, error) {
	ch := make(chan container.Tick, 1)

	r, err := s.cli.TickStream(ctx, &stock.TickRequest{Codes: code})
	if err != nil {
		return nil, err
	}
	go func(tch chan container.Tick) {
	Done:
		for {
			select {
			case <-ctx.Done():
				break Done
			default:
				msg, err := r.Recv()
				if err != nil {
					st, _ := status.FromError(err)
					if st.Code() == codes.Unimplemented || st.Code() == codes.Unavailable {
						break
					}
					log.Err(err).Send()
				}
				ti, err := types.TimestampFromProto(msg.GetDate())
				if err != nil {
					log.Err(err).Send()
				}
				tch <- container.Tick{
					Code:   code,
					Date:   ti,
					Price:  msg.GetPrice(),
					Volume: msg.GetVolume(),
				}
			}
		}
	}(ch)

	return ch, nil
}

func (s *store) Order(code string, ot order.OType, size int64, price float64) error {
	switch ot {
	case order.Buy:
		return nil
		_, err := s.cli.Buy(context.Background(), &stock.BuyRequest{
			Code:       code,
			Otype:      stock.LimitOrder,
			Volume:     float64(size),
			Price:      price,
			Identifier: uuid.NewV4().String(),
		})
		if err != nil {
			return err
		}
	case order.Sell:
		return nil
		_, err := s.cli.Sell(context.Background(), &stock.SellRequest{
			Code:       code,
			Otype:      stock.LimitOrder,
			Volume:     float64(size),
			Price:      price,
			Identifier: uuid.NewV4().String(),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *store) Cancel(id string) error {
	if _, err := s.cli.CancelOrder(context.Background(), &stock.CancelOrderRequest{Id: id}); err != nil {
		return err
	}
	return nil
}

func (s *store) Uid() string {
	return s.uid
}
