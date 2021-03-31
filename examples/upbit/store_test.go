package main

import (
	"fmt"
	"testing"
)

func TestStore_Cash(t *testing.T) {
	s := NewStore("")
	fmt.Println(s.Cash())
}
