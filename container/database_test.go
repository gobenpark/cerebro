/*
 * Copyright 2023 The Trader Authors
 *
 * Licensed under the GNU General Public License v3.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   <https:fsf.org/>
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package container

import (
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestDatabase_Add(t *testing.T) {
	db, err := badger.Open(badger.DefaultOptions("").WithInMemory(true))
	require.NoError(t, err)

	//err = db.Update(func(txn *badger.Txn) error {
	//	return txn.SetEntry(badger.NewEntry([]byte("code:data"), []byte("datatest")))
	//})
	//require.NoError(t, err)

	err = db.View(func(txn *badger.Txn) error {
		it, err := txn.Get([]byte("code:data"))
		if err != nil {
			return err
		}

		t.Log(it.Value(func(val []byte) error {
			fmt.Println(string(val))
			return nil
		}))
		return nil
	})
	require.NoError(t, err)

}
