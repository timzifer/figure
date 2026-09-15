// Command spectrogram draws a measured field at the size a measured field
// comes in.
//
// Four seconds of audio, short-time Fourier transformed into 1322 frames of 128
// bins: 169,216 readings, one picture. Drawn as one rect per cell that is a
// path apiece in the SVG, every one of them smaller than a pixel; drawn as a
// raster it is one image the size of the panel, and the file is the same size
// whether the field has ten thousand cells or ten million.
//
// # Why it passes Resample(geom.Max)
//
// The lattice is finer than the panel along time, so several frames land on
// each pixel column and something has to be dropped. Which thing depends on
// what was measured: a spectrum's reading is a peak, and a peak one frame wide
// is exactly what the nearest frame to a pixel centre is as likely as not to
// miss — the chirp comes out dotted and the click disappears. Max keeps it.
// A temperature map would pass geom.Mean, and both are named rather than
// assumed for that reason.
//
// The colourbar down the side is the mark's own, in dB: it is what the raster
// has and the hexbin does not, because these values are the data's and are
// settled before the panel is measured.
//
// The signal is synthesised from a formula rather than read from a file, so
// this draws the same chart on every machine and needs no fixture: a chirp
// sweeping up through the band, a steady tone across the whole of it, and one
// broadband click a third of the way in.
//
//	go run ./examples/spectrogram
package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/palette"
	"github.com/timzifer/figure/scale"
)

// The recording and the transform over it. The hop is a quarter of the window,
// which is the overlap that makes a chirp a line rather than a staircase; the
// window is a power of two out of habit rather than necessity, because the
// transform below is a plain one.
const (
	sampleRate = 8000
	seconds    = 4
	window     = 256
	hop        = 24
	bins       = window / 2 // up to the Nyquist frequency
)

// floorDB is where the ramp bottoms out. A logarithm has no floor of its own
// and silence is minus infinity, so a field drawn without one spends its whole
// ramp on the difference between two kinds of nothing.
const floorDB = -70

func main() {
	out := flag.String("o", "spectrogram.svg", "output SVG path")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "spectrogram:", err)
		os.Exit(1)
	}
}

func run(out string) error {
	src := transform()
	t, _ := data.Float64Column(src, "t")
	frames := len(t) / bins

	p := figure.New(
		figure.Size(900, 480),
		figure.Title(fmt.Sprintf("Four seconds of audio — %d frames by %d bins", frames, bins)),
		figure.XTitle("time (s)"),
		figure.YTitle("frequency (Hz)"),
	)
	p.X(scale.Linear())
	p.Y(scale.Linear())

	p.Add(geom.Raster(src,
		geom.X("t"), geom.Y("hz"), geom.Z("db"),
		geom.ColorBy("db", scale.Sequential(palette.Magma)),
		// The peak survives the downscale. See the note at the top.
		geom.Resample(geom.Max),
	))
	return p.Render(figure.SVG(out))
}

// transform is the short-time Fourier transform of the signal, as the long
// (t, hz, db) table a raster reads: one row per frame per bin, every cell
// present exactly once.
//
// It is a plain transform rather than a fast one. The example is about drawing
// a field, and 1320 frames of 128 bins is a third of a second's arithmetic —
// which is cheaper than an FFT this file would otherwise have to explain.
func transform() figure.Source {
	pcm := signal()
	frames := (len(pcm) - window) / hop

	// The window function, and the one table of sines the transform reads. Both
	// are per bin or per sample rather than per cell, which is what keeps this
	// a third of a second rather than a minute.
	taper := make([]float64, window)
	for i := range taper {
		taper[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(window-1))
	}

	ts := make([]float64, 0, frames*bins)
	hz := make([]float64, 0, frames*bins)
	db := make([]float64, 0, frames*bins)

	frame := make([]float64, window)
	for f := range frames {
		at := f * hop
		for i := range frame {
			frame[i] = pcm[at+i] * taper[i]
		}
		t := float64(at+window/2) / sampleRate
		for k := range bins {
			ts = append(ts, t)
			hz = append(hz, float64(k)*sampleRate/window)
			db = append(db, decibels(magnitude(frame, k)))
		}
	}
	return figure.NewTable().Float64("t", ts).Float64("hz", hz).Float64("db", db)
}

// magnitude is the size of one frequency component of one frame.
func magnitude(frame []float64, k int) float64 {
	w := 2 * math.Pi * float64(k) / float64(len(frame))
	re, im := 0.0, 0.0
	for i, v := range frame {
		s, c := math.Sincos(w * float64(i))
		re, im = re+v*c, im-v*s
	}
	return 2 * math.Hypot(re, im) / float64(len(frame))
}

// decibels is a magnitude in dB, floored so that silence is the bottom of the
// ramp rather than minus infinity — which is a value no scale can place and no
// colour can show.
func decibels(mag float64) float64 {
	if mag <= 0 {
		return floorDB
	}
	return math.Max(20*math.Log10(mag), floorDB)
}

// hash is a deterministic value in [-1, 1) from a sample index: broadband
// enough to read as noise, and the same on every machine.
func hash(i int) float64 {
	v := math.Sin(float64(i)*12.9898) * 43758.5453
	return 2*(v-math.Floor(v)) - 1
}

// signal is the recording: a chirp, a steady tone, and one click.
//
// Deterministic, like every other example here — a formula rather than
// math/rand, so a fresh run and the committed figure are the same chart.
func signal() []float64 {
	n := sampleRate * seconds
	out := make([]float64, n)
	for i := range out {
		t := float64(i) / sampleRate

		// A chirp sweeping 200 Hz to 3 kHz. The phase is the integral of the
		// frequency, not the frequency times the time — the second is the
		// mistake that draws a sweep at half the slope it claims.
		f0, f1 := 200.0, 3000.0
		rate := (f1 - f0) / seconds
		out[i] += 0.8 * math.Sin(2*math.Pi*(f0*t+rate*t*t/2))

		// A steady tone across the whole recording, which is the horizontal
		// line a reader checks the frequency axis against.
		out[i] += 0.35 * math.Sin(2*math.Pi*1200*t)

		// One broadband click, a third of the way in and a millisecond wide:
		// the event a mean would average into the noise.
		if d := t - float64(seconds)/3; d >= 0 && d < 0.001 {
			out[i] += 3 * (1 - d/0.001)
		}

		// A little hiss, so that the quiet parts are a floor rather than a
		// flat minus seventy. It is a hash of the sample index rather than a
		// random source, for the reason above — and a hash rather than a sine,
		// because a sine is a tone and would draw a line of its own across the
		// picture.
		out[i] += 0.006 * hash(i)
	}
	return out
}
