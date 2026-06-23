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

// Package store provides ready-made broker.Storage implementations for
// persisting and restoring the broker's per-strategy ledger across a restart.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gobenpark/cerebro/broker"
)

// FileStorage persists the broker ledger to a single JSON file. Writes are
// atomic — the ledger is written to a temp file in the same directory, flushed,
// and renamed over the target — so a crash mid-write leaves the previous good
// snapshot intact rather than a half-written one.
type FileStorage struct {
	path string
}

var _ broker.Storage = (*FileStorage)(nil)

// NewFileStorage returns a FileStorage that reads and writes the ledger at path.
func NewFileStorage(path string) *FileStorage {
	return &FileStorage{path: path}
}

// Save atomically writes the ledger to the configured path.
func (f *FileStorage) Save(_ context.Context, l broker.Ledger) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ledger: %w", err)
	}

	dir := filepath.Dir(f.path)
	tmp, err := os.CreateTemp(dir, ".ledger-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	// Remove the temp file on any error path; after a successful rename it is gone
	// already, so this is a no-op.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, f.path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// Load reads the persisted ledger. A missing file is reported as a fresh start
// (a zero-value Ledger and a nil error), not an error.
func (f *FileStorage) Load(_ context.Context) (broker.Ledger, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return broker.Ledger{}, nil
	}
	if err != nil {
		return broker.Ledger{}, fmt.Errorf("read ledger: %w", err)
	}
	var l broker.Ledger
	if err := json.Unmarshal(data, &l); err != nil {
		return broker.Ledger{}, fmt.Errorf("unmarshal ledger: %w", err)
	}
	return l, nil
}

// MemoryStorage keeps the ledger in memory. It is handy for tests and for runs
// that want restore-within-process semantics (e.g. a paper-trading session that
// hands its ledger to a fresh broker) without touching disk. It is safe for
// concurrent use.
type MemoryStorage struct {
	mu     sync.Mutex
	ledger broker.Ledger
	saved  bool
}

var _ broker.Storage = (*MemoryStorage)(nil)

// NewMemoryStorage returns an empty in-memory ledger store.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

// Save records the ledger in memory.
func (m *MemoryStorage) Save(_ context.Context, l broker.Ledger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ledger = l
	m.saved = true
	return nil
}

// Load returns the last saved ledger, or a fresh-start zero value if none has
// been saved yet.
func (m *MemoryStorage) Load(_ context.Context) (broker.Ledger, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.saved {
		return broker.Ledger{}, nil
	}
	return m.ledger, nil
}
