package strategy

import (
	"fmt"
	"github.com/BumwooPark/trader/store/model"
)

type Smart struct {
}

func NewSmartStrategy() *Smart {
	return &Smart{}
}

func (s Smart) Logic(data model.Chart) {
	fmt.Printf("%#v\n", data)
}
