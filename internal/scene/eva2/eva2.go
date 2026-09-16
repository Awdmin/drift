// Package eva2 renders a lane-based technical readout: alternating
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
//
// This file holds core state, lifecycle, and sweep-timing logic; the
// actual drawing functions (bar lanes, label lanes, border) live in
// render.go.
package eva2

import (
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
// was backwards in an earlier comment — increasing it moves toward
// vertical/upright).
// A shallow (low) slope needs a wider lane to render as one clean
// diagonal stroke instead of wrapping into choppy segments — that's why
// barLaneCols is as wide as it is alongside this value.
const barSlope = 1.3

const barLaneCols = 9   // widened to fit the shallower slope cleanly (see barSlope)
const labelLaneCols = 9 // shortened labels (see drawLabelLane) need less room now

// Slant variety, confirmed by zooming into the reference: the green
// (left) side's bars lean one way and the red (right) side's lean the
// opposite way — a mirror across the border, not the same direction on
// both sides. A flat/horizontal variant is mixed in occasionally too,
// for variety beyond the strict mirror.
const (
	slantRising  = iota // leans "/" — bottom-left to top-right
	slantFalling        // leans "\" — top-left to bottom-right
	slantFlat           // no lean at all — plain horizontal bands
)

var (
	colorGreenCore = scene.RGBColor{R: 60, G: 255, B: 140}
	colorGreenDim  = scene.RGBColor{R: 10, G: 90, B: 45}
	colorRedCore   = scene.RGBColor{R: 255, G: 60, B: 55}
	colorRedDim    = scene.RGBColor{R: 90, G: 15, B: 15}
	colorBorder    = scene.RGBColor{R: 140, G: 255, B: 210}
	colorLabel     = scene.RGBColor{R: 230, G: 150, B: 60}
)

// Sweep timing, in seconds.
const sweepApproach = 4.0 // 0 -> border, revealing block by block
const sweepHang = 14.0    // long dwell: a few blocks near the border flicker
const sweepComplete = 4.0 // border -> end, revealing block by block
const sweepReset = 0.6    // snap back to all-red
const sweepTotal = sweepApproach + sweepHang + sweepComplete + sweepReset

// hangWindowBlocks is how many blocks on either side of the border
// point actively flicker during the hang — a handful of blocks near the
// borderline alternating between red and green.
const hangWindowBlocks = 3
const flickerSpeed = 3.0 // how fast a flickering block toggles

type laneRect struct {
	x0, x1    int
	sideIndex int // this lane's index within its own side (for the brick offset)
	slant     int // slantRising, slantFalling, or slantFlat
}

type Eva2 struct {
	w, h int
	time float64

	cfgSpeed float64
	cfgLabel string
}

func New(cfg config.Eva2Config) *Eva2 {
	return &Eva2{cfgSpeed: cfg.Speed, cfgLabel: cfg.Label}
}

func (g *Eva2) Name() string { return "eva2" }

func (g *Eva2) Init(w, h int, t scene.Theme) {
	g.w, g.h = w, h
}

func (g *Eva2) Resize(w, h int) {
	g.w, g.h = w, h
}

func (g *Eva2) Update(dt float64) {
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

func (g *Eva2) Draw(screen tcell.Screen) {
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
func (g *Eva2) sweepState(borderBlock, totalBlocks float64) (float64, bool) {
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
func (g *Eva2) collectBarLanes(borderX int) []laneRect {
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

// blockColor decides whether a single block (identified by its global
// sweep-order index) is green. Blocks already past the reveal point are
// solid green; blocks not yet reached are solid red; during the hang, a
// small window of blocks around the border point flicker instead.
func (g *Eva2) blockColor(globalIdx int, revealIndex float64, hangActive bool, borderBlock int) bool {
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
