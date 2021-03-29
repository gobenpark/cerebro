package cerebro

import "testing"

func TestDefaultLogger_Info(t *testing.T) {
	cerebroLogger.Info("test")
}
