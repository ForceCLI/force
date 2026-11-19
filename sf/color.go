package sf

import (
	"io"
	"os"

	"golang.org/x/term"
)

// colorEnabled reports whether output to w is styled with ANSI escape
// codes.  It follows what sf 2.138.6 does: FORCE_COLOR=0 and NO_COLOR turn
// color off, another FORCE_COLOR value or COLORTERM=truecolor turns it on
// even when the output is not a terminal, and otherwise color is on only
// for a terminal.
func colorEnabled(w io.Writer) bool {
	if force, ok := os.LookupEnv("FORCE_COLOR"); ok && force != "" {
		return force != "0" && force != "false"
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("COLORTERM") == "truecolor" {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if wrapper, ok := w.(*deprecationWriter); ok {
		w = wrapper.Writer
	}
	if file, ok := w.(*os.File); ok {
		return term.IsTerminal(int(file.Fd()))
	}
	return false
}

type style struct {
	open  string
	close string
}

var (
	styleBoldRed       = style{"\x1b[1m\x1b[31m", "\x1b[39m\x1b[22m"}
	styleBoldYellow    = style{"\x1b[1m\x1b[33m", "\x1b[39m\x1b[22m"}
	styleDim           = style{"\x1b[2m", "\x1b[22m"}
	styleBrightCyan    = style{"\x1b[96m", "\x1b[39m"}
	styleBrightMagenta = style{"\x1b[95m", "\x1b[39m"}
	styleBrightYellow  = style{"\x1b[93m", "\x1b[39m"}
	styleGreen         = style{"\x1b[32m", "\x1b[39m"}
	styleRed           = style{"\x1b[31m", "\x1b[39m"}
	styleMagenta       = style{"\x1b[35m", "\x1b[39m"}
	styleTableHeader   = style{"\x1b[1m\x1b[34m", "\x1b[39m\x1b[22m"}
	styleBold          = style{"\x1b[1m", "\x1b[22m"}
	styleBoldGreen     = style{"\x1b[1m\x1b[32m", "\x1b[39m\x1b[22m"}
)

// paint wraps text in the style when color is enabled for w.
func paint(w io.Writer, s style, text string) string {
	if !colorEnabled(w) {
		return text
	}
	return s.open + text + s.close
}
