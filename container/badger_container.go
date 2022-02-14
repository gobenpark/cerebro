/*
 *  Copyright 2021 The Trader Authors
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

package container

import (
	"fmt"

	"github.com/dgraph-io/badger/v3"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type bgContainer struct {
	*badger.DB
	DataContainer
}

func NewBadgerContainer(db *badger.DB, info Info) Container {
	b := &bgContainer{}
	b.Info = info
	b.DB = db
	return b
}

func (b *bgContainer) getTx() (*badger.Txn, *badger.Item, error) {
	txn := b.NewTransaction(false)
	bt, err := json.Marshal(b.Info)
	if err != nil {
		return nil, nil, err
	}
	it, err := txn.Get(bt)
	if err != nil {
		return nil, nil, err
	}
	return txn, it, nil
}

func (b *bgContainer) Empty() bool {
	txn := b.NewTransaction(false)
	defer txn.Discard()
	bt, err := json.Marshal(b.DataContainer.Info)
	if err != nil {
		return true
	}
	it, err := txn.Get(bt)
	if err != nil {
		return true
	}
	if it.EstimatedSize() != 0 {
		return false
	}
	return true
}

func (b *bgContainer) Size() int {
	txn := b.NewTransaction(false)
	defer txn.Discard()
	bt, err := json.Marshal(b.Info)
	if err != nil {
		return 0
	}
	it, err := txn.Get(bt)
	if err != nil {
		return 0
	}
	return int(it.ValueSize())
}

func (b *bgContainer) Clear() {
	panic("implement me")
}

func (b *bgContainer) Values() []Candle {
	var c []Candle
	txn, it, err := b.getTx()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer txn.Discard()
	d, err := it.ValueCopy(nil)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	if err := json.Unmarshal(d, &c); err != nil {
		fmt.Println(err)
		return nil
	}
	return c
}

func (b *bgContainer) Add(candle Candle) {

	txn := b.NewTransaction(true)
	bt, err := json.Marshal(b.Info)
	if err != nil {
		fmt.Println(err)
		return
	}
	it, err := txn.Get(bt)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			jcs, err := json.Marshal([]Candle{candle})
			if err != nil {
				fmt.Println(err)
				return
			}
			if err := txn.SetEntry(&badger.Entry{
				Key:   bt,
				Value: jcs,
			}); err != nil {
				fmt.Println(err)
				return
			}
		} else {
			fmt.Println(err)
			return
		}
	} else {
		var cs []Candle
		data, err := it.ValueCopy(nil)
		if err != nil {
			fmt.Println(err)
			return
		}
		if err := json.Unmarshal(data, &cs); err != nil {
			fmt.Println(err)
			return
		}

		cs = append([]Candle{candle}, cs...)
		jcs, err := json.Marshal(cs)
		if err != nil {
			fmt.Println(err)
			return
		}
		if err := txn.SetEntry(&badger.Entry{
			Key:   bt,
			Value: jcs,
		}); err != nil {
			fmt.Println(err)
			return
		}
	}
	txn.Commit()
}
