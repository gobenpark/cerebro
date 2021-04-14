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
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/gobenpark/trader/container"
)

type TraderChart struct {
	sync.Mutex
	container container.Container
	Input     chan container.Container
	page      *components.Page
}

func NewTraderChart() *TraderChart {
	page := components.NewPage()
	chart := &TraderChart{
		container: nil,
		Input:     make(chan container.Container, 1),
		page:      page,
	}
	return chart
}

type klineData struct {
	date string
	data [4]float32
}

func (c *TraderChart) klineStyle() *charts.Kline {
	kline := charts.NewKLine()

	x := make([]string, 0)
	y := make([]opts.KlineData, 0)

	if c.container == nil {
		return nil
	}
	c.Lock()
	data := c.container.Values()
	c.Unlock()

	sort.SliceStable(data, func(i, j int) bool {
		return data[i].Date.Before(data[j].Date)
	})

	for _, i := range data {
		x = append(x, i.Date.Format("2006-01-02 15:04:05"))
		y = append(y, opts.KlineData{Value: [4]float64{i.Open, i.Close, i.Low, i.High}})
	}

	kline.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: data[0].Code,
		}),
		charts.WithXAxisOpts(opts.XAxis{
			SplitNumber: 20,
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Scale: true,
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Start:      50,
			End:        100,
			XAxisIndex: []int{0},
		}),
	)

	kline.SetXAxis(x).AddSeries("kline", y).
		SetSeriesOptions(
			charts.WithMarkPointNameTypeItemOpts(opts.MarkPointNameTypeItem{
				Name:     "highest value",
				Type:     "max",
				ValueDim: "highest",
			}),
			charts.WithMarkPointNameTypeItemOpts(opts.MarkPointNameTypeItem{
				Name:     "lowest value",
				Type:     "min",
				ValueDim: "lowest",
			}),
			charts.WithMarkPointStyleOpts(opts.MarkPointStyle{
				Label: &opts.Label{
					Show: true,
				},
			}),
			charts.WithItemStyleOpts(opts.ItemStyle{
				Color:        "#ec0000",
				Color0:       "#130FF3",
				BorderColor:  "#8A0000",
				BorderColor0: "#130FF3",
			}),
		)
	return kline
}

func (t *TraderChart) Start() {

	go func() {
		for i := range t.Input {
			t.Lock()
			t.container = i
			t.Unlock()
		}
	}()

	go func() {
		http.HandleFunc("/", t.handler)
		if err := http.ListenAndServe(":8081", nil); err != nil {
			fmt.Println(err)
		}
	}()
}

func (t *TraderChart) handler(writer http.ResponseWriter, request *http.Request) {
	if t.page.Charts == nil {
		t.page.AddCharts(t.klineStyle())
	}
	t.page.Render(writer)
}
