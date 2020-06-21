package strategy

import "github.com/BumwooPark/trader/store/model"

type Strategy interface {
	Logic(data model.Chart)
}
