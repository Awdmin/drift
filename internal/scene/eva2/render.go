package eva2

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/phlx0/drift/internal/scene"
)

// drawBarLane renders one lane's blocks. Each block is a whole,
// single-color unit — its color depends only on its global position in
// the sweep order (laneOrder*blocksPerLane + blockRow), never on
// sub-block pixel position.
func (g *Eva2) drawBarLane(screen tcell.Screen, lane laneRect, laneOrder, blocksPerLane, rowsPerBlock int, revealIndex float64, hangActive bool, borderBlock int) {
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

// layoutLabelsAndBorder draws the label lanes (with their connector
// spines) and the central border — pure layout, independent of the bar
// sweep state in eva2.go.
func (g *Eva2) layoutLabelsAndBorder(screen tcell.Screen, borderX int) {
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

func (g *Eva2) drawLabelLane(screen tcell.Screen, x0, x1, laneIndex, dir int) {
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

func (g *Eva2) drawBorder(screen tcell.Screen, borderX int) {
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
