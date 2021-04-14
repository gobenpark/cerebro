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
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTraderChart_Start(t *testing.T) {
	chart := NewTraderChart()
	http.HandleFunc("/", chart.handler)
	err := http.ListenAndServe(":8081", nil)
	require.NoError(t, err)
}
