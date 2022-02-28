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

import "github.com/gobenpark/trader/log"

var _log = log.NewZapLogger()

func Debug(v ...interface{}) {
	_log.Debug(v...)
}

func Debugf(format string, v ...interface{}) {
	_log.Debugf(format, v...)
}

func Error(v ...interface{}) {
	_log.Error(v...)
}

func Errorf(format string, v ...interface{}) {
	_log.Errorf(format, v...)
}

func Info(v ...interface{}) {
	_log.Info(v...)
}
func Infof(format string, v ...interface{}) {
	_log.Infof(format, v...)
}

func Warning(v ...interface{}) {
	_log.Warning(v...)
}
func Warningf(format string, v ...interface{}) {
	_log.Warningf(format, v...)
}

func Fatal(v ...interface{}) {
	_log.Fatal(v...)
}

func Fatalf(format string, v ...interface{}) {
	_log.Fatalf(format, v...)
}

func Panic(v ...interface{}) {
	_log.Panic(v...)
}

func Panicf(format string, v ...interface{}) {
	_log.Panicf(format, v...)
}
