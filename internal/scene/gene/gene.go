// Package gene renders a lane-based technical readout: alternating
// columns of static diagonal bars and label lists, colored by a
// discrete left-to-right sweep — a from-scratch rewrite built by
// directly measuring the reference footage, corrected twice more from
// specific feedback against earlier attempts:
//
//  1. The shapes are plain static slanted rectangles, not pointed
//     chevrons, and nothing scrolls sideways.
//  2. The red/green split is a snapshot of an ongoing sweep, not a
//     fixed half-and-half.
//  3. The sweep operates on whole blocks, not a continuous per-pixel
//     gradient: a block (one bar) is entirely red or entirely green,
//     never partial. It reveals one lane's blocks top-to-bottom before
//     moving to the next lane to the right — that's the "up and down
//     the columns, left to right" part. Near the border it doesn't just
//     pause; a handful of blocks around that point actually flicker
//     between red and green (still whole-block, still in column order)
//     for a long dwell before the sweep continues and finishes, then
//     resets to all-red.
//
// Bars are narrower and less steeply slanted than the first pass, and
// label lanes carry a thin orange connector spine + tick per row,
// matching the schematic connector lines visible in the reference.
package gene

import (
	"fmt"
	"math"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/phlx0/drift/internal/config"
	"github.com/phlx0/drift/internal/scene"
)

// Bar geometry, in canonical braille sub-pixel rows (each character row
// is 4 sub-pixel rows tall).
const barPeriod = 20
const barDutyCycle = 0.30

// barSlope is sub-pixel columns of horizontal shift per row. LOWER =
// more horizontal shift per row = more horizontal-leaning bars (this
// was backwards in the previous comment — increasing it moves toward
// vertical/upright, which was the wrong direction the last time around).
// A shallow (low) slope needs a wider lane to render as one clean
// diagonal stroke instead of wrapping into choppy segments — that's why
// barLaneCols went back up alongside this change.
const barSlope = 1.3

const barLaneCols = 9   // widened to fit the shallower slope cleanly (see barSlope)
const labelLaneCols = 9 // shortened labels (see drawLabelLane) need less room now

// Slant variety, confirmed by zooming into the reference: the green
// (left) side's bars lean one way and the red (right) side's lean the
// opposite way — a mirror across the border, not the same direction on
// both sides like earlier versions had. A flat/horizontal variant is
// mixed in occasionally too, for the variety asked for beyond the
// strict mirror.
const (
	slantRising  = iota // leans "/" — bottom-left to top-right
	slantFalling        // leans "\" — top-left to bottom-right
	slantFlat           // no lean at all — plain horizontal bands
)

var (
	colorGreenCore = scene.RGBColor{60, 255, 140}
	colorGreenDim  = scene.RGBColor{10, 90, 45}
	colorRedCore   = scene.RGBColor{255, 60, 55}
	colorRedDim    = scene.RGBColor{90, 15, 15}
	colorBorder    = scene.RGBColor{140, 255, 210}
	colorLabel     = scene.RGBColor{230, 150, 60}
)

// Sweep timing, in seconds.
const sweepApproach = 4.0 // 0 -> border, revealing block by block
const sweepHang = 14.0    // long dwell: a few blocks near the border flicker
const sweepComplete = 4.0 // border -> end, revealing block by block
const sweepReset = 0.6    // snap back to all-red
const sweepTotal = sweepApproach + sweepHang + sweepComplete + sweepReset

// hangWindowBlocks is how many blocks on either side of the border
// point actively flicker during the hang — "a few genes around the
// borderline alternating between red and green."
const hangWindowBlocks = 3
const flickerSpeed = 3.0 // how fast a flickering block toggles

type laneRect struct {
	x0, x1    int
	sideIndex int // this lane's index within its own side (for the brick offset)
	slant     int // slantRising, slantFalling, or slantFlat
}

type Gene struct {
	w, h int
	time float64

	cfgSpeed float64
	cfgLabel string
}

func New(cfg config.GeneConfig) *Gene {
	return &Gene{cfgSpeed: cfg.Speed, cfgLabel: cfg.Label}
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
	g.time += dt * speed
}

func mod(a, b float64) float64 {
	m := a - float64(int(a/b))*b
	if m < 0 {
		m += b
	}
	return m
}

func (g *Gene) Draw(screen tcell.Screen) {
	if g.w <= 0 || g.h <= 0 {
		return
	}

	borderX := g.w / 2
	barLanes := g.collectBarLanes(borderX)
	sort.Slice(barLanes, func(i, j int) bool { return barLanes[i].x0 < barLanes[j].x0 })

	rowsPerBlock := barPeriod / 4
	if rowsPerBlock < 2 {
		rowsPerBlock = 2
	}
	blocksPerLane := g.h / rowsPerBlock
	if blocksPerLane < 1 {
		blocksPerLane = 1
	}
	totalBlocks := len(barLanes) * blocksPerLane
	borderBlock := totalBlocks / 2

	revealIndex, hangActive := g.sweepState(float64(borderBlock), float64(totalBlocks))

	for laneOrder, lane := range barLanes {
		g.drawBarLane(screen, lane, laneOrder, blocksPerLane, rowsPerBlock, revealIndex, hangActive, borderBlock)
	}

	g.layoutLabelsAndBorder(screen, borderX)
}

