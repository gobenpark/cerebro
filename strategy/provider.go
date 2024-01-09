package strategy

import (
	"context"

	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
)

type CandleProvider interface {
	// GetCandle returns candles of code
	Candles(ctx context.Context, candleType market.CandleType) (indicator.Candles, error)
}

type provider struct {
	st  market.Market
	itm item.Item
}

func NewCandleProvider(st market.Market, itm item.Item) CandleProvider {
	return &provider{st: st, itm: itm}
}

func (p *provider) Candles(ctx context.Context, candleType market.CandleType) (indicator.Candles, error) {
	return p.st.Candles(ctx, p.itm.Code, candleType)
}
