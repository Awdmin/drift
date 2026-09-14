// Package tubes renders 9 independent wireframe "tubes" weaving across
// the screen — a from-scratch design, built after directly analyzing
// the reference footage (pixel sampling + zoomed crops) rather than
// tuning-by-eye. That analysis showed the reference isn't "two crossing
// sine waves made of many wires" (which is what the helix scene in this
// app does) — it's closer to several distinct, independently-moving
// tubes, each a *tight* bundle of 4-5 lines with a small, constant
// perpendicular radius, tied together by ring cross-braces at regular
// intervals (measured at roughly 5.5% of a tube's travel per ring).
//
// The key structural difference from helix: there, a "pair"'s two lines
// share the same large amplitude but opposite phase, so they're close
// together only right at their crossing points and far apart everywhere
// else — which meant ring ties could only ever appear near crossings.
// Here, every line within one tube shares the exact same path (same
// frequency, phase, speed) offset by only a tiny constant radius, so a
// tube's lines never diverge — the rings stay visible continuously
// along the whole curve, which is what actually matches the reference.
//
// This scene is intentionally independent of helix — different file,
// different tuning, nothing shared beyond the generic braille-cell
// infrastructure every scene in this app already uses. helix is left
// untouched; this is a from-zero alternative to compare against it.
package tubes

import (
	"fmt"
	"math"

	"github.com/gdamore/tcell/v2"
	"github.com/phlx0/drift/internal/config"
	"github.com/phlx0/drift/internal/scene"
)

const headerRows = 2
const footerRows = 2

var (
	colorRuler    = scene.RGBColor{235, 225, 205}
	colorHeader   = scene.RGBColor{255, 150, 40}
	colorCrossing = scene.RGBColor{255, 245, 230}
)

// tubeColors cycles through the reference's warm/cool palette, one hue
// per tube so each of the 9 stays visually identifiable as a single
// coherent object even while they overlap.
var tubeColors = []scene.RGBColor{
	{255, 90, 190},  // hot pink
	{200, 100, 255}, // magenta
	{255, 150, 50},  // orange
	{230, 60, 50},   // red
	{80, 170, 255},  // blue
	{255, 90, 190},
	{200, 100, 255},
	{255, 150, 50},
	{80, 170, 255},
}

type tube struct {
	freq, phase, speed, baseline, amp float64
	color                             scene.RGBColor
}

