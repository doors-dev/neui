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
