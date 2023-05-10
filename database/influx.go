package database

import influxdb2 "github.com/influxdata/influxdb-client-go/v2"

type Influx struct {
	cli influxdb2.Client
}

func NewInfluxDatabase(url string, token string) Database {
	cli := influxdb2.NewClientWithOptions(url, token, influxdb2.DefaultOptions())
	return Influx{cli: cli}
}
