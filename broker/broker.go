/*
 *  Copyright 2023 The Cerebro Authors
 *
 *  Licensed under the GNU General Public License v3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      <https:fsf.org/>
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */
package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/event"
	"github.com/gobenpark/cerebro/indicator"
	"github.com/gobenpark/cerebro/item"
	"github.com/gobenpark/cerebro/market"
	"github.com/gobenpark/cerebro/order"
	"github.com/gobenpark/cerebro/position"
	"github.com/gobenpark/cerebro/risk"
)

// Broker tracks cash and open orders. Cash accounting is exchange-authoritative:
// balance reflects the settled cash reported by the market, while open buy orders
// reserve cash so the broker never over-commits before settlement.
type Broker struct {
	EventEngine event.Broadcaster
	market      market.Market
	logger      *slog.Logger
	// orders holds open (unsettled) orders. An order leaves this set once it is
	// completed, canceled, or rejected, which releases its cash reservation.
	orders []order.Order
	// positions is the latest snapshot of account positions from the market.
	positions []position.Position
	// balance is the settled cash reported by the exchange. It is seeded from
	// AccountBalance() and updated from market.ChangeBalanceEvent.
	balance decimal.Decimal
	// lots tracks each strategy's open long position per code, rebuilt from the
	// attributed fills the broker observes. size is the held quantity and cost is
	// the price*size acquisition cost (commission excluded), so the average entry
	// is cost/size. It is the source of truth for realized PnL and reporting.
	lots map[string]map[string]*lot
	// realized accumulates each strategy's realized PnL (price gains/losses on
	// closed quantity); fees accumulates the commission it has paid. Net realized
	// performance is realized minus fees.
	realized map[string]decimal.Decimal
	fees     map[string]decimal.Decimal
	// filled tracks cumulative filled size per order id, so a partial-then-complete
	// sequence books each increment once. It is keyed off the immutable order Size,
	// not the order's remaining size, which a market adapter may mutate on the
	// shared order object before the broker observes the fill event.
	filled map[string]decimal.Decimal
	// submitted holds the ids of orders the market has been told about (market.Order
	// returned). cancelPending holds ids a strategy asked to cancel before submission
	// completed. Together they serialize Cancel with the async submit goroutine: a
	// cancel that arrives before the market knows the order is deferred and forwarded
	// once submit registers it, rather than lost as an unknown cancel.
	submitted     map[string]struct{}
	cancelPending map[string]struct{}
	// lastPrice is the most recent tick price per code. It is the final fallback for
	// valuing a market fill whose event and order both carry no price, so positions
	// (and the PnL/risk that read them) are still tracked for such adapters.
	lastPrice map[string]decimal.Decimal
	// now is the latest observed tick time, the clock the broker stamps trades and
	// equity samples with (sim time under replay, wall time live). Zero until the
	// first tick.
	now time.Time
	// trades is the closed round-trip log, appended when a lot returns to flat, the
	// basis for trade-level performance metrics (win rate, profit factor, ...).
	trades []Trade
	// equity is the account equity sampled once per calendar day (by tick time), the
	// basis for time-level metrics (max drawdown, Sharpe, total return).
	equity []EquityPoint
	// mu guards orders, positions, balance, and the PnL ledger (lots/realized/fees).
	mu sync.RWMutex
	// wg tracks in-flight submit goroutines so Wait can join them on shutdown.
	wg sync.WaitGroup
	// risk is the optional pre-trade gate consulted in Order; nil means no gate.
	risk *risk.Manager
	// store is the optional durable ledger persisted after each booked fill and
	// reloaded on Restore; nil means no persistence (existing behavior).
	store Storage
	// legacyLedger is set when Restore loaded a pre-fill-progress ledger (schema < 2):
	// b.filled could not be restored, so open-order recovery falls back to the exchange's
	// reported fill to avoid double-counting a partial the restored lots already include.
	legacyLedger bool
	// saveMu serializes ledger writes through store so overlapping saves cannot
	// interleave or move the persisted state backwards.
	saveMu sync.Mutex
	// orderTimeout, when > 0, bounds the market.Order call in submit; exceeding it
	// makes the submission in-doubt (kept open and reserved) rather than rejected.
	orderTimeout time.Duration
	// inDoubtHandler is invoked on its own goroutine when a submission ends in-doubt;
	// nil means the broker only logs it.
	inDoubtHandler func(o order.Order, err error)
}

// lot is one strategy's open position in one code. The round-trip accumulators below
// span the lot's life (first buy to flat) so a Trade can be emitted when it closes;
// they are distinct from size/cost, which track only the currently-held quantity.
type lot struct {
	item *item.Item
	size decimal.Decimal // held quantity
	cost decimal.Decimal // acquisition cost of the held quantity (price*size, no fees)
	peak decimal.Decimal // highest fill price since the lot opened (seeds trailing stops)

	openedAt    time.Time       // time of the first buy that opened this round-trip
	lastAt      time.Time       // time of the most recent fill (becomes Trade.ClosedAt)
	boughtQty   decimal.Decimal // total quantity bought over the round-trip
	boughtValue decimal.Decimal // total buy notional (price*size) — avg entry is value/qty
	soldQty     decimal.Decimal // total quantity sold over the round-trip
	soldValue   decimal.Decimal // total sell notional — avg exit is value/qty
	realized    decimal.Decimal // round-trip realized PnL (price gains on closed quantity)
	feesPaid    decimal.Decimal // round-trip commission (buys + sells)
}

