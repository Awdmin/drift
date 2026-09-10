// Package synapse renders a static, faithful pixel-art reproduction of
// the SYNAPSE-L/R reference image, framed by two narrow side columns of
// vertically-scrolling technical labels — reusing the label set from an
// earlier procedural version of this scene (SYNAPSE-L, SGL 00, AXON
// CELL-B, etc.). The art itself deliberately isn't procedural — the
// reference is a detailed illustration, not a repeating geometric
// pattern, so the only way to actually look like it is to reproduce its
// actual pixels (quantized at native resolution ahead of time; see
// bitmap_data.go) rather than generate an approximation at runtime.
//
// The side margins double as a deliberate crop: the source image's own
// label text already runs off the left/right edges of the frame, so
// giving those columns to our own scrolling labels instead loses
// nothing that wasn't already clipped in the original.
//
// Art rendering uses braille sub-pixel cells (2x4 dots per character),
// the same technique the helix and gene scenes use — this gives 8
// samples per cell instead of a half-block's 2, so edges and fine
// linework come through much more crisply. The tradeoff is one color
// per cell rather than two, decided by majority vote among that cell's
// non-background samples — a good trade here since most of the image is
// either black background or a single dominant color region.
package synapse

import (
	"math/rand"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/phlx0/drift/internal/config"
	"github.com/phlx0/drift/internal/scene"
)

// marginCols is how many columns each side margin claims for its
// scrolling label ticker; the art fills whatever's left in the middle.
const marginCols = 14

// labelSpacingRows is the vertical gap (in rows) between successive
// labels in a side column's scroll cycle.
const labelSpacingRows = 4

var tickerColor = scene.RGBColor{230, 140, 70} // matches the reference's orange label text

// leftLabels / rightLabels are the original label sets from the earlier
// procedural version of this scene, now scrolling vertically in the
// side margins instead of sitting at fixed rows.
var leftLabels = []string{"SYNAPSE-L", "SGL 00", "NOR 01", "SENSORY", "ROOT", "SPINAL", "NERVE", "AXON CELL-B"}
var rightLabels = []string{"SYNAPSE-R", "SGR", "NOR", "SPINAL", "CORD", "PROTO", "TYPE", "EVA-00"}

type Synapse struct {
	w, h int
	time float64
	rng  *rand.Rand

	// Occasional CRT-style flicker on the art — a brief brightness
	// flash or dip, sometimes with a bit of signal dropout, at random
	// intervals. flickerTimer counts down while a flicker is active;
	// flickerAcc tracks time since the last one, compared against
	// nextFlickerIn to decide when to trigger the next.
	flickerTimer   float64
	flickerAcc     float64
	nextFlickerIn  float64
	flickerBright  float64
	flickerDropout bool

	cfgSpeed float64
}

func New(cfg config.SynapseConfig) *Synapse {
	return &Synapse{cfgSpeed: cfg.Speed}
}

func (sy *Synapse) Name() string { return "synapse" }

func (sy *Synapse) Init(w, h int, t scene.Theme) {
	sy.w, sy.h = w, h
	sy.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	sy.scheduleNextFlicker()
}

func (sy *Synapse) Resize(w, h int) {
	sy.w, sy.h = w, h
}

func (sy *Synapse) Update(dt float64) {
	speed := sy.cfgSpeed
	if speed <= 0 {
		speed = 1.0
	}
	sy.time += dt * speed

	if sy.rng == nil {
		return
	}

	if sy.flickerTimer > 0 {
		sy.flickerTimer -= dt
	}
	sy.flickerAcc += dt
	if sy.flickerTimer <= 0 && sy.flickerAcc >= sy.nextFlickerIn {
		sy.triggerFlicker()
	}
}

// scheduleNextFlicker picks how long to wait before the next flicker —
// 4 to 12 seconds, so it reads as occasional and irregular rather than
// a steady pulse.
func (sy *Synapse) scheduleNextFlicker() {
	sy.nextFlickerIn = 4 + sy.rng.Float64()*8
	sy.flickerAcc = 0
}

// triggerFlicker starts a short burst: either a bright flash or a dim
// dip, occasionally paired with brief dropout (random cells going
// blank), like a weak signal cutting in and out.
func (sy *Synapse) triggerFlicker() {
	sy.flickerTimer = 0.05 + sy.rng.Float64()*0.12
	if sy.rng.Float64() < 0.5 {
		sy.flickerBright = 1.8 + sy.rng.Float64()*0.8
		sy.flickerDropout = false
	} else {
		sy.flickerBright = 0.15 + sy.rng.Float64()*0.25
		sy.flickerDropout = sy.rng.Float64() < 0.5
	}
	sy.scheduleNextFlicker()
}

