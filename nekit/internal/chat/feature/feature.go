// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package feature

type Feature interface {
	feature() uint32
}

type Part uint8

const (
	Tokens Part = iota + 1
	Thinking
	Tools
	Tasks
	Edit
	Retry
	Attachments
	Sidebar
	ThreadEdit
	MessageSearch
	Greeting
	Files
)

func (p Part) feature() uint32 {
	return 1 << p
}

type Mode uint8

const (
	Float Mode = iota
	Dock
)

func (m Mode) feature() uint32 {
	return 1 << (24 + m)
}

type Set uint32

func NewSet(features []Feature) Set {
	var s Set
	for _, f := range features {
		if f != nil {
			s |= Set(f.feature())
		}
	}
	return s
}

func (s Set) Has(f Feature) bool {
	return s&Set(f.feature()) != 0
}
