package eva3

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/phlx0/drift/internal/config"
	"github.com/phlx0/drift/internal/scene"
)

func newTestEva3(w, h int) *Eva3 {
	e := New(config.Default().Scene.Eva3)
	e.Init(w, h, scene.Themes["cosmic"])
	return e
}

func drawOn(t *testing.T, e *Eva3, w, h int) {
	t.Helper()
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(w, h)
	e.Update(0.033)
	e.Draw(screen)
}

func TestEva3UpdateDoesNotPanic(t *testing.T) {
	e := newTestEva3(80, 24)
	for range 600 {
		e.Update(0.033)
	}
}

func TestEva3DrawDoesNotPanic(t *testing.T) {
	e := newTestEva3(80, 24)
	drawOn(t, e, 80, 24)
}

func TestEva3SmallTerminalDoesNotPanic(t *testing.T) {
	e := newTestEva3(8, 4)
	drawOn(t, e, 8, 4)
}

func TestEva3TinyTerminalDoesNotPanic(t *testing.T) {
	e := newTestEva3(1, 1)
	for range 60 {
		e.Update(0.05)
	}
	drawOn(t, e, 1, 1)
}

func TestEva3ResizeReinitsDimensions(t *testing.T) {
	e := newTestEva3(80, 24)
	e.Update(0.033)
	e.Resize(40, 12)
	if e.w != 40 || e.h != 12 {
		t.Errorf("expected w=40 h=12 after Resize, got w=%d h=%d", e.w, e.h)
	}
	drawOn(t, e, 40, 12)
}

// TestEva3FlickerEventuallyTriggers checks the flicker state machine
// actually reaches an active flicker within a reasonable time budget,
// rather than being permanently stuck at flickerTimer <= 0.
func TestEva3FlickerEventuallyTriggers(t *testing.T) {
	e := newTestEva3(80, 24)
	triggered := false
	for range 2000 {
		e.Update(0.05) // up to 100s of simulated time
		if e.flickerTimer > 0 {
			triggered = true
			break
		}
	}
	if !triggered {
		t.Error("expected a flicker to trigger within 100s of simulated time")
	}
}

// TestBitmapDataIsConsistent guards the generated bitmap_data.go file
// against corruption: every row must be exactly bitmapW characters, and
// every hex digit used must have a corresponding palette entry.
func TestBitmapDataIsConsistent(t *testing.T) {
	if len(bitmapRows) != bitmapH {
		t.Fatalf("expected %d bitmap rows, got %d", bitmapH, len(bitmapRows))
	}
	for y, row := range bitmapRows {
		if len(row) != bitmapW {
			t.Fatalf("row %d: expected %d chars, got %d", y, bitmapW, len(row))
		}
		for x := 0; x < len(row); x++ {
			idx := hexVal(row[x])
			if idx < 0 || idx >= len(bitmapPalette) {
				t.Fatalf("row %d col %d: palette index %d out of range (palette has %d entries)", y, x, idx, len(bitmapPalette))
			}
		}
	}
}

func TestSampleIndexStaysInBitmapBounds(t *testing.T) {
	pw, ph := 160, 96
	for py := 0; py < ph; py += 7 {
		for px := 0; px < pw; px += 5 {
			idx := sampleIndex(px, py, pw, ph)
			if idx < 0 || idx >= len(bitmapPalette) {
				t.Fatalf("sampleIndex(%d,%d) = %d, out of palette range", px, py, idx)
			}
		}
	}
}
