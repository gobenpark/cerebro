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
	"fmt"
	"maps"
	"sort"

	"github.com/shopspring/decimal"

	"github.com/gobenpark/cerebro/item"
)

// ledgerVersion is the on-disk schema version of a Ledger. Bump it whenever the
// shape of the persisted state changes so a Storage can migrate or reject an
// incompatible snapshot instead of silently misreading it.
const ledgerVersion = 1

// Storage persists and restores the broker's durable per-strategy ledger so that
// realized PnL, fees, and open lots survive a process restart. It captures only
// the state the exchange does not know about: cash balance and account positions
// are exchange-authoritative and re-fetched on start, so they are deliberately
// left out.
//
// Save is called on the broker's fill-processing path after each booked fill, so
// an implementation should be reasonably fast (or buffer/coalesce writes). Load
// is called once, before any event is processed, and must report a fresh start
// (a zero-value Ledger, Version 0) when nothing has been persisted yet.
//
// In-flight orders and their partial-fill progress are NOT part of the ledger:
// the market interface exposes no open-order query to reconcile them against on
// restart, so a restart mid-order may miss that order's remaining fills. Only the
// settled per-strategy attribution ledger is durable in this version.
type Storage interface {
	Save(ctx context.Context, l Ledger) error
	Load(ctx context.Context) (Ledger, error)
}

// Ledger is the broker's durable, exchange-unknown state at a point in time: the
// per-strategy realized PnL, fees, and open lots rebuilt from attributed fills.
// It is the unit a Storage writes and reads.
type Ledger struct {
	// Version is the schema version the ledger was written with. A zero value
	// means "nothing persisted" (a fresh start), which Restore treats as a no-op.
	Version  int                        `json:"version"`
	Lots     []LotState                 `json:"lots"`
	Realized map[string]decimal.Decimal `json:"realized"`
	Fees     map[string]decimal.Decimal `json:"fees"`
}

// LotState is one strategy's open position in one code, flattened for storage.
// Size is the held quantity, Cost the acquisition cost of that quantity (no
// fees, so the average entry is Cost/Size), and Peak the highest fill price since
// the lot opened (it re-seeds a trailing stop after a restart).
type LotState struct {
	Strategy string          `json:"strategy"`
	Item     *item.Item      `json:"item"`
	Size     decimal.Decimal `json:"size"`
	Cost     decimal.Decimal `json:"cost"`
	Peak     decimal.Decimal `json:"peak"`
}

// SetStorage installs the durable ledger store. Like SetRisk it is set once at
// construction, before any order or fill flows, so it needs no synchronization.
func (b *Broker) SetStorage(s Storage) { b.store = s }

// Restore loads the persisted ledger (if any) into the broker before it starts
// processing events, so realized PnL, fees, and open lots carry over a restart.
// It is a no-op when no storage is configured or nothing has been persisted yet.
// Cash balance and positions are not restored here: NewDefaultBroker already
// seeds them from the exchange, which is authoritative across a restart.
func (b *Broker) Restore(ctx context.Context) error {
	if b.store == nil {
		return nil
	}
	l, err := b.store.Load(ctx)
	if err != nil {
		return fmt.Errorf("restore ledger: %w", err)
	}
	if l.Version == 0 {
		return nil // nothing persisted yet — fresh start
	}

	lots := map[string]map[string]*lot{}
	for _, ls := range l.Lots {
		if ls.Item == nil || ls.Size.LessThanOrEqual(decimal.Zero) {
			continue // skip a flat or malformed lot
		}
		codes := lots[ls.Strategy]
		if codes == nil {
			codes = map[string]*lot{}
			lots[ls.Strategy] = codes
		}
		// Seed the round-trip accumulators from the restored position so a later close
		// still emits a sane Trade. The pre-restart entry detail (open time, buy-side
		// fees) is not persisted, so a trade spanning a restart records exact
		// quantity/entry/realized but an approximate fee total and a zero open time.
		codes[ls.Item.Code] = &lot{
			item: ls.Item, size: ls.Size, cost: ls.Cost, peak: ls.Peak,
			boughtQty: ls.Size, boughtValue: ls.Cost,
		}
	}

	b.mu.Lock()
	b.lots = lots
	b.realized = cloneDecMap(l.Realized)
	b.fees = cloneDecMap(l.Fees)
	b.mu.Unlock()
	return nil
}

// snapshotLedger captures the durable ledger under the read lock. The returned
// Ledger shares no maps or slices with the broker, so it is safe to hand to a
// Storage that retains it.
func (b *Broker) snapshotLedger() Ledger {
	b.mu.RLock()
	defer b.mu.RUnlock()

	lots := make([]LotState, 0)
	for strategy, codes := range b.lots {
		for _, l := range codes {
			lots = append(lots, LotState{
				Strategy: strategy,
				Item:     l.item,
				Size:     l.size,
				Cost:     l.cost,
				Peak:     l.peak,
			})
		}
	}
	// Stable order (by strategy, then code) keeps the persisted file deterministic.
	sort.Slice(lots, func(i, j int) bool {
		if lots[i].Strategy != lots[j].Strategy {
			return lots[i].Strategy < lots[j].Strategy
		}
		return lots[i].Item.Code < lots[j].Item.Code
	})

	return Ledger{
		Version:  ledgerVersion,
		Lots:     lots,
		Realized: cloneDecMap(b.realized),
		Fees:     cloneDecMap(b.fees),
	}
}

// persist writes the current ledger through the configured Storage. It is a
// no-op without one. saveMu serializes writers so two overlapping saves cannot
// interleave; because each save re-reads the current full ledger (rather than a
// delta captured earlier), the persisted state never moves backwards.
func (b *Broker) persist(ctx context.Context) {
	if b.store == nil {
		return
	}
	b.saveMu.Lock()
	defer b.saveMu.Unlock()
	if err := b.store.Save(ctx, b.snapshotLedger()); err != nil {
		b.logger.Error("persist ledger failed", "error", err)
	}
}

// cloneDecMap returns a non-nil copy of m, so the broker's write paths (which
// index into these maps) never touch a nil map after a restore.
func cloneDecMap(m map[string]decimal.Decimal) map[string]decimal.Decimal {
	out := make(map[string]decimal.Decimal, len(m))
	maps.Copy(out, m)
	return out
}
