// Package helix renders a dense woven cable of interlaced sine wires —
// two multi-strand "ropes" 180 degrees out of phase, plus a separate
// simpler blue strand threading between them — framed by a graticule
// ruler and a corner telemetry readout. Colors are hardcoded to match the
// EVA-01 sync-graph reference rather than the shared terminal theme,
// since the reference's palette (magenta/orange/red/blue on black) is
// its own distinct look.
package helix

import (
	"fmt"
	"math"

	"github.com/gdamore/tcell/v2"
	"github.com/phlx0/drift/internal/config"
	"github.com/phlx0/drift/internal/scene"
)

const headerRows = 2 // top: "[ LABEL ]" and "SUBJECT: ..."
const footerRows = 2 // bottom: tick marks row + numbered ruler row

// Reference palette, sampled from the EVA-01 sync-graph image.
var (
	colorRuler    = scene.RGBColor{235, 225, 205} // cream/white grid + ticks
	colorHeader   = scene.RGBColor{255, 150, 40}  // orange box + subject text
	colorHotPink  = scene.RGBColor{255, 90, 190}
	colorMagenta  = scene.RGBColor{200, 100, 255}
	colorRed      = scene.RGBColor{230, 60, 50}
	colorOrange   = scene.RGBColor{255, 150, 50}
	colorBlue     = scene.RGBColor{80, 170, 255}
	colorCrossing = scene.RGBColor{255, 245, 230} // near-white, for wire overlaps
)

type strand struct {
	freq  float64
	amp   float64
	speed float64
	phase float64
	color scene.RGBColor
}

type Helix struct {
	w, h  int
	theme scene.Theme

	pw, ph int
	pixels [][]uint8

	time    float64
	strands []strand

	cfgSpeed     float64
	cfgAmplitude float64
	cfgLabel     string
	cfgSubject   string
}

func New(cfg config.HelixConfig) *Helix {
	return &Helix{
		cfgSpeed:     cfg.Speed,
		cfgAmplitude: cfg.Amplitude,
		cfgLabel:     cfg.Label,
		cfgSubject:   cfg.Subject,
	}
}

func (hx *Helix) Name() string { return "helix" }

func (hx *Helix) Init(w, h int, t scene.Theme) {
	hx.w, hx.h = w, h
	hx.theme = t
	hx.allocBuffers()
	hx.buildStrands()
}

func (hx *Helix) Resize(w, h int) {
	hx.w, hx.h = w, h
	hx.allocBuffers()
}

func (hx *Helix) allocBuffers() {
	bandRows := hx.h - headerRows - footerRows
	if bandRows < 1 {
		bandRows = 1
	}
	hx.pw, hx.ph = hx.w*2, bandRows*4
	hx.pixels = make([][]uint8, hx.pw)
	for i := range hx.pixels {
		hx.pixels[i] = make([]uint8, hx.ph)
	}
}

// buildStrands assembles two multi-wire "ropes" (each a bundle of 4
// slightly-offset wires in warm tones) plus one separate cooler-toned
// strand threading through — matching the reference's dense woven-cable
// look instead of a handful of bare sine lines.
func (hx *Helix) buildStrands() {
	speed := hx.cfgSpeed
	if speed <= 0 {
		speed = 1.0
	}

	const baseFreq = 1.0
	const baseAmpFrac = 0.97
	const baseSpeed = 0.42
	const wiresPerRope = 10

	ropeAStops := []scene.RGBColor{colorHotPink, colorMagenta, colorRed, colorOrange}
	ropeBStops := []scene.RGBColor{colorOrange, colorRed, colorMagenta, colorHotPink}

	phaseSpread := linspace(-0.42, 0.42, wiresPerRope)
	ampSpread := linspace(0.82, 1.18, wiresPerRope)

	var strands []strand

	for i := 0; i < wiresPerRope; i++ {
		strands = append(strands, strand{
			freq:  baseFreq,
			amp:   baseAmpFrac * ampSpread[i],
			speed: baseSpeed * speed,
			phase: 0 + phaseSpread[i],
			color: gradientAt(ropeAStops, i, wiresPerRope),
		})
	}
	for i := 0; i < wiresPerRope; i++ {
		strands = append(strands, strand{
			freq:  baseFreq,
			amp:   baseAmpFrac * ampSpread[i],
			speed: baseSpeed * speed,
			phase: math.Pi + phaseSpread[i],
			color: gradientAt(ropeBStops, i, wiresPerRope),
		})
	}

	// Two lone accent strands: simpler, different frequency and
	// direction, weaving through the two ropes rather than bundled with
	// either — mirroring the reference's pair of blue swoops.
	strands = append(strands,
		strand{freq: 1.35, amp: 0.60, speed: -0.30 * speed, phase: math.Pi / 2, color: colorBlue},
		strand{freq: 1.15, amp: 0.50, speed: -0.24 * speed, phase: -math.Pi / 2.6, color: colorBlue},
	)

	hx.strands = strands
}

// linspace returns n values evenly spaced from a to b inclusive (n=1
// returns just a).
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

