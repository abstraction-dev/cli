// Package assets holds static files compiled into the binary.
package assets

import _ "embed"

// ASCIISpinner is a frame-by-frame ASCII rendering of the Abstraction symbol
// rotation, used as the REPL's loading animation. Frames are 30 columns by 15
// rows and are separated by a line containing only "---".
//
// The file holds 98 frames, one short of the source Lottie's range: its final
// frame renders a pose from ten frames earlier, which made the loop visibly
// step backwards before starting over. If these are ever regenerated, drop that
// last frame again — TestLoaderLoopIsSeamless is what catches it.
//
//go:embed ascii-spinner/ascii-30.txt
var ASCIISpinner string