// Trade is one completed round-trip: a position opened from flat and returned to
// flat. NetPnL is Realized minus Fees; a winning trade has NetPnL > 0.
type Trade struct {
	Strategy string
	Code     string
	Qty      decimal.Decimal // quantity traded (entry size)
	Entry    decimal.Decimal // average entry price
	Exit     decimal.Decimal // average exit price
	Realized decimal.Decimal // gross realized PnL over the round-trip
	Fees     decimal.Decimal // commission over the round-trip
	OpenedAt time.Time
	ClosedAt time.Time
}

// NetPnL is the round-trip profit after commission.
func (t Trade) NetPnL() decimal.Decimal { return t.Realized.Sub(t.Fees) }

// EquityPoint is account equity (settled cash plus the mark-to-market value of open
// positions) sampled at a point in time.
type EquityPoint struct {
	Time   time.Time
	Equity decimal.Decimal
}

// StrategyReport is a per-strategy view of trading performance at a point in time.
type StrategyReport struct {
	Strategy  string
	Realized  decimal.Decimal // realized PnL (price gains/losses on closed quantity)
	Fees      decimal.Decimal // cumulative commission paid
	Positions []position.Position
}

// SetRisk installs the pre-trade risk gate. It is set once at construction before
// any order flows, so it needs no synchronization.
func (b *Broker) SetRisk(rm *risk.Manager) { b.risk = rm }

// SetOrderTimeout bounds the market.Order call in submit. Set once at construction
// before any order flows, like SetRisk, so it needs no synchronization. Zero leaves
// the call unbounded (existing behavior).
func (b *Broker) SetOrderTimeout(d time.Duration) { b.orderTimeout = d }

// SetInDoubtHandler installs the callback run when a submission ends in-doubt (the
// market.Order call exceeded the order timeout). Set once at construction, like
// SetRisk.
func (b *Broker) SetInDoubtHandler(fn func(o order.Order, err error)) { b.inDoubtHandler = fn }

// snapshotLocked captures the account state the risk rules vet an order against.
// Callers must hold b.mu so the snapshot (including pending orders) is consistent
// with the reservation done in the same critical section.
func (b *Broker) snapshotLocked() risk.Snapshot {
	return risk.Snapshot{
		Balance:   b.balance,
		Available: b.balance.Sub(b.reservedLocked()),
		Realized:  b.totalRealizedLocked(),
		Positions: slices.Clone(b.positions),
		Open:      slices.Clone(b.orders),
	}
}

// recordFillLocked folds one fill into the per-strategy PnL ledger. Caller holds
// b.mu. A buy grows the lot; a sell realizes PnL on the closed quantity at the
// lot's average cost and shrinks (or clears) it. The commission Rate carries its
// own unit, so the fee is rate.Of(value).
func (b *Broker) recordFillLocked(strategy string, it *item.Item, action order.Action, size, price decimal.Decimal) {
	lots := b.lots[strategy]
	if lots == nil {
		lots = map[string]*lot{}
		b.lots[strategy] = lots
	}
	l := lots[it.Code]
	if l == nil {
		l = &lot{item: it, openedAt: b.now}
		lots[it.Code] = l
	}
	l.lastAt = b.now
	rate := b.market.Commission()

	switch action {
	case order.Buy:
		fee := rate.Of(price.Mul(size))
		b.fees[strategy] = b.fees[strategy].Add(fee)
		l.feesPaid = l.feesPaid.Add(fee)
		l.cost = l.cost.Add(price.Mul(size))
		l.size = l.size.Add(size)
		l.boughtQty = l.boughtQty.Add(size)
		l.boughtValue = l.boughtValue.Add(price.Mul(size))
		if price.GreaterThan(l.peak) {
			l.peak = price // track the high-water fill price for trailing stops
		}
	case order.Sell:
		// Never close more than is held (a defensive guard; the exchange/replay
		// should not report an oversell).
		sold := decimal.Min(size, l.size)
		if sold.GreaterThan(decimal.Zero) {
			avg := l.cost.Div(l.size)
			fee := rate.Of(price.Mul(sold))
			gain := price.Sub(avg).Mul(sold)
			b.fees[strategy] = b.fees[strategy].Add(fee)
			b.realized[strategy] = b.realized[strategy].Add(gain)
			l.feesPaid = l.feesPaid.Add(fee)
			l.realized = l.realized.Add(gain)
			l.soldQty = l.soldQty.Add(sold)
			l.soldValue = l.soldValue.Add(price.Mul(sold))
			l.cost = l.cost.Sub(avg.Mul(sold))
			l.size = l.size.Sub(sold)
		}
		if l.size.LessThanOrEqual(decimal.Zero) {
			b.trades = append(b.trades, tradeFromLot(strategy, l))
			delete(lots, it.Code)
			if len(lots) == 0 {
				delete(b.lots, strategy)
			}
		}
	default:
		// Cancel/Edit are not fills and do not move the position.
	}
}

