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

package island

import (
	"context"

	"neui.dev/internal/common"
)

// Space is what an island needs from the page space it is rendered in.
type Space interface {
	AddData(ctx context.Context, text string, object any) error
	AddSpace(ctx context.Context, name string, desc string) (context.Context, error)
	AddTool(ctx context.Context, t common.Tool) error
}

func getSpace(ctx context.Context) Space {
	s, ok := ctx.Value(common.KeySpace).(Space)
	if !ok {
		panic("island: no space in context")
	}
	return s
}
