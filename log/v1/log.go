/*
 *  Copyright 2021 The Trader Authors
 *
 *  Licensed under the GNU General Public License v3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      <https:fsf.org/>
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package log

import (
	"github.com/gobenpark/cerebro/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	l *zap.SugaredLogger
}

func NewLogger() (log.Logger, error) {
	conf := zap.NewProductionConfig()
	conf.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.StacktraceKey = ""
	conf.EncoderConfig = encoderConfig
	conf.OutputPaths = []string{
		"stderr",
	}

	l, err := conf.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, err
	}

	return &Logger{l.Sugar()}, nil
}

func (l *Logger) Error(msg string, kv ...interface{}) {
	l.l.Errorw(msg, kv...)
	l.l.Sync()
}

func (l *Logger) Info(msg string, kv ...interface{}) {
	l.l.Infow(msg, kv...)
	l.l.Sync()
}

func (l *Logger) Warn(msg string, kv ...interface{}) {
	l.l.Warnw(msg, kv...)
	l.l.Sync()
}

func (l *Logger) Debug(msg string, kv ...interface{}) {
	l.l.Debugw(msg, kv...)
	l.l.Sync()
}

func (l *Logger) Panic(msg string, kv ...interface{}) {
	l.l.Panicw(msg, kv...)
	l.l.Sync()
}