// tradeFromLot builds the closed-trade record for a lot that has returned to flat.
func tradeFromLot(strategy string, l *lot) Trade {
	t := Trade{
		Strategy: strategy,
		Code:     l.item.Code,
		Qty:      l.boughtQty,
		Realized: l.realized,
		Fees:     l.feesPaid,
		OpenedAt: l.openedAt,
		ClosedAt: l.lastAt,
	}
	if l.boughtQty.GreaterThan(decimal.Zero) {
		t.Entry = l.boughtValue.Div(l.boughtQty)
	}
	if l.soldQty.GreaterThan(decimal.Zero) {
		t.Exit = l.soldValue.Div(l.soldQty)
	}
	return t
}

// totalRealizedLocked sums realized PnL across strategies. Caller holds b.mu.
func (b *Broker) totalRealizedLocked() decimal.Decimal {
	sum := decimal.Zero
	for _, v := range b.realized {
		sum = sum.Add(v)
	}
	return sum
}

// StrategyPosition returns a strategy's open position in a code (average entry as
// Position.Price), the high-water fill price since it opened, and ok=false if it
// holds none. It is the source of truth the reactive risk monitor evaluates exit
// policies against; the fill high lets a trailing stop account for scale-ins above
// the average entry that no observed tick may have captured.
func (b *Broker) StrategyPosition(strategy, code string) (position.Position, decimal.Decimal, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	l := b.lots[strategy][code]
	if l == nil || l.size.LessThanOrEqual(decimal.Zero) {
		return position.Position{}, decimal.Zero, false
	}
	return position.Position{Item: l.item, Size: l.size, Price: l.cost.Div(l.size)}, l.peak, true
}

// RealizedPnL returns the realized PnL accumulated by one strategy.
func (b *Broker) RealizedPnL(strategy string) decimal.Decimal {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.realized[strategy]
}

// TotalRealizedPnL returns realized PnL across every strategy.
func (b *Broker) TotalRealizedPnL() decimal.Decimal {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.totalRealizedLocked()
}

// Report returns a per-strategy snapshot of realized PnL, fees, and open
// positions, sorted by strategy name for stable output.
func (b *Broker) Report() []StrategyReport {
	b.mu.RLock()
	defer b.mu.RUnlock()

	names := map[string]struct{}{}
	for s := range b.realized {
		names[s] = struct{}{}
	}
	for s := range b.fees {
		names[s] = struct{}{}
	}
	for s := range b.lots {
		names[s] = struct{}{}
	}

	out := make([]StrategyReport, 0, len(names))
	for s := range names {
		rep := StrategyReport{Strategy: s, Realized: b.realized[s], Fees: b.fees[s]}
		for _, l := range b.lots[s] {
			avg := decimal.Zero
			if l.size.GreaterThan(decimal.Zero) {
				avg = l.cost.Div(l.size)
			}
			rep.Positions = append(rep.Positions, position.Position{Item: l.item, Size: l.size, Price: avg})
		}
		sort.Slice(rep.Positions, func(i, j int) bool {
			return rep.Positions[i].Item.Code < rep.Positions[j].Item.Code
		})
		out = append(out, rep)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Strategy < out[j].Strategy })
	return out
}

// equityLocked is settled cash plus the mark-to-market value of every open lot,
// valued at the latest tick price (falling back to average entry when a held code
// has no observed price yet). Caller holds b.mu.
func (b *Broker) equityLocked() decimal.Decimal {
	eq := b.balance
	for _, lots := range b.lots {
		for code, l := range lots {
			if l.size.LessThanOrEqual(decimal.Zero) {
				continue
			}
			px, ok := b.lastPrice[code]
			if !ok {
				px = l.cost.Div(l.size) // no tick yet: mark at entry (unrealized 0)
			}
			eq = eq.Add(px.Mul(l.size))
		}
	}
	return eq
}

// sampleEquityLocked keeps one equity point per calendar day (in tick time): it
// appends a point on a new day and otherwise updates the current day's point to the
// latest equity, so each completed day holds that day's closing equity (reflecting
// intraday price moves and fills) rather than its open. Caller holds b.mu.
func (b *Broker) sampleEquityLocked(now time.Time) {
	eq := b.equityLocked()
	if n := len(b.equity); n > 0 && sameDay(b.equity[n-1].Time, now) {
		b.equity[n-1] = EquityPoint{Time: now, Equity: eq}
		return
	}
	b.equity = append(b.equity, EquityPoint{Time: now, Equity: eq})
}

// sameDay reports whether a and b fall on the same UTC calendar day.
func sameDay(a, c time.Time) bool {
	ay, am, ad := a.UTC().Date()
	cy, cm, cd := c.UTC().Date()
	return ay == cy && am == cm && ad == cd
}

// Equity returns the current account equity (settled cash plus mark-to-market open
// positions).
func (b *Broker) Equity() decimal.Decimal {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.equityLocked()
}

// Trades returns a copy of the closed round-trip log, oldest first.
func (b *Broker) Trades() []Trade {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return slices.Clone(b.trades)
}

// EquityCurve returns the daily-sampled equity series, oldest first, with the final
// (current) day reflecting equity as of now — including fills booked since the last
// tick — so the curve always ends at the live equity.
func (b *Broker) EquityCurve() []EquityPoint {
	b.mu.RLock()
	defer b.mu.RUnlock()
	curve := slices.Clone(b.equity)
	if b.now.IsZero() {
		return curve
	}
	cur := b.equityLocked()
	if n := len(curve); n > 0 && sameDay(curve[n-1].Time, b.now) {
		curve[n-1] = EquityPoint{Time: b.now, Equity: cur}
	} else {
		curve = append(curve, EquityPoint{Time: b.now, Equity: cur})
	}
	return curve
}

