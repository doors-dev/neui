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

package thread

import (
	"context"

	"neui.dev/api"
	"neui.dev/internal/replay"
)

type compaction struct {
	seq        int
	cancel     context.CancelFunc
	compaction replay.Compaction
	held       *commandApplyCompaction
	chunk      int
}

func (c *compaction) start(ctx context.Context, rc replay.Compaction) (context.Context, int) {
	if c.isActive() {
		panic("alrady active")
	}
	ctx, cancel := context.WithCancel(ctx)
	c.seq += 1
	c.cancel = cancel
	c.compaction = rc
	return ctx, c.seq
}

func (c *compaction) stop() {
	c.chunk = 0
	if !c.isActive() {
		return
	}
	c.seq += 1
	c.cancel()
	c.cancel = nil
	c.held = nil
}

func (c *compaction) cut(round api.RoundID) bool {
	if !c.isActive() || round > c.compaction.Tail {
		return false
	}
	c.stop()
	return true
}

func (c *compaction) report(seq int) (replay.Compaction, bool) {
	if c.seq != seq {
		return replay.Compaction{}, false
	}
	c.seq += 1
	c.cancel()
	c.cancel = nil
	c.chunk = 0
	return c.compaction, true
}

func (c compaction) isActive() bool {
	return c.cancel != nil
}
