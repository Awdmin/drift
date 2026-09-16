// Package eva1 renders 9 independent wireframe "tubes" weaving across
// the screen, in the style of the Evangelion sync-graph display — a
// design built by directly analyzing reference footage (pixel sampling
// and zoomed crops) rather than tuning by eye. That analysis showed the
// underlying shape isn't simply "two crossing sine waves made of many
// wires" — it's closer to several distinct, independently-moving tubes,
// each a *tight* bundle of 4-5 lines with a small, constant
// perpendicular radius, tied together by ring cross-braces at regular
// intervals (measured at roughly 5.5% of a tube's travel per ring).
//
// The radius being small and constant (rather than the two lines of a
// pair sharing one large amplitude at opposite phase, which would only
// bring them close together right at their crossing points) is what
// keeps the rings visible continuously along the whole curve, matching
// the reference, instead of only near crossings.
//
// This file holds core state, lifecycle, and tube-building logic; the
// actual drawing functions (header, footer, tube rendering, braille
// cells) live in render.go.
package eva1

import (
	"math"

	"github.com/phlx0/drift/internal/config"
	"github.com/phlx0/drift/internal/scene"
)

const headerRows = 2
const footerRows = 2

var (
	colorRuler    = scene.RGBColor{R: 235, G: 225, B: 205}
	colorHeader   = scene.RGBColor{R: 255, G: 150, B: 40}
	colorCrossing = scene.RGBColor{R: 255, G: 245, B: 230}
)

// tubeColors cycles through the reference's warm/cool palette, one hue
// per tube so each of the 9 stays visually identifiable as a single
// coherent object even while they overlap.
var tubeColors = []scene.RGBColor{
	{R: 255, G: 90, B: 190},  // hot pink
	{R: 200, G: 100, B: 255}, // magenta
	{R: 255, G: 150, B: 50},  // orange
	{R: 230, G: 60, B: 50},   // red
	{R: 80, G: 170, B: 255},  // blue
	{R: 255, G: 90, B: 190},
	{R: 200, G: 100, B: 255},
	{R: 255, G: 150, B: 50},
	{R: 80, G: 170, B: 255},
}

type tube struct {
	freq, phase, speed, baseline, amp float64
	color                             scene.RGBColor
}

type Eva1 struct {
	w, h  int
	theme scene.Theme

	pw, ph int
	pixels [][]uint8

	time  float64
	tubes []tube

	cfgSpeed     float64
	cfgAmplitude float64
	cfgLabel     string
	cfgSubject   string
}

func New(cfg config.Eva1Config) *Eva1 {
	return &Eva1{
		cfgSpeed:     cfg.Speed,
		cfgAmplitude: cfg.Amplitude,
		cfgLabel:     cfg.Label,
		cfgSubject:   cfg.Subject,
	}
}

func (e *Eva1) Name() string { return "eva1" }

func (e *Eva1) Init(w, h int, t scene.Theme) {
	e.w, e.h = w, h
	e.theme = t
	e.allocBuffers()
	e.buildTubes()
}

func (e *Eva1) Resize(w, h int) {
	e.w, e.h = w, h
	e.allocBuffers()
}

func (e *Eva1) allocBuffers() {
	bandRows := e.h - headerRows - footerRows
	if bandRows < 1 {
		bandRows = 1
	}
	e.pw, e.ph = e.w*2, bandRows*4
	e.pixels = make([][]uint8, e.pw)
	for i := range e.pixels {
		e.pixels[i] = make([]uint8, e.ph)
	}
}

const tubeCount = 9
const linesPerTube = 4

// tubeRadius is the constant fraction-of-amplitude spacing between a
// tube's own lines — deliberately small and CONSTANT (not diverging
// with x, which would only bring lines close together near crossing
// points), so a tube's cross-section stays tight and its rings stay
// continuously visible along the whole curve. Needs to be large enough
// that the 4 lines actually read as separate rather than collapsing
// into one thick line — verified against a rendered
// simulation before landing on this value.
const tubeRadius = 0.11

func (e *Eva1) buildTubes() {
	speed := e.cfgSpeed
	if speed <= 0 {
		speed = 1.0
	}

	const baseFreq = 1.4
	const baseSpeed = 0.4

	baselines := linspace(-0.55, 0.55, tubeCount)

	e.tubes = make([]tube, tubeCount)
	for i := 0; i < tubeCount; i++ {
		e.tubes[i] = tube{
			freq:     baseFreq * (1.0 + 0.03*(float64(i)-float64(tubeCount)/2)),
			phase:    float64(i) * (math.Pi / float64(tubeCount)) * 1.3,
			speed:    baseSpeed * speed * (0.85 + 0.03*float64(i)),
			baseline: baselines[i],
			amp:      0.55 + 0.015*float64(i%4),
			color:    tubeColors[i%len(tubeColors)],
		}
	}
}

func (e *Eva1) Update(dt float64) {
	e.time += dt
}