// Submitter is the broker surface a strategy uses. A scoped Submitter tags every
// order with the strategy's name so fills can be attributed back to it, and Cancel
// only acts on orders that strategy placed.
type Submitter interface {
	Order(ctx context.Context, o order.Order, safe bool) error
	// Cancel requests cancellation of one of this strategy's open orders by id. It
	// returns an error if no such open order is owned by the strategy; the order's
	// cash reservation is released when the exchange confirms the cancellation.
	Cancel(ctx context.Context, orderID string) error
	Available() decimal.Decimal
	Balance() decimal.Decimal
	Position(ticker string) (position.Position, bool)
	Orders(code string) []order.Order
}

var (
	_ Submitter = (*Broker)(nil)
	_ risk.Book = (*Broker)(nil)
)

// Scoped returns a Submitter that stamps strategy onto every order before
// submitting it through the broker.
func (b *Broker) Scoped(strategy string) Submitter {
	return &scopedBroker{Broker: b, strategy: strategy}
}

type scopedBroker struct {
	*Broker
	strategy string
}

func (s *scopedBroker) Order(ctx context.Context, o order.Order, safe bool) error {
	o.SetStrategy(s.strategy)
	return s.Broker.Order(ctx, o, safe)
}

// Cancel cancels one of this strategy's own open orders; an order placed by another
// strategy is not cancelable through this scoped handle.
func (s *scopedBroker) Cancel(ctx context.Context, orderID string) error {
	return s.cancel(ctx, orderID, s.strategy)
}

func NewDefaultBroker(eventEngine event.Broadcaster, store market.Market, logger *slog.Logger) *Broker {
	return &Broker{
		orders:      []order.Order{},
		EventEngine: eventEngine,
		market:      store,
		logger:      logger,
		// Seed settled cash and positions from the exchange at startup. There is no
		// request context here, so use Background; the adapter must keep these reads
		// time-bounded.
		positions:     store.AccountPositions(context.Background()),
		balance:       store.AccountBalance(context.Background()),
		lots:          map[string]map[string]*lot{},
		realized:      map[string]decimal.Decimal{},
		fees:          map[string]decimal.Decimal{},
		filled:        map[string]decimal.Decimal{},
		lastPrice:     map[string]decimal.Decimal{},
		submitted:     map[string]struct{}{},
		cancelPending: map[string]struct{}{},
	}
}

// orderValue returns the cash an open order still commits, including commission.
// It is based on the unfilled remainder (RemainPrice), so a partial fill releases
// the reservation for the portion that has already settled. For a fresh order the
// remainder equals the full size. The commission Rate carries its own unit, so
// the fee is Commission().Of(value).
func (b *Broker) orderValue(o order.Order) decimal.Decimal {
	value := o.RemainPrice()
	fee := b.market.Commission().Of(value)
	return value.Add(fee)
}

// isTerminalStatus reports whether a status ends an order's lifecycle, meaning
// the order should leave the open set and release any cash it reserved.
func isTerminalStatus(s order.Status) bool {
	switch s {
	case order.Completed, order.Canceled, order.Expired, order.Margin, order.Rejected:
		return true
	default:
		return false
	}
}

// reservedLocked returns the cash committed by open buy orders.
// Callers must hold b.mu (read or write).
func (b *Broker) reservedLocked() decimal.Decimal {
	reserved := decimal.Zero
	for i := range b.orders {
		if b.orders[i].Action() == order.Buy {
			reserved = reserved.Add(b.orderValue(b.orders[i]))
		}
	}
	return reserved
}

// Balance returns the settled cash reported by the exchange.
func (b *Broker) Balance() decimal.Decimal {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.balance
}

// Available returns the cash available for new buy orders:
// settled balance minus cash reserved by open buy orders.
func (b *Broker) Available() decimal.Decimal {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.balance.Sub(b.reservedLocked())
}

func (b *Broker) Order(ctx context.Context, o order.Order, safe bool) error {
	// Pre-trade validation: return the error to the caller as the single handling
	// point — the broker does not also log it (that would duplicate it in aggregators).
	if o.Type() == order.Market && !o.Price().IsZero() {
		return ErrMarketOrderPrice
	}
	if o.Type() == order.Limit && o.Size().IsZero() {
		return ErrOrderSizeIsZero
	}
	if o.Type() == order.Limit && o.Price().IsZero() {
		return ErrPriceIsZero
	}

	b.mu.Lock()
	// Pre-trade risk gate: vet against the current account state — including
	// pending orders — atomically with the reservation below, before submitting.
	if b.risk != nil {
		if err := b.risk.Check(o, b.snapshotLocked()); err != nil {
			b.mu.Unlock()
			b.logger.Info("order rejected by risk gate", "code", o.Item().Code, "error", err)
			return err
		}
	}
	if safe {
		if slices.ContainsFunc(b.orders, func(od order.Order) bool {
			return od.Item().Code == o.Item().Code
		}) {
			b.mu.Unlock()
			return errors.New("waiting for conclusion")
		}
	}

	// Reserve cash for buy orders against available (settled minus reserved) cash.
	if o.Action() == order.Buy {
		if value := b.orderValue(o); value.GreaterThan(b.balance.Sub(b.reservedLocked())) {
			b.mu.Unlock()
			return ErrNotEnoughMoney
		}
	}

	b.orders = append(b.orders, o)
	b.mu.Unlock()

	b.wg.Go(func() {
		b.submit(ctx, o)
	})
	return nil
}

