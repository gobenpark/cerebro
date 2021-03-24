package container

import (
	"fmt"
	"testing"
)

func TestLeftInsert(t *testing.T) {
	data := []int{1, 2, 3, 4, 5}

	data = append([]int{7}, data...)
	fmt.Println(data)

}
