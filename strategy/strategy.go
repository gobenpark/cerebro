package strategy

import "github.com/BumwooPark/trader/store/model"

type Strategy interface {
	Next() chan<- model.Chart
}