// sweepState returns the current (possibly fractional, for the
// approach/complete phases) reveal index and whether the hang-flicker
// window is currently active.
func (g *Gene) sweepState(borderBlock, totalBlocks float64) (float64, bool) {
	tc := mod(g.time, sweepTotal)
	switch {
	case tc < sweepApproach:
		return borderBlock * (tc / sweepApproach), false
	case tc < sweepApproach+sweepHang:
		return borderBlock, true
	case tc < sweepApproach+sweepHang+sweepComplete:
		f := (tc - sweepApproach - sweepHang) / sweepComplete
		return borderBlock + (totalBlocks-borderBlock)*f, false
	default:
		return 0, false
	}
}

// collectBarLanes walks both sides the same way layoutLabelsAndBorder
// does, but only records the bar lanes' rectangles — used to build the
// global left-to-right sweep order before any drawing happens.
func (g *Gene) collectBarLanes(borderX int) []laneRect {
	var lanes []laneRect
	for _, dir := range []int{-1, 1} {
		x := borderX
		laneIndex := 0
		for {
			isBarLane := laneIndex%2 == 0
			width := labelLaneCols
			if isBarLane {
				width = barLaneCols
			}
			var x0, x1 int
			if dir < 0 {
				x1, x0 = x, x-width
			} else {
				x0, x1 = x, x+width
			}
			if x1 <= 0 || x0 >= g.w {
				break
			}
			if x0 < 0 {
				x0 = 0
			}
			if x1 > g.w {
				x1 = g.w
			}
			if isBarLane {
				// Base direction mirrors by side (confirmed against the
				// reference); every 4th bar lane on a side becomes flat
				// for extra variety.
				slant := slantRising
				if dir > 0 {
					slant = slantFalling
				}
				if (laneIndex/2)%4 == 3 {
					slant = slantFlat
				}
				lanes = append(lanes, laneRect{x0: x0, x1: x1, sideIndex: laneIndex, slant: slant})
			}
			x += dir * width
			laneIndex++
			if laneIndex > 40 {
				break
			}
		}
	}
	return lanes
}

// drawBarLane renders one lane's blocks. Each block is a whole,
// single-color unit — its color depends only on its global position in
// the sweep order (laneOrder*blocksPerLane + blockRow), never on
// sub-block pixel position.
func (g *Gene) drawBarLane(screen tcell.Screen, lane laneRect, laneOrder, blocksPerLane, rowsPerBlock int, revealIndex float64, hangActive bool, borderBlock int) {
	x0, x1 := lane.x0, lane.x1
	pw := (x1 - x0) * 2
	ph := g.h * 4
	laneOffset := (lane.sideIndex / 2) * (barPeriod / 2)

	for blockRow := 0; blockRow < blocksPerLane; blockRow++ {
		globalIdx := laneOrder*blocksPerLane + blockRow
		green := g.blockColor(globalIdx, revealIndex, hangActive, borderBlock)
		core, dim := colorRedCore, colorRedDim
		if green {
			core, dim = colorGreenCore, colorGreenDim
		}

		rowStart := blockRow * rowsPerBlock
		rowEnd := rowStart + rowsPerBlock
		if rowEnd > g.h {
			rowEnd = g.h
		}

		for cy := rowStart; cy < rowEnd; cy++ {
			for cx := 0; cx < x1-x0; cx++ {
				var mask uint8
				filled := false

				for subRow := 0; subRow < 4; subRow++ {
					for subCol := 0; subCol < 2; subCol++ {
						px := cx*2 + subCol
						py := cy*4 + subRow
						if px >= pw || py >= ph {
							continue
						}
						v := fillValue(lane.slant, px, py, laneOffset)
						if float64(v) < barPeriod*barDutyCycle {
							bitOffset := scene.BrailleOffsets[subRow][subCol]
							mask |= uint8(1) << bitOffset
							filled = true
						}
					}
				}
				if !filled {
					continue
				}

				color := dim
				vCenter := fillValue(lane.slant, cx*2, cy*4+2, laneOffset)
				if float64(vCenter) < barPeriod*0.12 {
					color = core
				}

				screen.SetContent(x0+cx, cy, '\u2800'|rune(mask), nil, color.Style())
			}
		}
	}
}

