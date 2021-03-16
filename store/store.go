package store

//go:generate mockgen -source=./store.go -destination=./mock/mock_store.go

import (
	"context"
	"time"

	"github.com/gobenpark/proto/stock"
	"github.com/gobenpark/trader/domain"
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

func (s *store) LoadHistory(ctx context.Context, du time.Duration) ([]domain.Candle, error) {

	r, err := s.cli.Chart(context.Background(), &stock.ChartRequest{
		Code: "KRW-BTC",
		To:   nil,
	})
	if err != nil {
		return nil, err
	}
	var d []domain.Candle
	for _, i := range r.GetData() {
		ti, err := types.TimestampFromProto(i.GetDate())
		if err != nil {
			log.Err(err).Send()
		}
		d = append(d, domain.Candle{
			Code:   "KRW-BTC",
			Low:    i.GetLow(),
			High:   i.GetHigh(),
			Open:   i.GetOpen(),
			Close:  i.GetClose(),
			Volume: i.GetVolume(),
			Date:   ti,
		})
	}
	return d, nil
}

func (s *store) LoadTick(ctx context.Context) (<-chan domain.Tick, error) {
	ch := make(chan domain.Tick, 1)

	r, err := s.cli.TickStream(ctx, &stock.TickRequest{Codes: "KRW-BTC"})
	if err != nil {
		return nil, err
	}
	go func(tch chan domain.Tick) {
	Done:
		for {
			select {
			case <-ctx.Done():
				break Done
			default:
				msg, err := r.Recv()
				if err != nil {
					st, _ := status.FromError(err)
					if st.Code() == codes.Unimplemented {
						break
					}
					log.Err(err).Send()
				}
				ti, err := types.TimestampFromProto(msg.GetDate())
				if err != nil {
					log.Err(err).Send()
				}
				tch <- domain.Tick{
					Code:   "KRW-BTC",
					Date:   ti,
					Price:  msg.GetPrice(),
					Volume: msg.GetVolume(),
				}
			}
		}
	}(ch)

	return ch, nil
}

func (s *store) Order() {
	panic("implement me")
}

func (s *store) Cancel() {
	panic("implement me")
}

func (s *store) Uid() string {
	return s.uid
}
