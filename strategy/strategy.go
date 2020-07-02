package strategy

import "github.com/BumwooPark/trader/store/model"

type Strategy interface {
	ChartChannel() chan<- model.Chart
	Logic()
}
