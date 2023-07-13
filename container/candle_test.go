package container

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"testing"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/stretchr/testify/require"
)

func TestBuffer(t *testing.T) {
	buf := bytes.NewBuffer([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	require.Equal(t, buf.Len(), 9)
	fmt.Println(buf.Cap())

	fmt.Println(buf.Next(1))
	fmt.Println(buf.Next(1))
	fmt.Println(buf.Cap())
	fmt.Println(buf.Len())
}

func TestBufferSwap(t *testing.T) {
	influxcli := influxdb2.NewClientWithOptions(
		"http://benpark.iptime.org:50086",
		"Oeov-ZmdqapihV-MUkLqM-TY8NCPvmh8xZpWiQMxC4pD4SjAXeE3FE7dVdrf6E0DJTrHNpi5WTuHZJSSgBiM1Q==",
		influxdb2.DefaultOptions().SetTLSConfig(&tls.Config{InsecureSkipVerify: true}).
			SetMaxRetryInterval(uint(5*time.Second.Milliseconds())).SetFlushInterval(uint(10*time.Second.Milliseconds())))

	api := influxcli.QueryAPI("stock")
	api.Query(context.TODO(), `
from(bucket: "stock")
  |> range(start: 2023-05-30, stop: now())
  |> filter(fn: (r) => r["_measurement"] == "price")
`)

}
