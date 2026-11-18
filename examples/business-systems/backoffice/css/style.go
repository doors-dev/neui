package css

import _ "embed"

// Style is the back office stylesheet, built by build.sh from input.css. Link
// it as <link rel="stylesheet" href=(css.Style)>; Doors serves it as a
// cacheable resource.
//
//go:embed output.css
var Style []byte
