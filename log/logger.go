package log

type Logger interface {
	Error(msg string, kv ...interface{})
	Info(msg string, kv ...interface{})
	Warn(msg string, kv ...interface{})
	Debug(msg string, kv ...interface{})
}
