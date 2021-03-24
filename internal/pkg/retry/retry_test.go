package retry

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetry(t *testing.T) {
	err := Retry(5, func() error {
		fmt.Println("start")
		return errors.New("test")
	})
	assert.NoError(t, err)
}
