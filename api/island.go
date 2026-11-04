package api

import (
	"context"

	"github.com/doors-dev/gox"
)

// Widget is the component a Library.View or Library.Control factory returns:
// it renders once when an island script creates it, then receives the patches
// the script sends with update. P is the patch type; its JSON schema is what
// the model is told the patch looks like.
type Widget[P any] interface {
	// Update applies a patch already decoded and validated against P; a patch
	// that fails validation never reaches it. Use ctx for the door and beam
	// updates the patch causes.
	Update(ctx context.Context, patch P)
	gox.Comp
}
