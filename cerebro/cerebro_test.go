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
package cerebro

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCerebro(t *testing.T) {
	tests := []struct {
		name    string
		cerebro *Cerebro
		checker func(c *Cerebro, t *testing.T)
	}{
		{
			"default broker",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.broker)
			},
		},
		{
			"live true",
			NewCerebro(WithLive(true)),
			func(c *Cerebro, t *testing.T) {
				assert.True(t, c.isLive)
			},
		},
		{
			"live false",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.False(t, c.isLive)
			},
		},
		{
			"preload false",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.False(t, c.preload)
			},
		},
		{
			"preload true",
			NewCerebro(WithPreload(true)),
			func(c *Cerebro, t *testing.T) {
				assert.True(t, c.preload)
			},
		},
		{
			"resample",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				WithResample("KRW", 3*time.Minute, true)(c)
				assert.Equal(t, 3*time.Minute, c.compress["KRW"][0].level)
			},
		},
		{
			"cerebro order channel exist",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.order)
			},
		},
		{
			"cerebro data container not exist",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.Nil(t, c.containers)
			},
		},
		{
			"cerebro strategy engine exist",
			NewCerebro(),
			func(c *Cerebro, t *testing.T) {
				assert.NotNil(t, c.strategyEngine)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.checker(test.cerebro, t)
		})
	}
}

func TestCerebro_Stop(t *testing.T) {
	c := NewCerebro()
	err := c.Stop()
	assert.NoError(t, err)
	assert.Equal(t, "context canceled", c.Ctx.Err().Error())
}
