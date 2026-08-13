// Package assets holds static files compiled into the binary.
package assets

import _ "embed"

// ASCIISpinner is a frame-by-frame ASCII rendering of the Abstraction symbol
// rotation, used as the REPL's loading animation. Frames are 30 columns by 15
// rows and are separated by a line containing only "---".
//
//go:embed ascii-spinner/ascii-30.txt
var ASCIISpinner string
