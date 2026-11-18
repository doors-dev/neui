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

package tools

import "context"

type fromKey struct{}

type from int

const (
	fromAgent from = iota
	fromJS
)

func FromJS(ctx context.Context) bool {
	from, ok := ctx.Value(fromKey{}).(from)
	if !ok {
		return false
	}
	return from == fromJS
}

func FromAgent(ctx context.Context) bool {
	from, ok := ctx.Value(fromKey{}).(from)
	if !ok {
		return false
	}
	return from == fromAgent
}

func Attach(ctx context.Context, source context.Context) context.Context {
	f, ok := source.Value(fromKey{}).(from)
	if !ok {
		return ctx
	}
	return context.WithValue(ctx, fromKey{}, f)
}

func makeFromJS(ctx context.Context) context.Context {
	from, ok := ctx.Value(fromKey{}).(from)
	if ok && from == fromJS {
		return ctx
	}
	return context.WithValue(ctx, fromKey{}, fromJS)
}

func makeFromAgent(ctx context.Context) context.Context {
	from, ok := ctx.Value(fromKey{}).(from)
	if ok && from == fromAgent {
		return ctx
	}
	return context.WithValue(ctx, fromKey{}, fromAgent)
}
