package api

import (
	"context"

	"github.com/doors-dev/gox"
)

type Widget[P any] interface {
	Update(ctx context.Context, patch P)
	gox.Comp
}
