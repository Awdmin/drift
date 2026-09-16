package eva1

import (
	"fmt"
	"math"

	"github.com/gdamore/tcell/v2"
	"github.com/phlx0/drift/internal/scene"
)

func (e *Eva1) Draw(screen tcell.Screen) {
	e.drawHeader(screen)
	e.drawFooter(screen)
	e.drawTubes(screen)
}

func (e *Eva1) drawHeader(screen tcell.Screen) {
	label := e.cfgLabel
	if label == "" {
		label = "EVA-01"
	}
	subject := e.cfgSubject
	if subject == "" {
		subject = "PILOT"
	}

	headerStyle := colorHeader.Style()
	e.drawRightAligned(screen, 0, fmt.Sprintf("[ %s ]", label), headerStyle)
	e.drawRightAligned(screen, 1, fmt.Sprintf("SUBJECT: %s", subject), headerStyle)

	mm := int(e.time) / 60
	ss := int(e.time) % 60
	counter := fmt.Sprintf("+%d:%02d  %05d", mm, ss, int(e.time*173)%99999)
	e.drawLeftAligned(screen, 0, counter, colorRuler.Style())
}

func (e *Eva1) drawFooter(screen tcell.Screen) {
	tickRow := e.h - 2
	numRow := e.h - 1
	if tickRow < 0 || numRow >= e.h {
		return
	}
	rulerStyle := colorRuler.Style()
	for x := 0; x < e.w; x++ {
		if x%2 == 0 {
			screen.SetContent(x, tickRow, '-', nil, rulerStyle)
		}
	}
	const marks = 11
	for i := 0; i < marks; i++ {
		val := i - marks/2
		x := int(float64(i) / float64(marks-1) * float64(e.w-1))
		screen.SetContent(x, tickRow, '+', nil, rulerStyle)
		label := fmt.Sprintf("%+d", val)
		lx := x - len(label)/2
		for j, r := range label {
			if lx+j >= 0 && lx+j < e.w {
				screen.SetContent(lx+j, numRow, r, nil, rulerStyle)
			}
		}
	}
}

func (e *Eva1) drawTubes(screen tcell.Screen) {
	for px := range e.pixels {
		for py := range e.pixels[px] {
			e.pixels[px][py] = 0
		}
	}

	centerPY := e.ph / 2
	baseAmp := float64(e.ph) / 2 * e.cfgAmplitude

	// Codes: each tube gets `linesPerTube` consecutive codes, so
	// tube i's lines are codes [i*linesPerTube+1 .. i*linesPerTube+linesPerTube].
	lineY := make([][]int, tubeCount*linesPerTube)
	for i, tb := range e.tubes {
		amp := tb.amp * baseAmp
		baselinePx := int(tb.baseline * baseAmp)
		radiusOffsets := linspace(-tubeRadius, tubeRadius, linesPerTube)

		for li := 0; li < linesPerTube; li++ {
			code := i*linesPerTube + li + 1
			lineY[code-1] = make([]int, e.pw)
			radiusPx := radiusOffsets[li] * baseAmp

			for px := 0; px < e.pw; px++ {
				fx := float64(px) / float64(e.pw)
				py := centerPY + baselinePx + int(radiusPx) +
					int(amp*math.Sin(fx*tb.freq*2*math.Pi+tb.phase+e.time*tb.speed))
				lineY[code-1][px] = py
				for th := 0; th <= 1; th++ {
					yy := py + th
					if yy >= 0 && yy < e.ph {
						e.pixels[px][yy] = uint8(code)
					}
				}
			}
		}
	}

	// Ring cross-braces: tie each tube's outermost two lines together
	// at regular intervals. Because a tube's radius is constant, this
	// stays visible along the ENTIRE curve, unlike a divergent pair.
	ringSpacing := e.pw / 18
	if ringSpacing < 4 {
		ringSpacing = 4
	}
	for px := 0; px < e.pw; px += ringSpacing {
		for i := 0; i < tubeCount; i++ {
			topCode := i*linesPerTube + 1
			botCode := i*linesPerTube + linesPerTube
			y0, y1 := lineY[topCode-1][px], lineY[botCode-1][px]
			if y0 > y1 {
				y0, y1 = y1, y0
			}
			for yy := y0; yy <= y1; yy++ {
				if yy < 0 || yy >= e.ph {
					continue
				}
				code := topCode
				if (yy-y0)%2 == 1 {
					code = botCode
				}
				if e.pixels[px][yy] == 0 {
					e.pixels[px][yy] = uint8(code)
				}
			}
		}
	}

	for cx := 0; cx < e.w; cx++ {
		for cy := 0; cy < e.h-headerRows-footerRows; cy++ {
			ch, style, ok := e.buildBrailleCell(cx, cy)
			if ok {
				screen.SetContent(cx, headerRows+cy, ch, nil, style)
			}
		}
	}
}

func (e *Eva1) buildBrailleCell(cx, cy int) (rune, tcell.Style, bool) {
	var mask uint8
	totalCodes := tubeCount * linesPerTube
	counts := make([]int, totalCodes+1) // 1-based; index 0 unused

	for subRow := 0; subRow < 4; subRow++ {
		for subCol := 0; subCol < 2; subCol++ {
			px := cx*2 + subCol
			py := cy*4 + subRow
			if px >= e.pw || py >= e.ph {
				continue
			}
			idx := e.pixels[px][py]
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
	color := e.tubes[tubeIdx].color

	if len(overlapTubes) > 1 {
		boost := scene.Clamp64(float64(total)/8.0, 0, 1) * 0.55
		color = scene.Lerp(color, colorCrossing, boost)
	}

	return '\u2800' | rune(mask), color.Style(), true
}

func (e *Eva1) drawLeftAligned(screen tcell.Screen, row int, text string, style tcell.Style) {
	for i, r := range text {
		if i >= e.w {
			break
		}
		screen.SetContent(i, row, r, nil, style)
	}
}

func (e *Eva1) drawRightAligned(screen tcell.Screen, row int, text string, style tcell.Style) {
	start := e.w - len(text)
	if start < 0 {
		start = 0
	}
	for i, r := range text {
		if start+i >= 0 && start+i < e.w {
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
