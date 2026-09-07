// Package syncrate renders a scrolling, jittery telemetry trace with a live
// percentage readout above it — the "pilot sync graph" aesthetic: an
// irregular scope line (not a clean sine) that occasionally spikes, plus a
// slowly drifting numeric readout.
package syncrate

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/phlx0/drift/internal/config"
	"github.com/phlx0/drift/internal/scene"
)

// Pixel codes for the braille buffer.
const (
	codeEmpty    uint8 = 0
	codeBaseline uint8 = 1
	codeNormal   uint8 = 2
	codeSpike    uint8 = 3
)

// columnsPerSecond is the baseline scroll rate at cfgSpeed == 1.0.
const columnsPerSecond = 18.0

// spikeMagnitude is how far (as a fraction of half-height) a spike can kick
// the trace in one step.
const spikeMagnitude = 0.85

type SyncRate struct {
	w, h  int
	theme scene.Theme

	pw, ph int // pixel resolution of the waveform band (below the label row)

	history []float64 // one value per pixel column, range [-1, 1]
	pixels  [][]uint8 // [px][py] = code

	scrollAcc  float64
	currentVal float64

	ratio       float64 // displayed sync percentage, 0-100
	ratioTarget float64
	flashT      float64 // >0 while the label should render in the flash color

	rng *rand.Rand

	cfgSpeed       float64
	cfgVolatility  float64
	cfgSpikeChance float64
	cfgLabel       string
}

func New(cfg config.SyncRateConfig) *SyncRate {
	return &SyncRate{
		cfgSpeed:       cfg.Speed,
		cfgVolatility:  cfg.Volatility,
		cfgSpikeChance: cfg.SpikeChance,
		cfgLabel:       cfg.Label,
	}
}

func (sr *SyncRate) Name() string { return "syncrate" }

func (sr *SyncRate) Init(w, h int, t scene.Theme) {
	sr.w, sr.h = w, h
	sr.theme = t
	sr.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	sr.ratio = 30 + sr.rng.Float64()*40
	sr.ratioTarget = sr.ratio
	sr.allocBuffers()
}

func (sr *SyncRate) Resize(w, h int) {
	sr.w, sr.h = w, h
	sr.allocBuffers()
}

// allocBuffers (re)sizes the pixel/history buffers for the current terminal
// size. The top row is reserved for the text readout; everything below is
// the braille waveform band.
func (sr *SyncRate) allocBuffers() {
	bandRows := sr.h - 1
	if bandRows < 1 {
		bandRows = 1
	}
	sr.pw, sr.ph = sr.w*2, bandRows*4

	sr.pixels = make([][]uint8, sr.pw)
	for i := range sr.pixels {
		sr.pixels[i] = make([]uint8, sr.ph)
	}

	if len(sr.history) != sr.pw {
		old := sr.history
		sr.history = make([]float64, sr.pw)
		// Preserve as much of the old trace as fits, right-aligned, so a
		// resize doesn't flatline the graph.
		if len(old) > 0 {
			n := len(old)
			if n > sr.pw {
				n = sr.pw
			}
			copy(sr.history[sr.pw-n:], old[len(old)-n:])
		}
	}
}

func (sr *SyncRate) Update(dt float64) {
	if sr.rng == nil {
		return
	}

	if sr.flashT > 0 {
		sr.flashT -= dt
	}

	speed := sr.cfgSpeed
	if speed <= 0 {
		speed = 1.0
	}
	volatility := sr.cfgVolatility
	if volatility <= 0 {
		volatility = 0.5
	}

	sr.scrollAcc += dt * speed * columnsPerSecond
	for sr.scrollAcc >= 1.0 {
		sr.scrollAcc -= 1.0
		sr.stepColumn(volatility)
	}

	// Sync ratio drifts toward a slowly-wandering target, with the same
	// jitter driving small flickers so the number never looks static.
	if sr.rng.Float64() < 0.01 {
		sr.ratioTarget = 20 + sr.rng.Float64()*70
	}
	sr.ratio += (sr.ratioTarget - sr.ratio) * 0.02
	sr.ratio += sr.rng.NormFloat64() * 0.15
	sr.ratio = scene.Clamp64(sr.ratio, 0, 99.9)
}

