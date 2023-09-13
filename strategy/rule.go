/*
 * Copyright 2023 The Trader Authors
 *
 * Licensed under the GNU General Public License v3.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   <https:fsf.org/>
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package strategy

type Rule interface {
	Satisfied() bool
}

type or struct {
	rules []Rule
}

func Or(rules ...Rule) Rule {
	return &or{rules}
}

func (o *or) Satisfied() bool {
	for i := range o.rules {
		if o.rules[i].Satisfied() {
			return true
		}
	}
	return false
}

type and struct {
	rules []Rule
}

func And(rules ...Rule) Rule {
	return &and{rules}
}

func (a *and) Satisfied() bool {
	for i := range a.rules {
		if !a.rules[i].Satisfied() {
			return false
		}
	}
	return true
}