// gradientAt samples a smooth multi-stop gradient at position i of n,
// so a rope's wire count can change independently of how many named
// colors define its gradient.
func gradientAt(stops []scene.RGBColor, i, n int) scene.RGBColor {
	if len(stops) == 1 || n == 1 {
		return stops[0]
	}
	t := float64(i) / float64(n-1)
	p := t * float64(len(stops)-1)
	lo := int(math.Floor(p))
	hi := lo + 1
	if hi >= len(stops) {
		hi = len(stops) - 1
	}
	return scene.Lerp(stops[lo], stops[hi], p-float64(lo))
}

func (hx *Helix) Update(dt float64) {
	hx.time += dt
}

func (hx *Helix) Draw(screen tcell.Screen) {
	hx.drawHeader(screen)
	hx.drawFooter(screen)
	hx.drawStrands(screen)
}

func (hx *Helix) drawHeader(screen tcell.Screen) {
	label := hx.cfgLabel
	if label == "" {
		label = "EVA-01"
	}
	subject := hx.cfgSubject
	if subject == "" {
		subject = "PILOT"
	}

	boxed := fmt.Sprintf("[ %s ]", label)
	line2 := fmt.Sprintf("SUBJECT: %s", subject)

	headerStyle := colorHeader.Style()
	hx.drawRightAligned(screen, 0, boxed, headerStyle)
	hx.drawRightAligned(screen, 1, line2, headerStyle)

	mm := int(hx.time) / 60
	ss := int(hx.time) % 60
	counter := fmt.Sprintf("+%d:%02d  %05d", mm, ss, int(hx.time*173)%99999)
	hx.drawLeftAligned(screen, 0, counter, colorRuler.Style())
}

func (hx *Helix) drawFooter(screen tcell.Screen) {
	tickRow := hx.h - 2
	numRow := hx.h - 1
	if tickRow < 0 || numRow >= hx.h {
		return
	}

	rulerStyle := colorRuler.Style()

	for x := 0; x < hx.w; x++ {
		if x%2 == 0 {
			screen.SetContent(x, tickRow, '-', nil, rulerStyle)
		}
	}

	const marks = 11 // -5 .. +5
	for i := 0; i < marks; i++ {
		val := i - marks/2
		x := int(float64(i) / float64(marks-1) * float64(hx.w-1))
		screen.SetContent(x, tickRow, '+', nil, rulerStyle)
		label := fmt.Sprintf("%+d", val)
		lx := x - len(label)/2
		for j, r := range label {
			if lx+j >= 0 && lx+j < hx.w {
				screen.SetContent(lx+j, numRow, r, nil, rulerStyle)
			}
		}
	}
}

func (hx *Helix) drawStrands(screen tcell.Screen) {
	for px := range hx.pixels {
		for py := range hx.pixels[px] {
			hx.pixels[px][py] = 0
		}
	}

	centerPY := hx.ph / 2
	baseAmp := float64(hx.ph) / 2 * hx.cfgAmplitude

	for si, s := range hx.strands {
		amp := s.amp * baseAmp
		for px := 0; px < hx.pw; px++ {
			fx := float64(px) / float64(hx.pw)
			py := centerPY + int(amp*math.Sin(fx*s.freq*2*math.Pi+s.phase+hx.time*s.speed))
			for offset := 0; offset <= 1; offset++ {
				yy := py + offset
				if yy >= 0 && yy < hx.ph {
					hx.pixels[px][yy] = uint8(si + 1)
				}
			}
		}
	}

	for cx := 0; cx < hx.w; cx++ {
		for cy := 0; cy < hx.h-headerRows-footerRows; cy++ {
			ch, style, ok := hx.buildBrailleCell(cx, cy)
			if ok {
				screen.SetContent(cx, headerRows+cy, ch, nil, style)
			}
		}
	}
}

func (hx *Helix) buildBrailleCell(cx, cy int) (rune, tcell.Style, bool) {
	var mask uint8
	counts := make([]int, len(hx.strands)+1) // 1-based; index 0 unused

	for subRow := 0; subRow < 4; subRow++ {
		for subCol := 0; subCol < 2; subCol++ {
			px := cx*2 + subCol
			py := cy*4 + subRow
			if px >= hx.pw || py >= hx.ph {
				continue
			}
			idx := hx.pixels[px][py]
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
	overlap := 0
	total := 0
	for i := 1; i <= len(hx.strands); i++ {
		if counts[i] > counts[best] {
			best = i
		}
		if counts[i] > 0 {
			overlap++
			total += counts[i]
		}
	}

	color := hx.strands[best-1].color
	if overlap > 1 {
		// Wires crossing — glow toward near-white, like the reference's
		// bright overlap highlights.
		boost := scene.Clamp64(float64(total)/8.0, 0, 1) * 0.55
		color = scene.Lerp(color, colorCrossing, boost)
	}

	return '\u2800' | rune(mask), color.Style(), true
}

func (hx *Helix) drawLeftAligned(screen tcell.Screen, row int, text string, style tcell.Style) {
	for i, r := range text {
		if i >= hx.w {
			break
		}
		screen.SetContent(i, row, r, nil, style)
	}
}

func (hx *Helix) drawRightAligned(screen tcell.Screen, row int, text string, style tcell.Style) {
	start := hx.w - len(text)
	if start < 0 {
		start = 0
	}
	for i, r := range text {
		if start+i >= 0 && start+i < hx.w {
			screen.SetContent(start+i, row, r, nil, style)
		}
	}
}