type Tubes struct {
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

func New(cfg config.TubesConfig) *Tubes {
	return &Tubes{
		cfgSpeed:     cfg.Speed,
		cfgAmplitude: cfg.Amplitude,
		cfgLabel:     cfg.Label,
		cfgSubject:   cfg.Subject,
	}
}

func (tx *Tubes) Name() string { return "tubes" }

func (tx *Tubes) Init(w, h int, t scene.Theme) {
	tx.w, tx.h = w, h
	tx.theme = t
	tx.allocBuffers()
	tx.buildTubes()
}

func (tx *Tubes) Resize(w, h int) {
	tx.w, tx.h = w, h
	tx.allocBuffers()
}

func (tx *Tubes) allocBuffers() {
	bandRows := tx.h - headerRows - footerRows
	if bandRows < 1 {
		bandRows = 1
	}
	tx.pw, tx.ph = tx.w*2, bandRows*4
	tx.pixels = make([][]uint8, tx.pw)
	for i := range tx.pixels {
		tx.pixels[i] = make([]uint8, tx.ph)
	}
}

const tubeCount = 9
const linesPerTube = 4

// tubeRadius is the constant fraction-of-amplitude spacing between a
// tube's own lines — deliberately small and CONSTANT (not diverging
// with x like helix's pairs), so a tube's cross-section stays tight and
// its rings stay continuously visible along the whole curve. Needs to
// be large enough that the 4 lines actually read as separate rather
// than collapsing into one thick line — verified against a rendered
// simulation before landing on this value.
const tubeRadius = 0.11

func (tx *Tubes) buildTubes() {
	speed := tx.cfgSpeed
	if speed <= 0 {
		speed = 1.0
	}

	const baseFreq = 1.4
	const baseSpeed = 0.4

	baselines := linspace(-0.55, 0.55, tubeCount)

	tx.tubes = make([]tube, tubeCount)
	for i := 0; i < tubeCount; i++ {
		tx.tubes[i] = tube{
			freq:     baseFreq * (1.0 + 0.03*(float64(i)-float64(tubeCount)/2)),
			phase:    float64(i) * (math.Pi / float64(tubeCount)) * 1.3,
			speed:    baseSpeed * speed * (0.85 + 0.03*float64(i)),
			baseline: baselines[i],
			amp:      0.55 + 0.015*float64(i%4),
			color:    tubeColors[i%len(tubeColors)],
		}
	}
}

func (tx *Tubes) Update(dt float64) {
	tx.time += dt
}

func (tx *Tubes) Draw(screen tcell.Screen) {
	tx.drawHeader(screen)
	tx.drawFooter(screen)
	tx.drawTubes(screen)
}

func (tx *Tubes) drawHeader(screen tcell.Screen) {
	label := tx.cfgLabel
	if label == "" {
		label = "EVA-01"
	}
	subject := tx.cfgSubject
	if subject == "" {
		subject = "PILOT"
	}

	headerStyle := colorHeader.Style()
	tx.drawRightAligned(screen, 0, fmt.Sprintf("[ %s ]", label), headerStyle)
	tx.drawRightAligned(screen, 1, fmt.Sprintf("SUBJECT: %s", subject), headerStyle)

	mm := int(tx.time) / 60
	ss := int(tx.time) % 60
	counter := fmt.Sprintf("+%d:%02d  %05d", mm, ss, int(tx.time*173)%99999)
	tx.drawLeftAligned(screen, 0, counter, colorRuler.Style())
}

func (tx *Tubes) drawFooter(screen tcell.Screen) {
	tickRow := tx.h - 2
	numRow := tx.h - 1
	if tickRow < 0 || numRow >= tx.h {
		return
	}
	rulerStyle := colorRuler.Style()
	for x := 0; x < tx.w; x++ {
		if x%2 == 0 {
			screen.SetContent(x, tickRow, '-', nil, rulerStyle)
		}
	}
	const marks = 11
	for i := 0; i < marks; i++ {
		val := i - marks/2
		x := int(float64(i) / float64(marks-1) * float64(tx.w-1))
		screen.SetContent(x, tickRow, '+', nil, rulerStyle)
		label := fmt.Sprintf("%+d", val)
		lx := x - len(label)/2
		for j, r := range label {
			if lx+j >= 0 && lx+j < tx.w {
				screen.SetContent(lx+j, numRow, r, nil, rulerStyle)
			}
		}
	}
}

func (tx *Tubes) drawTubes(screen tcell.Screen) {
	for px := range tx.pixels {
		for py := range tx.pixels[px] {
			tx.pixels[px][py] = 0
		}
	}

	centerPY := tx.ph / 2
	baseAmp := float64(tx.ph) / 2 * tx.cfgAmplitude

	// Codes: each tube gets `linesPerTube` consecutive codes, so
	// tube i's lines are codes [i*linesPerTube+1 .. i*linesPerTube+linesPerTube].
	lineY := make([][]int, tubeCount*linesPerTube)
	for i, tb := range tx.tubes {
		amp := tb.amp * baseAmp
		baselinePx := int(tb.baseline * baseAmp)
		radiusOffsets := linspace(-tubeRadius, tubeRadius, linesPerTube)

		for li := 0; li < linesPerTube; li++ {
			code := i*linesPerTube + li + 1
			lineY[code-1] = make([]int, tx.pw)
			radiusPx := radiusOffsets[li] * baseAmp

			for px := 0; px < tx.pw; px++ {
				fx := float64(px) / float64(tx.pw)
				py := centerPY + baselinePx + int(radiusPx) +
					int(amp*math.Sin(fx*tb.freq*2*math.Pi+tb.phase+tx.time*tb.speed))
				lineY[code-1][px] = py
				for th := 0; th <= 1; th++ {
					yy := py + th
					if yy >= 0 && yy < tx.ph {
						tx.pixels[px][yy] = uint8(code)
					}
				}
			}
		}
	}

	// Ring cross-braces: tie each tube's outermost two lines together
	// at regular intervals. Because a tube's radius is constant, this
	// stays visible along the ENTIRE curve, unlike a divergent pair.
	ringSpacing := tx.pw / 18
	if ringSpacing < 4 {
		ringSpacing = 4
	}
	for px := 0; px < tx.pw; px += ringSpacing {
		for i := 0; i < tubeCount; i++ {
			topCode := i*linesPerTube + 1
			botCode := i*linesPerTube + linesPerTube
			y0, y1 := lineY[topCode-1][px], lineY[botCode-1][px]
			if y0 > y1 {
				y0, y1 = y1, y0
			}
			for yy := y0; yy <= y1; yy++ {
				if yy < 0 || yy >= tx.ph {
					continue
				}
				code := topCode
				if (yy-y0)%2 == 1 {
					code = botCode
				}
				if tx.pixels[px][yy] == 0 {
					tx.pixels[px][yy] = uint8(code)
				}
			}
		}
	}

	for cx := 0; cx < tx.w; cx++ {
		for cy := 0; cy < tx.h-headerRows-footerRows; cy++ {
			ch, style, ok := tx.buildBrailleCell(cx, cy)
			if ok {
				screen.SetContent(cx, headerRows+cy, ch, nil, style)
			}
		}
	}
}

func (tx *Tubes) buildBrailleCell(cx, cy int) (rune, tcell.Style, bool) {
	var mask uint8
	totalCodes := tubeCount * linesPerTube
	counts := make([]int, totalCodes+1) // 1-based; index 0 unused

	for subRow := 0; subRow < 4; subRow++ {
		for subCol := 0; subCol < 2; subCol++ {
			px := cx*2 + subCol
			py := cy*4 + subRow
			if px >= tx.pw || py >= tx.ph {
				continue
			}
			idx := tx.pixels[px][py]
			if idx == 0 {
				continue
			}
			bitOffset := scene.BrailleOffsets[subRow][subCol]
			mask |= uint8(1) << bitOffset
			if int(idx) < len(counts) {
				counts[idx]++
			}
		}
	}

	if mask == 0 {
		return 0, tcell.StyleDefault, false
	}

	best := 1
	overlapTubes := map[int]bool{}
	total := 0
	for i := 1; i <= totalCodes; i++ {
		if counts[i] > 0 {
			overlapTubes[(i-1)/linesPerTube] = true
			total += counts[i]
		}
		if counts[i] > counts[best] {
			best = i
		}
	}

	tubeIdx := (best - 1) / linesPerTube
	color := tx.tubes[tubeIdx].color

	if len(overlapTubes) > 1 {
		boost := scene.Clamp64(float64(total)/8.0, 0, 1) * 0.55
		color = scene.Lerp(color, colorCrossing, boost)
	}

	return '\u2800' | rune(mask), color.Style(), true
}

func (tx *Tubes) drawLeftAligned(screen tcell.Screen, row int, text string, style tcell.Style) {
	for i, r := range text {
		if i >= tx.w {
			break
		}
		screen.SetContent(i, row, r, nil, style)
	}
}

func (tx *Tubes) drawRightAligned(screen tcell.Screen, row int, text string, style tcell.Style) {
	start := tx.w - len(text)
	if start < 0 {
		start = 0
	}
	for i, r := range text {
		if start+i >= 0 && start+i < tx.w {
			screen.SetContent(start+i, row, r, nil, style)
		}
	}
}

// linspace returns n values evenly spaced from a to b inclusive.
func linspace(a, b float64, n int) []float64 {
	out := make([]float64, n)
	if n == 1 {
		out[0] = a
		return out
	}
	for i := 0; i < n; i++ {
		t := float64(i) / float64(n-1)
		out[i] = a + (b-a)*t
	}
	return out
}
