package cerebro

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestFileext(t *testing.T) {
	data := filepath.Ext("test.txt")
	fmt.Println(data)
	panic("data")

}