// stepColumn advances the random-walk trace by one column and pushes it
// into the scrolling history buffer.
func (sr *SyncRate) stepColumn(volatility float64) {
	sr.currentVal += sr.rng.NormFloat64() * volatility * 0.12

	if sr.rng.Float64() < sr.cfgSpikeChance {
		sr.currentVal += (sr.rng.Float64()*2 - 1) * spikeMagnitude
		sr.flashT = 0.4
	}

	// Gentle mean-reversion keeps the trace from wandering off permanently.
	sr.currentVal *= 0.94
	sr.currentVal = scene.Clamp64(sr.currentVal, -1, 1)

	copy(sr.history, sr.history[1:])
	sr.history[len(sr.history)-1] = sr.currentVal
}

func (sr *SyncRate) Draw(screen tcell.Screen) {
	sr.drawReadout(screen)
	sr.drawTrace(screen)
}

func (sr *SyncRate) drawReadout(screen tcell.Screen) {
	label := sr.cfgLabel
	if label == "" {
		label = "SYNC RATIO"
	}
	text := fmt.Sprintf("%s  %4.1f%%", label, sr.ratio)

	style := sr.theme.Palette[0].Style()
	if sr.flashT > 0 {
		style = sr.theme.Bright.Style()
	}

	x := (sr.w - len(text)) / 2
	if x < 0 {
		x = 0
	}
	for i, r := range text {
		if x+i >= sr.w {
			break
		}
		screen.SetContent(x+i, 0, r, nil, style)
	}
}

func (sr *SyncRate) drawTrace(screen tcell.Screen) {
	for px := range sr.pixels {
		for py := range sr.pixels[px] {
			sr.pixels[px][py] = codeEmpty
		}
	}

	centerPY := sr.ph / 2
	halfPY := float64(sr.ph) / 2

	// Baseline: a faint dotted zero-line across the full width, every other
	// pixel column, so it reads as a scope guide rather than a solid rule.
	for px := 0; px < sr.pw; px += 2 {
		if centerPY >= 0 && centerPY < sr.ph {
			sr.pixels[px][centerPY] = codeBaseline
		}
	}

	for px := 0; px < sr.pw && px < len(sr.history); px++ {
		v := sr.history[px]
		py := centerPY - int(v*halfPY*0.92)

		code := codeNormal
		if scene.AbsInt(int(v*100)) > 55 {
			code = codeSpike
		}

		for offset := 0; offset <= 1; offset++ {
			yy := py + offset
			if yy >= 0 && yy < sr.ph {
				sr.pixels[px][yy] = code
			}
		}
	}

	bandY := 1 // row 0 is the text readout
	for cx := 0; cx < sr.w; cx++ {
		for cy := 0; cy < sr.h-1; cy++ {
			ch, style, ok := sr.buildBrailleCell(cx, cy)
			if ok {
				screen.SetContent(cx, bandY+cy, ch, nil, style)
			}
		}
	}
}

func (sr *SyncRate) buildBrailleCell(cx, cy int) (rune, tcell.Style, bool) {
	var mask uint8
	var counts [4]int // indexed by code

	for subRow := 0; subRow < 4; subRow++ {
		for subCol := 0; subCol < 2; subCol++ {
			px := cx*2 + subCol
			py := cy*4 + subRow
			if px >= sr.pw || py >= sr.ph {
				continue
			}
			code := sr.pixels[px][py]
			if code == codeEmpty {
				continue
			}
			bitOffset := scene.BrailleOffsets[subRow][subCol]
			mask |= uint8(1) << bitOffset
			counts[code]++
		}
	}

	if mask == 0 {
		return 0, tcell.StyleDefault, false
	}

	best := codeBaseline
	for _, c := range []uint8{codeBaseline, codeNormal, codeSpike} {
		if counts[c] > counts[best] {
			best = c
		}
	}

	var color scene.RGBColor
	switch best {
	case codeSpike:
		color = sr.theme.Bright
	case codeNormal:
		color = sr.theme.Palette[0]
	default:
		color = sr.theme.Dim[0]
	}

	return '\u2800' | rune(mask), color.Style(), true
}