// Cancel requests cancellation of the open order with orderID, regardless of which
// strategy placed it. The reservation is released when the exchange confirms with a
// Canceled event (see applyOrderChange), so live and replay share one path.
func (b *Broker) Cancel(ctx context.Context, orderID string) error {
	return b.cancel(ctx, orderID, "")
}

// cancel finds the open order by id and asks the market to cancel it. owner, when
// non-empty, restricts cancellation to orders that strategy placed, so a scoped
// Submitter cannot cancel another strategy's order.
func (b *Broker) cancel(ctx context.Context, orderID, owner string) error {
	b.mu.Lock()
	var target order.Order
	for _, o := range b.orders {
		if o.ID() == orderID {
			target = o
			break
		}
	}
	if target == nil {
		b.mu.Unlock()
		return fmt.Errorf("cancel: no open order %q", orderID)
	}
	if owner != "" && target.Strategy() != owner {
		b.mu.Unlock()
		return fmt.Errorf("cancel: order %q not owned by %q", orderID, owner)
	}
	if _, ok := b.submitted[orderID]; !ok {
		// The order has not reached the market yet (submit is still in flight). Defer
		// the cancel so submit forwards it once registered, instead of racing ahead of
		// the order and being dropped as an unknown cancel.
		b.cancelPending[orderID] = struct{}{}
		b.mu.Unlock()
		return nil
	}
	b.mu.Unlock()
	return b.market.Cancel(ctx, target)
}

