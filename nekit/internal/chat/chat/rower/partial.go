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

package rower

import (
	"time"

	"github.com/doors-dev/gox"
	"neui.dev/nekit/internal/chat/chat/rower/marksman"
)

const partialInterval = 3 * time.Second

func newPartialRower(m map[int]string) *rower[partialText] {
	pm := make(map[int]streamer[partialText], len(m))
	for index, content := range m {
		pm[index] = partialStreamer{marksman.New(content, partialInterval, nil)}
	}
	return newRower(pm, func(pt partialText) streamer[partialText] {
		return partialStreamer{marksman.New(pt.text, partialInterval, nil)}
	})
}

type partialText struct {
	part int
	text string
}

func (p partialText) At() int {
	return p.part
}

type partialStreamer struct {
	marksman.Marksman
}

var _ streamer[partialText] = partialStreamer{}

func (t partialStreamer) Main() gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		if err := cur.Init("div"); err != nil {
			return err
		}
		if err := cur.Set("class", "nechat-col nechat-prose"); err != nil {
			return err
		}
		if err := cur.Submit(); err != nil {
			return err
		}
		if err := cur.Comp(t.Marksman); err != nil {
			return err
		}
		return cur.Close()
	})
}

func (t partialStreamer) Append(p partialText) []replacement {
	t.Marksman.Append(p.text)
	return nil
}

func (t partialStreamer) Close() []replacement {
	t.Marksman.Close()
	return nil
}
