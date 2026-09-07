// Package gene renders scrolling diagonal chevron bands split into two
// colored halves by a vertical "borderline", with scrolling pseudo-code
// columns and static tip labels along the border — a sequence-analysis
// readout aesthetic. Colors are hardcoded to match the reference image
// (neon green / neon red on black, golden border) rather than the shared
// terminal theme.
package gene

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/phlx0/drift/internal/config"
	"github.com/phlx0/drift/internal/scene"
)

// bandPeriod is the vertical period (in rows) of one full chevron point.
// Set to twice stripeWidth so each half of the triangle wave sweeps a
// full 45-degree diagonal across one stripe period — that's what turns
// the fill into a clean tall arrow instead of a short, shallow wiggle.
// stripeWidth is the horizontal period of the diagonal fill stripes.
const stripeWidth = 8
const bandPeriod = stripeWidth * 2
const stripeFill = 5          // >50% of stripeWidth: bold solid ribbons, not thin dashes
const tipSpacing = bandPeriod // one tip label per chevron apex, like the reference
const tipStartNum = 24        // reference labels its border tips starting at B24

var (
	colorGreenCore = scene.RGBColor{60, 255, 140}
	colorGreenDim  = scene.RGBColor{10, 90, 45}
	colorRedCore   = scene.RGBColor{255, 60, 55}
	colorRedDim    = scene.RGBColor{90, 15, 15}
	colorBorder    = scene.RGBColor{255, 205, 60} // golden border + labels
	colorSync      = scene.RGBColor{255, 240, 200}
)

type Gene struct {
	w, h int

	scrollOffset float64

	syncAcc  float64
	syncTTL  float64
	cfgSpeed float64
	cfgLabel string
}

func New(cfg config.GeneConfig) *Gene {
	return &Gene{
		cfgSpeed: cfg.Speed,
		cfgLabel: cfg.Label,
	}
}

func (g *Gene) Name() string { return "gene" }

func (g *Gene) Init(w, h int, t scene.Theme) {
	g.w, g.h = w, h
}

func (g *Gene) Resize(w, h int) {
	g.w, g.h = w, h
}

func (g *Gene) Update(dt float64) {
	speed := g.cfgSpeed
	if speed <= 0 {
		speed = 1.0
	}
	g.scrollOffset += dt * speed * 5.0

	g.syncAcc += dt
	if g.syncAcc > 2.2 {
		g.syncAcc = 0
		g.syncTTL = 0.6
	}
	if g.syncTTL > 0 {
		g.syncTTL -= dt
	}
}

// chevronShift returns the horizontal offset for row y: a triangle wave
// across bandPeriod rows, which turns plain diagonal stripes into pointed
// chevrons when tiled.
func chevronShift(y int) int {
	r := y % bandPeriod
	if r < 0 {
		r += bandPeriod
	}
	half := bandPeriod / 2
	if r > half {
		r = bandPeriod - r
	}
	return r
}

func (g *Gene) Draw(screen tcell.Screen) {
	borderX := g.w / 2
	scroll := int(g.scrollOffset)

	for y := 0; y < g.h; y++ {
		shift := chevronShift(y)
		for x := 0; x < g.w; x++ {
			if x == borderX {
				continue // border drawn on top, after the fill
			}

			v := (x + shift + scroll) % stripeWidth
			if v < 0 {
				v += stripeWidth
			}
			if v >= stripeFill {
				continue
			}

			core, edge := colorGreenCore, colorGreenDim
			if x > borderX {
				core, edge = colorRedCore, colorRedDim
			}

			// Solid block throughout — the reference's chevrons are bold
			// filled ribbons, not a stippled/dotted trace. Bright core
			// color traces both edges of the band; the interior is a
			// darker fill, giving each arrow a visible outline.
			ch := '\u2588'
			color := edge
			if v == 0 || v == stripeFill-1 {
				color = core
			}
			screen.SetContent(x, y, ch, nil, color.Style())
		}
	}

	g.drawSideColumns(screen, borderX)
	g.drawBorder(screen, borderX)
}

// drawSideColumns prints two narrow, steadily-scrolling columns of
// pseudo-telemetry codes hugging each margin — a sequential ID plus a
// short binary suffix on the left, a longer hex-style ID on the right —
// echoing the aligned code columns in the reference readout.
func (g *Gene) drawSideColumns(screen tcell.Screen, borderX int) {
	base := int(g.scrollOffset / 6)

	leftStyle := colorGreenCore.Style()
	rightStyle := colorRedCore.Style()

	for row := 0; row < g.h; row += 2 {
		leftText := fmt.Sprintf("A%04d %05b", (base+row)%10000, (base+row*3)%32)
		rightText := fmt.Sprintf("MT-%05d", (base*3+row*7)%100000)

		for i, r := range leftText {
			if i < g.w {
				screen.SetContent(i, row, r, nil, leftStyle)
			}
		}
		rx0 := g.w - len(rightText)
		for i, r := range rightText {
			x := rx0 + i
			if x >= 0 && x < g.w {
				screen.SetContent(x, row, r, nil, rightStyle)
			}
		}
	}

	if g.syncTTL > 0 {
		text := "SYNC"
		y := (base / 3) % maxInt(g.h, 1)
		x := borderX + 6
		style := colorSync.Style()
		for i, r := range text {
			if x+i >= 0 && x+i < g.w {
				screen.SetContent(x+i, y, r, nil, style)
			}
		}
	}
}

// drawBorder draws the vertical divider, its caption, and a static
// sequential tip label (B24, B25, ...) at each chevron point row — the
// row position stays fixed even as the chevron fill scrolls sideways,
// matching how the reference's tip labels sit still while the pattern
// moves underneath them.
func (g *Gene) drawBorder(screen tcell.Screen, borderX int) {
	if borderX < 0 || borderX >= g.w {
		return
	}
	style := colorBorder.Style()
	for y := 0; y < g.h; y++ {
		if y%2 == 0 {
			screen.SetContent(borderX, y, '\u2502', nil, style)
		}
	}

	label := g.cfgLabel
	if label == "" {
		label = "BORDERLINE"
	}
	lx := borderX - len(label)/2
	for i, r := range label {
		x := lx + i
		if x >= 0 && x < g.w {
			screen.SetContent(x, 1, r, nil, style)
		}
	}

	for y := tipSpacing; y < g.h; y += tipSpacing {
		tipNum := tipStartNum + y/tipSpacing
		tip := fmt.Sprintf("B%02d", tipNum)
		tx := borderX + 2
		for i, r := range tip {
			if tx+i >= 0 && tx+i < g.w {
				screen.SetContent(tx+i, y, r, nil, style)
			}
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
