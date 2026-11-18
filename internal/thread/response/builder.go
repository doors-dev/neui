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

package response

import (
	"errors"
	"fmt"
	"sort"

	"neui.dev/api"
)

type Builder = *builder

func NewBulder() Builder {
	return &builder{
		blocks: make(map[int]block),
	}
}

type builder struct {
	blocks map[int]block
	offset int
	start  *api.EventStart
	err    *api.EventErr
	ok     *api.EventOk
	usage  *api.EventUsage
	spent  api.EventUsage
}

type Response struct {
	code    any
	Cause   error
	Usage   api.EventUsage
	Content api.Indexed[api.Content]
}

func (r Response) Ok() (api.OkCode, bool) {
	if code, ok := r.code.(api.OkCode); ok {
		return code, true
	}
	return "", false
}

func (r Response) Err() (api.ErrCode, bool) {
	if code, ok := r.code.(api.ErrCode); ok {
		return code, true
	}
	return "", false
}

func (r Builder) State() api.Indexed[api.Content] {
	contents := make(api.Indexed[api.Content], len(r.blocks))
	for index, block := range r.blocks {
		contents[index] = block.assemble(false)
	}
	return contents
}

func (r Builder) Spent() api.EventUsage {
	spent := r.spent
	if r.usage == nil {
		return spent
	}
	spent.Input += r.usage.Input
	spent.Output += r.usage.Output
	spent.Reasoning += r.usage.Reasoning
	spent.CacheRead += r.usage.CacheRead
	spent.CacheWrite += r.usage.CacheWrite
	return spent
}

func (r Builder) Abort() (usage api.EventUsage, content api.Indexed[api.Content]) {
	defer r.reset()
	usage = r.flushUsage()
	if len(r.blocks) == 0 {
		return
	}
	content = make(api.Indexed[api.Content], len(r.blocks))
	for index, block := range r.blocks {
		content[index] = block.assemble(false)
	}
	return usage, content
}

func (r Builder) Flush() (response Response) {
	defer r.reset()
	response.Usage = r.flushUsage()
	if r.ok == nil && r.err == nil {
		r.onErr(fmt.Errorf("%w: the stream ended with no terminal event", ErrNotEnded))
	}
	if r.ok != nil && len(r.blocks) == 0 {
		r.onErr(fmt.Errorf("%w: %s arrived with nothing to store", ErrNoContent, r.ok.Code))
	}
	if r.err != nil {
		response.code = r.err.Code
		response.Cause = r.err.Err
	} else if r.ok != nil {
		response.code = r.ok.Code
	}
	if len(r.blocks) == 0 {
		return
	}
	indexes := make([]int, 0, len(r.blocks))
	for index := range r.blocks {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	r.offset = indexes[len(indexes)-1] + 1
	response.Content = make(api.Indexed[api.Content], len(r.blocks))
	for index, block := range r.blocks {
		response.Content[index] = block.assemble(true)
	}
	return
}

func (r *builder) onErr(err error) api.Event {
	e := api.EventErr{
		Code: api.CodeUnknown,
		Err:  err,
	}
	if r.err == nil {
		r.err = &e
	}
	return e
}

func (r *builder) On(e api.Event) (api.Event, bool) {
	switch e := e.(type) {
	case api.EventErr:
		if r.ok != nil {
			return r.onErr(fmt.Errorf("%w: %T arrived after the end", ErrEnded, e)), false
		}
		r.err = &e
		return e, true
	case api.EventOk:
		if r.start == nil {
			return r.onErr(fmt.Errorf("%w: %T arrived before the start", ErrNotStarted, e)), false
		}
		if r.err != nil {
			return r.onErr(fmt.Errorf("%w: %T arrived after the end", ErrEnded, e)), false
		}
		r.ok = &e
		return e, true
	case api.EventStart:
		if r.start != nil {
			return r.onErr(fmt.Errorf("%w: a second %T arrived", ErrStarted, e)), false
		}
		r.start = &e
		return e, true
	case api.IndexedEvent:
		if r.start == nil {
			return r.onErr(fmt.Errorf("%w: %T arrived before the start", ErrNotStarted, e)), false
		}
		if r.err != nil || r.ok != nil {
			return r.onErr(fmt.Errorf("%w: %T arrived after the end", ErrEnded, e)), false
		}
		if r.offset != 0 {
			e = e.Offset(r.offset)
		}
		block, ok := r.blocks[e.At()]
		if ok {
			if err := block.append(e); err != nil {
				return r.onErr(err), false
			}
			return e, true
		}
		block, err := newBlock(r.start.Model, e)
		if err != nil {
			return r.onErr(err), false
		}
		r.blocks[e.At()] = block
		return e, true
	case api.EventUsage:
		if r.start == nil {
			return r.onErr(fmt.Errorf("%w: %T arrived before the start", ErrNotStarted, e)), false
		}
		r.usage = &e
		return r.Spent(), true
	default:
		panic(fmt.Sprintf("thread: %T is not handled", e))
	}
}

func (r *builder) flushUsage() api.EventUsage {
	r.spent = r.Spent()
	r.usage = nil
	return r.spent
}

func (r *builder) reset() {
	r.start = nil
	r.ok = nil
	r.err = nil
}

var ErrNoContent = errors.New("thread: turn ended with no content")
var ErrNotStarted = errors.New("thread: turn has not started")
var ErrStarted = errors.New("thread: turn already started")
var ErrNotEnded = errors.New("thread: turn has not ended")
var ErrEnded = errors.New("thread: turn already ended")
