/*
 *                     GNU GENERAL PUBLIC LICENSE
 *                        Version 3, 29 June 2007
 *
 *  Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
 *  Everyone is permitted to copy and distribute verbatim copies
 *  of this license document, but changing it is not allowed.
 *
 *                             Preamble
 *
 *   The GNU General Public License is a free, copyleft license for
 * software and other kinds of works.
 */

package chart

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTraderChart_Start(t *testing.T) {
	ctx := context.TODO()
	chart := NewTraderChart()
	chart.Start()
	assert.NotNil(t, chart.Input)
	assert.Nil(t, chart.container)
	<-ctx.Done()
}