// ReconcileOpenOrders recovers the orders the exchange still has working into the
// broker's open set, so a process that restarts mid-order does not forget them. Each
// recovered order has its cash reserved again — against its unfilled remainder, since
// orderValue is based on RemainingSize, so an order that partially filled while the
// process was down reserves only what is left — and is marked submitted (the exchange
// already knows it). Without this, those orders' fill/cancel events would arrive as
// unknown and their reserved cash would be silently double-spendable.
//
// It is a no-op when the market adapter does not implement market.OpenOrderReporter
// (e.g. a backtest), so cold-started simulations are unaffected. The open-order
// bookkeeping is replaced wholesale from the exchange snapshot, which makes it
// idempotent: a retried Start cannot double-count. The durable per-strategy PnL
// ledger (Restore) is independent and left untouched.
//
// Attribution matters here: a recovered order's later fill must update the right
// strategy's lot, or a restored position can be left open after its exit order fills —
// the broker would still think the strategy holds it and could fire a duplicate exit.
// The exchange does not record the strategy. An attribution the adapter set (recovered
// from a client id) always wins. Otherwise the broker attributes only an unattributed
// SELL to the sole strategy holding a lot in its code — safe in a long-only book, where
// just the holder can sell — so a recovered exit closes the right position. A buy is
// left unattributed (holding a code does not prove the holder placed a buy on it), as
// is a sell on a code several strategies hold; the adapter should attribute those from
// the client id. A
// partial's already-booked quantity is carried over from the restored ledger (the fill
// progress the broker actually observed), so a completion books only what was not yet
// booked: a fill that landed offline — never observed, so absent — is still counted,
// while a persisted partial is not double-counted. Run after Restore so both the
// restored lots (for attribution) and the restored fill progress are available.
//
// It is a no-op when the market adapter does not implement market.OpenOrderReporter
// (e.g. a backtest). The open-order bookkeeping is replaced wholesale from the exchange
// snapshot, which makes it idempotent: a retried Start cannot double-count.
func (b *Broker) ReconcileOpenOrders(ctx context.Context) error {
	reporter, ok := b.market.(market.OpenOrderReporter)
	if !ok {
		return nil
	}
	orders, err := reporter.OpenOrders(ctx)
	if err != nil {
		return fmt.Errorf("reconcile open orders: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	open := make([]order.Order, 0, len(orders))
	submitted := make(map[string]struct{}, len(orders))
	filled := make(map[string]decimal.Decimal, len(orders))
	for _, o := range orders {
		// Defensive: only resting orders belong in the open set. A terminal one would
		// never receive a follow-up event to release its (here, reserved) cash.
		if o == nil || isTerminalStatus(o.Status()) {
			continue
		}
		// Reflect that the order is working on the exchange in its public status: callers
		// of Orders() filter on it, so a recovered live order must not read as Created. A
		// partial already reads Partial (its fills were applied on construction); advance
		// a not-yet-acknowledged order to Accepted.
		if s := o.Status(); s == order.Created || s == order.Submitted {
			o.Accept()
		}
		// Attribute an unattributed SELL to the sole strategy holding its code, so its
		// fill closes the right position — a recovered exit must not leave a phantom open
		// lot that then gets re-exited. In a long-only book only the holder can sell, so
		// this inference is safe. A buy is left unattributed: holding a code does not
		// prove the holder placed a buy on it (another strategy may be opening its own
		// position there), and mis-crediting would contaminate that lot. The adapter
		// should attribute buys — and any ambiguous order — from the client id.
		if o.Strategy() == "" && o.Action() == order.Sell {
			if owner, ok := b.soleLotOwnerLocked(o.Item().Code); ok {
				o.SetStrategy(owner)
			}
		}
		open = append(open, o)
		submitted[o.ID()] = struct{}{}
		if done, ok := b.recoveredFillProgressLocked(o); ok {
			filled[o.ID()] = done
		}
	}

	b.orders = open
	b.submitted = submitted
	b.filled = filled
	b.cancelPending = map[string]struct{}{}
	return nil
}

// recoveredFillProgressLocked returns the quantity of a recovered order already booked
// into the ledger, so a completion books only the rest. For a current-schema ledger (or
// no storage) this is the progress the broker actually observed, restored into b.filled
// — a partial that filled offline has no entry, so it books fully on completion, while a
// persisted partial keeps its entry and is not re-counted. It guards the Size()=original
// contract: progress exceeding Size() means the adapter reported Size() as the remainder,
// which would make the increment negative and book nothing, so it is dropped (with a
// warn) to at least book the reported remainder.
//
// For a legacy ledger (schema < 2, no persisted progress) b.filled is empty even though
// the restored lots may already include a partial; there it falls back to the exchange's
// reported fill (Size-RemainingSize), assuming the lots account for it, to avoid
// double-counting — an offline fill may then be undercounted, which is unavoidable for
// the old schema and safer than double-counting. Caller holds b.mu.
func (b *Broker) recoveredFillProgressLocked(o order.Order) (decimal.Decimal, bool) {
	if b.legacyLedger {
		done := o.Size().Sub(o.RemainingSize())
		return done, done.GreaterThan(decimal.Zero)
	}
	done, ok := b.filled[o.ID()]
	if !ok || done.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, false
	}
	if done.GreaterThan(o.Size()) {
		b.logger.Warn("recovered order fill progress exceeds its size; dropping it (adapter must report the original Size, not the remainder)",
			"order", o.ID(), "size", o.Size().String(), "observed", done.String())
		return decimal.Zero, false
	}
	return done, true
}

// soleLotOwnerLocked returns the only strategy holding an open lot in code, if exactly
// one does — the basis for attributing an unattributed recovered order to the position
// it must belong to. Returns ok=false when no strategy or more than one holds the code.
// Caller holds b.mu.
func (b *Broker) soleLotOwnerLocked(code string) (string, bool) {
	owner := ""
	for strategy, lots := range b.lots {
		if l, ok := lots[code]; ok && l.size.GreaterThan(decimal.Zero) {
			if owner != "" {
				return "", false // ambiguous: more than one strategy holds this code
			}
			owner = strategy
		}
	}
	return owner, owner != ""
}

// Wait blocks until all in-flight order submissions have completed. Callers must
// keep the event dispatcher alive until Wait returns, since submit broadcasts.
func (b *Broker) Wait() {
	b.wg.Wait()
}

// submitToMarket sends the order to the market. Without an order timeout it is a plain
// synchronous call (existing behavior). With one, it time-boxes the call: the adapter
// runs on a side goroutine and, when octx ends first — its deadline trips or the
// parent context is canceled — this returns octx.Err() without waiting for the adapter
// to return. That guards the genuinely hung case — an adapter or HTTP client that does
// not honor context cancellation — so a stuck call can never pin the submit goroutine
// (and Wait) forever. Because the detached call may still reach the exchange, the
// caller treats any octx-ended return as in-doubt (it does not free the reservation).
// The side goroutine keeps running until the adapter finally returns; the buffered
// channel lets it exit without leaking once it does, and its (unread) result is
// harmless because the submission has already been resolved in-doubt.
func (b *Broker) submitToMarket(octx context.Context, o order.Order) error {
	if b.orderTimeout == 0 {
		return b.market.Order(octx, o)
	}
	done := make(chan error, 1)
	go func() { done <- b.market.Order(octx, o) }()
	select {
	case err := <-done:
		return err
	case <-octx.Done():
		return octx.Err()
	}
}

// submit sends the order to the market in a goroutine. Cash accounting follows
// the exchange: balance is updated from ChangeBalanceEvent and an order's
// reservation is released once it leaves b.orders (complete/cancel/reject).
func (b *Broker) submit(ctx context.Context, o order.Order) {
	// If the run is already shutting down before we submit, the order never reached the
	// exchange, so reject it outright rather than sending it. This also keeps the
	// in-doubt path honest: that path is for a call that may have actually gone out, not
	// for one we never started because the context was already dead.
	if ctx.Err() != nil {
		b.rejectOrder(ctx, o, ctx.Err())
		return
	}

	o.Submit()
	b.notifyOrder(ctx, o.Copy())

	// Optionally bound the adapter's Order call so a hung exchange API cannot pin a
	// submission open forever. The bounded context governs only the call; the order's
	// own lifetime still follows the parent ctx.
	octx := ctx
	if b.orderTimeout > 0 {
		var cancel context.CancelFunc
		octx, cancel = context.WithTimeout(ctx, b.orderTimeout)
		defer cancel()
	}

	if err := b.submitToMarket(octx, o); err != nil {
		// In-doubt, not a rejection: if our own context ended the call — the order
		// timeout deadline, OR a parent-context cancellation (shutdown) — the detached
		// adapter call may still be reaching the exchange, so the order's fate is
		// unknown. Rejecting it would release the cash reservation while the exchange
		// might still be working the order — a double-exposure risk. Keep it open and
		// reserved, mark it submitted so a later cancel can target it, and hand it to
		// the operator to resolve. It stays Submitted (already broadcast above), which
		// is honest: it may well be live. Only an error returned while our context is
		// still live is a true adapter rejection (the exchange did not get it), so that
		// alone rejects.
		if b.orderTimeout > 0 && octx.Err() != nil {
			cancelNow := b.registerSubmitted(o)
			b.logger.Warn("order submit in doubt", "order", o.ID(), "code", o.Item().Code, "error", err)
			if b.inDoubtHandler != nil {
				go b.inDoubtHandler(o.Copy(), err)
			}
			// A cancel deferred while this submit was in flight must still be honored:
			// in-doubt means the order may be live on the exchange — exactly when a
			// cancel matters most. market.Cancel is a no-op if the order never arrived,
			// so forwarding is safe whichever way the in-doubt resolves.
			if cancelNow {
				b.forwardDeferredCancel(ctx, o)
			}
			return
		}
		b.rejectOrder(ctx, o, err)
		return
	}

	// The market now knows the order: register it submitted and forward any cancel that
	// was deferred while the submit was in flight (a cancel right after Order), which
	// would otherwise be lost as unknown.
	if b.registerSubmitted(o) {
		b.forwardDeferredCancel(ctx, o)
	}
}

// forwardDeferredCancel sends a cancel that was deferred while the order's submit was in
// flight (registerSubmitted has cleared cancelPending, so the broker now owns it).
//
// With an order timeout configured, it forwards on a context that is non-canceled but
// bounded — context.WithoutCancel stripped of the run's cancellation so a cancel the
// caller already requested is still sent during shutdown, then re-bounded by the same
// order timeout so a slow/hung adapter Cancel cannot pin the submit goroutine (and
// Wait) once cancellation is gone. Without an order timeout there is no in-doubt path
// and no configured bound, so it forwards on the caller context unchanged (the prior
// behavior). Best-effort either way: market.Cancel is a no-op if the order never
// reached the exchange.
func (b *Broker) forwardDeferredCancel(ctx context.Context, o order.Order) {
	cctx := ctx
	if b.orderTimeout > 0 {
		var cancel context.CancelFunc
		cctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), b.orderTimeout)
		defer cancel()
	}
	if err := b.market.Cancel(cctx, o); err != nil {
		b.logger.Info("forward deferred cancel", "order", o.ID(), "error", err)
	}
}

