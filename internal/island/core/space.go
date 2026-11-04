package core

import (
	"context"

	"neui.dev/internal/common"
)

type space interface {
	AddSpace(ctx context.Context, name string, desc string) (context.Context, error)
}

func getSpace(ctx context.Context) space {
	s, ok := ctx.Value(common.KeySpace).(space)
	if !ok {
		panic("island: no space in context")
	}
	return s
}
