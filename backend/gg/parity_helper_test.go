package gg_test

import (
	"io"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/backend/svg"
	"github.com/timzifer/figure/ir"
	"golang.org/x/image/font/gofont/goregular"
)

// figureSVG builds an SVG target measuring with the same font the gg backend
// draws with, so the two can be compared like for like.
func figureSVG(w io.Writer) ir.Target {
	return figure.SVGWriter(w, svg.WithFont(goregular.TTF))
}