func (b *Broker) notifyOrder(ctx context.Context, o order.Order) {
	// Drop the notification if shutdown is underway; the dispatcher may be draining.
	b.EventEngine.BroadCastContext(ctx, o)
}

// rejectOrder marks o rejected, drops it from the open set (releasing its cash
// reservation), and notifies listeners. It is the terminal path for an order the
// exchange definitively did not get — an outright adapter error, or a submit attempted
// after the run was already canceled.
func (b *Broker) rejectOrder(ctx context.Context, o order.Order, err error) {
	b.logger.Info("reject order", "order", o, "error", err)
	o.Reject()
	b.removeOrder(o) // also clears any pending cancel for this id
	b.notifyOrder(ctx, o.Copy())
}

// registerSubmitted records that the market has been told about an order — or may
// have been, on an in-doubt timeout — and reports whether a cancel was deferred while
// the submission was in flight, so the caller can forward it now. It marks the id
// submitted only while the order is still open: a terminal event (a fast fill/reject)
// may have already removed and cleared the id, and re-adding it would strand a stale
// submitted id that a later order reusing it could trip over. It always clears any
// deferred cancel for the id (the caller is now responsible for forwarding it). Both
// the success and the in-doubt path share this, so a cancel queued during submission
// is forwarded in either outcome, never silently dropped.
func (b *Broker) registerSubmitted(o order.Order) (cancelPending bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if slices.ContainsFunc(b.orders, func(od order.Order) bool { return od.ID() == o.ID() }) {
		b.submitted[o.ID()] = struct{}{}
		_, cancelPending = b.cancelPending[o.ID()]
	}
	delete(b.cancelPending, o.ID())
	return cancelPending
}

// removeOrder drops an order from the open set, releasing its cash reservation.
func (b *Broker) removeOrder(o order.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.orders = slices.DeleteFunc(b.orders, func(od order.Order) bool {
		return od.ID() == o.ID()
	})
	delete(b.submitted, o.ID())
	delete(b.cancelPending, o.ID())
}

func (b *Broker) Orders(code string) []order.Order {
	b.mu.RLock()
	defer b.mu.RUnlock()

	orders := []order.Order{}
	for i := range b.orders {
		if b.orders[i].Item().Code == code {
			orders = append(orders, b.orders[i])
		}
	}
	return orders
}

func (b *Broker) Position(ticker string) (position.Position, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return lo.Find(b.positions, func(item position.Position) bool {
		return item.Item.Code == ticker
	})
}

// refreshPositions pulls the latest positions from the market. The market call
// runs outside the lock to keep the critical section short. It uses WithoutCancel
// so a shutdown-canceled listener context cannot abort the read and overwrite the
// authoritative snapshot with an empty/stale result; a market adapter must bound
// the call with its own timeout.
func (b *Broker) refreshPositions(ctx context.Context) {
	positions := b.market.AccountPositions(context.WithoutCancel(ctx))
	b.mu.Lock()
	b.positions = positions
	b.mu.Unlock()
}