// fillValue computes the bar fill-test value for one sub-pixel,
// depending on the lane's slant mode:
//   - rising/falling: periodic in px, with a py-dependent shift that
//     leans the pattern one way or the other — this is what makes it
//     read as a diagonal stroke.
//   - flat: periodic in py alone, the same for every px in the lane —
//     this is what actually produces plain horizontal bands. Reusing
//     the diagonal formula with a zero shift would NOT do this; it
//     would collapse to a single solid vertical stripe instead, since
//     that formula's periodicity is fundamentally in px, not py.
func fillValue(slant, px, py, laneOffset int) int {
	var v int
	switch slant {
	case slantFalling:
		shift := int(float64(py) / barSlope)
		v = (px + shift + laneOffset) % barPeriod
	case slantFlat:
		v = (py + laneOffset) % barPeriod
	default: // slantRising
		shift := int(float64(py) / barSlope)
		v = (px - shift + laneOffset) % barPeriod
	}
	if v < 0 {
		v += barPeriod
	}
	return v
}

// blockColor decides whether a single block (identified by its global
// sweep-order index) is green. Blocks already past the reveal point are
// solid green; blocks not yet reached are solid red; during the hang, a
// small window of blocks around the border point flicker instead.
func (g *Gene) blockColor(globalIdx int, revealIndex float64, hangActive bool, borderBlock int) bool {
	if hangActive {
		dist := globalIdx - borderBlock
		if dist < 0 {
			dist = -dist
		}
		if dist <= hangWindowBlocks {
			phase := math.Sin(g.time*flickerSpeed + float64(globalIdx)*1.7)
			return phase > 0
		}
		return globalIdx < borderBlock
	}
	return float64(globalIdx) < revealIndex
}

// layoutLabelsAndBorder draws the label lanes (with their connector
// spines) and the central border — pure layout, independent of the bar
// sweep state above.
func (g *Gene) layoutLabelsAndBorder(screen tcell.Screen, borderX int) {
	for _, dir := range []int{-1, 1} {
		x := borderX
		laneIndex := 0
		for {
			isBarLane := laneIndex%2 == 0
			width := labelLaneCols
			if isBarLane {
				width = barLaneCols
			}
			var x0, x1 int
			if dir < 0 {
				x1, x0 = x, x-width
			} else {
				x0, x1 = x, x+width
			}
			if x1 <= 0 || x0 >= g.w {
				break
			}
			if x0 < 0 {
				x0 = 0
			}
			if x1 > g.w {
				x1 = g.w
			}
			if !isBarLane {
				g.drawLabelLane(screen, x0, x1, laneIndex, dir)
			}
			x += dir * width
			laneIndex++
			if laneIndex > 40 {
				break
			}
		}
	}
	g.drawBorder(screen, borderX)
}

func (g *Gene) drawLabelLane(screen tcell.Screen, x0, x1, laneIndex, dir int) {
	width := x1 - x0
	if width <= 2 {
		return // not enough room for even a connector + one character of text
	}
	rowsPerLabel := barPeriod / 4
	if rowsPerLabel < 2 {
		rowsPerLabel = 2
	}

	base := laneIndex * 137
	dual := (laneIndex/2)%2 == 0
	style := colorLabel.Style()

	// Connector spine: a thin vertical line on the border-facing edge of
	// the lane, with a short horizontal tick reaching toward each row's
	// text — the schematic connector lines visible in the reference.
	spineX := x0
	if dir < 0 {
		spineX = x1 - 1
	}
	for cy := 0; cy < g.h; cy++ {
		if spineX >= 0 && spineX < g.w {
			screen.SetContent(spineX, cy, '\u2502', nil, style)
		}
	}

	for cy := 0; cy < g.h; cy++ {
		if cy%rowsPerLabel != 0 {
			continue
		}
		row := cy / rowsPerLabel
		var text string
		if dual {
			text = fmt.Sprintf("A%02d%03b", (base+row*2)%100, (base+row*3)%8)
		} else {
			text = fmt.Sprintf("M%03d", (base*3+row*7)%1000)
		}
		if len(text) > width-2 {
			text = text[:width-2]
		}

		var startX, tickX int
		if dir < 0 {
			startX = spineX - len(text) - 1
			tickX = spineX - 1
		} else {
			startX = spineX + 2
			tickX = spineX + 1
		}
		if tickX >= 0 && tickX < g.w {
			screen.SetContent(tickX, cy, '\u2500', nil, style)
		}
		for i, r := range text {
			x := startX + i
			if x >= x0 && x < x1 {
				screen.SetContent(x, cy, r, nil, style)
			}
		}
	}
}

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

	tipSpacing := barPeriod / 4
	if tipSpacing < 2 {
		tipSpacing = 2
	}
	for y := tipSpacing; y < g.h; y += tipSpacing {
		tip := fmt.Sprintf("B%02d", 24+y/tipSpacing)
		tx := borderX + 2
		for i, r := range tip {
			if tx+i >= 0 && tx+i < g.w {
				screen.SetContent(tx+i, y, r, nil, style)
			}
		}
	}
}