func (sy *Synapse) Draw(screen tcell.Screen) {
	if sy.w <= 0 || sy.h <= 0 {
		return
	}

	margin := marginCols
	// On narrow terminals, shrink the margins rather than let them eat
	// the whole width — always leave at least 20 columns for the art.
	for margin > 0 && sy.w-2*margin < 20 {
		margin--
	}

	if margin > 0 {
		sy.drawSideColumn(screen, 0, margin, leftLabels, false)
		sy.drawSideColumn(screen, sy.w-margin, margin, rightLabels, true)
	}

	sy.drawArt(screen, margin, sy.w-margin)
}

// drawSideColumn renders one vertically-scrolling label ticker into the
// column range [x0, x0+width). rightAlign controls whether each label
// hugs the inner (art-facing) or outer edge of its column.
func (sy *Synapse) drawSideColumn(screen tcell.Screen, x0, width int, labels []string, rightAlign bool) {
	if width <= 0 || len(labels) == 0 {
		return
	}

	cycle := len(labels) * labelSpacingRows
	scroll := int(sy.time*3) % cycle

	style := tickerColor.Style()

	for cy := 0; cy < sy.h; cy++ {
		virtualRow := (cy + scroll) % cycle
		if virtualRow%labelSpacingRows != 0 {
			continue // blank spacing row between labels
		}
		label := labels[virtualRow/labelSpacingRows]
		if len(label) > width {
			label = label[:width]
		}

		start := x0
		if rightAlign {
			start = x0 + width - len(label)
		}
		for i, r := range label {
			x := start + i
			if x >= x0 && x < x0+width {
				screen.SetContent(x, cy, r, nil, style)
			}
		}
	}
}

// drawArt renders the pixel-art bitmap into the column range [x0, x1),
// full height — kept as its own function so the side margins can be
// tuned independently of how the art itself is sampled and drawn.
func (sy *Synapse) drawArt(screen tcell.Screen, x0, x1 int) {
	artCols := x1 - x0
	if artCols < 1 {
		return
	}
	pw, ph := artCols*2, sy.h*4

	for cy := 0; cy < sy.h; cy++ {
		for cx := 0; cx < artCols; cx++ {
			var mask uint8
			var counts [16]int

			for subRow := 0; subRow < 4; subRow++ {
				for subCol := 0; subCol < 2; subCol++ {
					px := cx*2 + subCol
					py := cy*4 + subRow
					idx := sampleIndex(px, py, pw, ph)
					if idx == 0 {
						continue
					}
					bitOffset := scene.BrailleOffsets[subRow][subCol]
					mask |= uint8(1) << bitOffset
					counts[idx]++
				}
			}

			if mask == 0 {
				continue // fully background — leave the terminal's own background showing
			}

			best := 1
			for i := 2; i < len(counts); i++ {
				if counts[i] > counts[best] {
					best = i
				}
			}

			if sy.flickerTimer > 0 {
				if sy.flickerDropout && sy.rng.Float64() < 0.35 {
					continue // brief dropout — this cell goes dark for the frame
				}
			}

			p := bitmapPalette[best]
			color := scene.RGBColor{R: p.R, G: p.G, B: p.B}
			if sy.flickerTimer > 0 {
				color = scaleColor(color, sy.flickerBright)
			}
			screen.SetContent(x0+cx, cy, '\u2800'|rune(mask), nil, color.Style())
		}
	}
}

// sampleIndex nearest-neighbor-samples the canonical bitmap for
// sub-pixel position (px, py) given the current braille grid size
// (pw, ph).
func sampleIndex(px, py, pw, ph int) int {
	bx := px * bitmapW / pw
	by := py * bitmapH / ph
	if bx >= bitmapW {
		bx = bitmapW - 1
	}
	if by >= bitmapH {
		by = bitmapH - 1
	}
	return hexVal(bitmapRows[by][bx])
}

func hexVal(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	default:
		return 0
	}
}

// scaleColor multiplies each channel by factor, clamping to [0, 255] —
// used to brighten or dim a cell during a flicker burst.
func scaleColor(c scene.RGBColor, factor float64) scene.RGBColor {
	scale := func(v uint8) uint8 {
		f := float64(v) * factor
		if f < 0 {
			f = 0
		}
		if f > 255 {
			f = 255
		}
		return uint8(f)
	}
	return scene.RGBColor{R: scale(c.R), G: scale(c.G), B: scale(c.B)}
}