func (b *Broker) Listen(ctx context.Context, e any) {
	switch evt := e.(type) {
	case order.Order:
		if evt.Status() == order.Submitted {
			b.refreshPositions(ctx)
		}
	case indicator.Tick:
		// Remember the latest price per code so a market fill that carries no price
		// can still be valued (see applyOrderChange), advance the broker clock, and
		// sample equity once per day for the performance time series.
		b.mu.Lock()
		b.lastPrice[evt.Code] = evt.Price
		if evt.Date.After(b.now) {
			b.now = evt.Date
		}
		if !b.now.IsZero() {
			b.sampleEquityLocked(b.now)
		}
		b.mu.Unlock()
	case market.MarketEvent:
		b.handleMarketEvent(ctx, evt)
	}
}

func (b *Broker) handleMarketEvent(ctx context.Context, m market.MarketEvent) {
	switch evt := m.(type) {
	case market.ChangeOrderEvent:
		b.logger.Info("market change order", "message", evt.Message, "id", evt.ID, "action", evt.Action)
		b.applyOrderChange(ctx, evt)
	case market.ChangeBalanceEvent:
		b.logger.Info("market change balance", "message", evt.Message, "balance", evt.Balance)
		b.mu.Lock()
		b.balance = evt.Balance
		b.mu.Unlock()
	}
}

// applyOrderChange updates an open order from an exchange event. Completed and
// canceled orders leave the open set, releasing their cash reservation.
func (b *Broker) applyOrderChange(ctx context.Context, evt market.ChangeOrderEvent) {
	// Pull the latest positions outside the lock (the market call is slow), then
	// apply the order-state change and the position refresh under one lock. Doing
	// both atomically means a fill's exposure is never momentarily absent from
	// both the open set and positions, which would let the risk gate under-count
	// it if a strategy places an order in reaction to the fill notification.
	//
	// WithoutCancel: this refresh is the authoritative half of completing a fill, so
	// a shutdown-canceled listener context must not abort it and leave b.positions
	// overwritten with an empty/stale snapshot while the order is still completed.
	// The adapter is responsible for bounding the call with its own timeout.
	positions := b.market.AccountPositions(context.WithoutCancel(ctx))

	b.mu.Lock()
	idx := slices.IndexFunc(b.orders, func(od order.Order) bool {
		return od.ID() == evt.ID
	})
	if idx < 0 {
		b.mu.Unlock()
		return
	}

	o := b.orders[idx]
	// Capture the size newly filled by this event before mutating the order, so the
	// PnL ledger counts each increment exactly once (a partial then a completion add
	// up to the full size).
	var fillInc decimal.Decimal
	switch evt.Action {
	case order.Accepted:
		o.Accept()
	case order.Completed:
		// Completion fills whatever remains; derive it from the immutable order size
		// minus what already filled, not from the (possibly externally mutated)
		// remaining size.
		fillInc = o.Size().Sub(b.filled[evt.ID])
		o.Complete()
	case order.Canceled:
		o.Cancel()
	case order.Expired:
		o.Expire()
	case order.Margin:
		o.Margin()
	case order.Rejected:
		o.Reject()
	case order.Partial:
		// A partial fill reduces the remaining size; the order stays open and its
		// reservation shrinks to the unfilled remainder (see orderValue).
		fillInc = evt.FilledSize
		b.filled[evt.ID] = b.filled[evt.ID].Add(evt.FilledSize)
		o.Partial(evt.FilledSize)
	default:
		// None/Created/Submitted are not delivered as exchange events; nothing to apply.
	}
	// Fold the fill into the per-strategy PnL ledger. The exchange's fill price is
	// preferred; for limit orders the order's own price is an exact fallback.
	if fillInc.GreaterThan(decimal.Zero) {
		price := evt.Price
		if price.IsZero() {
			price = o.Price() // limit orders carry an exact price
		}
		if price.IsZero() {
			price = b.lastPrice[o.Item().Code] // market fill with no reported price
		}
		if price.GreaterThan(decimal.Zero) {
			b.recordFillLocked(o.Strategy(), o.Item(), o.Action(), fillInc, price)
		}
	}
	// Terminal statuses end the order's lifecycle: drop it from the open set so
	// its reserved cash is released. Non-terminal updates (e.g. Accepted) keep
	// the order open and its reservation in place.
	if isTerminalStatus(evt.Action) {
		b.orders = slices.Delete(b.orders, idx, idx+1)
		delete(b.filled, evt.ID)
		delete(b.submitted, evt.ID)
		delete(b.cancelPending, evt.ID)
	}
	b.positions = positions
	// Only a booked fill moves the durable ledger (lots/realized/fees); other
	// order transitions touch state that is re-derived from the exchange on start.
	ledgerChanged := fillInc.GreaterThan(decimal.Zero)
	b.mu.Unlock()

	// Persist after releasing the lock so the (possibly slow) write never holds up
	// other broker operations. Fills are processed serially in the broker's single
	// listener goroutine, so writes stay ordered.
	if ledgerChanged {
		b.persist(ctx)
	}

	b.logger.Info("order changed", "id", o.ID(), "code", o.Item().Code, "action", evt.Action)
	b.notifyOrder(ctx, o.Copy())
}
