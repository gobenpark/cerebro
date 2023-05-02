package store

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	kv "github.com/gobenpark/kinvest-go"
	"github.com/gobenpark/trader/container"
	"github.com/gobenpark/trader/internal/pkg"
	"github.com/gobenpark/trader/item"
	"github.com/gobenpark/trader/order"
	"github.com/gobenpark/trader/position"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

type kinvest struct {
	*kv.Kinvest
}

func NewKinvestStore() Store {
	c := &kv.Config{}
	cli := kv.NewKinvest(c)
	return &kinvest{cli}
}

func (k *kinvest) MarketItems(ctx context.Context) []item.Item {
	kcodes, err := k.Kinvest.CodeManager.Kosdaq(ctx)
	if err != nil {
		return nil
	}

	kpcodes, err := k.Kinvest.CodeManager.Kospi(ctx)
	if err != nil {
		return nil
	}

	var it []item.Item
	for i := range kcodes {
		it = append(it, item.Item{
			Code: kcodes[i].Code,
			Type: "kosdaq",
			Name: kcodes[i].Name,
			Tag:  kcodes[i].Industry,
		})
	}

	for i := range kpcodes {
		it = append(it, item.Item{
			Code: kpcodes[i].Code,
			Type: "kospi",
			Name: kpcodes[i].Name,
			Tag:  kpcodes[i].Industry,
		})
	}
	return it
}

func (k *kinvest) Candles(ctx context.Context, code string, level time.Duration) (container.Candles, error) {
	s := time.Now().Add(-(24 * 365 * time.Hour))
	p, err := k.DailyChartPrice(ctx, s, time.Now(), kv.Stock, kv.Day, code, true)
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

func (k *kinvest) TradeCommits(ctx context.Context, code string) ([]container.TradeHistory, error) {
	//TODO implement me
	panic("implement me")
}

func (k *kinvest) Tick(ctx context.Context, codes ...string) (<-chan container.Tick, error) {
	var once sync.Once
	approvalkey, err := k.ApprovalKey(ctx)
	if err != nil {
		return nil, err
	}

	ch := make(chan container.Tick, 1)

	for i := range codes {
		go func(code string) {

			res, err := k.RealtimeContract(ctx, approvalkey, code)
			if err != nil {
				return
			}
			for v := range pkg.OrDone(ctx, res) {
				for _, j := range v.Datas {
					timein := time.Now().Local().Add(time.Hour*time.Duration(j.GetContractHour().Hour()) +
						time.Minute*time.Duration(j.GetContractHour().Minute()) +
						time.Second*time.Duration(j.GetContractHour().Second()))

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

func (k *kinvest) Order(ctx context.Context, o order.Order) error {
	//TODO implement me
	panic("implement me")
}

func (k *kinvest) Cancel(id string) error {
	//TODO implement me
	panic("implement me")
}

func (k *kinvest) Uid() string {
	//TODO implement me
	panic("implement me")
}

func (k *kinvest) Cash() int64 {
	b, err := k.Kinvest.AccountBalance(context.Background())
	if err != nil {
		return 0
	}

	var total int64 = 0
	for _, i := range b.Output2 {
		v, err := strconv.Atoi(i.DncaTotAmt)
		if err != nil {
			continue
		}
		total += int64(v)
	}
	return total
}

func (k *kinvest) Commission() float64 {
	//TODO implement me
	panic("implement me")
}

func (k *kinvest) Positions() map[string]position.Position {
	//TODO implement me
	panic("implement me")
}

func (k *kinvest) OrderInfo(id string) (order.Order, error) {
	//TODO implement me
	panic("implement me")
}

func TestItems(t *testing.T) {
	st := NewKinvestStore()
	it := st.MarketItems(context.TODO())
	for _, i := range it {
		fmt.Println(i)
	}
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestCash(t *testing.T) {
	st := NewKinvestStore()
	fmt.Println(st.Cash())
}

func TestCandles(t *testing.T) {
	st := NewKinvestStore()
	cd, err := st.Candles(context.Background(), "005935", time.Second)
	require.NoError(t, err)
	for _, i := range cd {
		fmt.Println(i)
	}
}

func TestTick(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	fmt.Println(runtime.NumGoroutine())
	st := NewKinvestStore()
	tk, err := st.Tick(ctx, "005935", "005930")
	require.NoError(t, err)

	for i := range tk {
		fmt.Println(i)
		fmt.Println(runtime.NumGoroutine())
	}

	<-time.After(3 * time.Second)
	fmt.Println(runtime.NumGoroutine())
}
