package island

import (
	"context"

	"neui.dev/internal/common"
)

type space interface {
	AddData(ctx context.Context, text string, object any) error
	AddSpace(ctx context.Context, name string, desc string) (context.Context, error)
}

func getSpace(ctx context.Context) space {
	s, ok := ctx.Value(common.KeySpace).(space)
	if !ok {
		panic("island: no space in context")
	}
	return s
}
