package strategy

import (
	"context"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/store"
)

type CandleProvider interface {
	// GetCandle returns candles of code
	Candles(ctx context.Context, candleType store.CandleType) (indicator.Candles, error)
}

type provider struct {
	st  store.Store
	itm item.Item
}

func NewCandleProvider(st store.Store, itm item.Item) CandleProvider {
	return &provider{st: st, itm: itm}
}

func (p *provider) Candles(ctx context.Context, candleType store.CandleType) (indicator.Candles, error) {
	return p.st.Candles(ctx, p.itm.Code, candleType)

}
