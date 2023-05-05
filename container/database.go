/*
 *  Copyright 2023 The Trader Authors
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
	"errors"

	"github.com/dgraph-io/badger/v4"
)

const (
	StockCodeTick = "stock:code:tick"
)

type DataObject interface {
	Code() string
}

type Database struct {
	data map[string]interface{}
	*badger.DB
}

func (d *Database) Add(input DataObject) error {
	txn := d.DB.NewTransaction(true)
	defer txn.Discard()

	item, err := txn.Get([]byte(StockCodeTick + input.Code()))
	if errors.Is(err, badger.ErrKeyNotFound) {

	} else if err != nil {
		return err
	}
	_ = item
	return nil
}
