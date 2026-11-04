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
	return c.compaction, true
}

func (c compaction) isActive() bool {
	return c.cancel != nil
}
