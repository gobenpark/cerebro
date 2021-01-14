package strategy

import "github.com/gobenpark/trader/store/model"

type Strategy interface {
	ChartChannel() chan<- model.Chart
	Logic()
}
