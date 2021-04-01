package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/gobenpark/trader/order"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
)

func TestStore_Cash(t *testing.T) {
	s := NewStore("")
	fmt.Println(s.Cash())
}

func TestStore_Order(t *testing.T) {
	s := NewStore("")

	o := &order.Order{
		OType:      order.Buy,
		ExecType:   order.Limit,
		Code:       "KRW-MLK",
		Size:       10,
		Price:      2500,
		UUID:       uuid.NewV4().String(),
		CreatedAt:  time.Time{},
		ExecutedAt: time.Time{},
	}
	err := s.Order(o)
	fmt.Println(o)
	require.NoError(t, err)
}

func TestStore_OrderState(t *testing.T) {
	s := NewStore("")
	o, err := s.OrderInfo("57f761cd-73bf-4d2e-86ec-506640388ddf")
	require.NoError(t, err)
	fmt.Println(o.Status())
}
