package eva2

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/phlx0/drift/internal/config"
	"github.com/phlx0/drift/internal/scene"
)

func newTestEva2(w, h int) *Eva2 {
	e := New(config.Default().Scene.Eva2)
	e.Init(w, h, scene.Themes["cosmic"])
	return e
}

func drawOn(t *testing.T, e *Eva2, w, h int) {
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

func TestEva2UpdateDoesNotPanic(t *testing.T) {
	e := newTestEva2(80, 24)
	for range 120 {
		e.Update(0.033)
	}
}

func TestEva2DrawDoesNotPanic(t *testing.T) {
	e := newTestEva2(80, 24)
	drawOn(t, e, 80, 24)
}

func TestEva2SmallTerminalDoesNotPanic(t *testing.T) {
	e := newTestEva2(8, 4)
	drawOn(t, e, 8, 4)
}

func TestEva2TinyTerminalDoesNotPanic(t *testing.T) {
	e := newTestEva2(1, 1)
	for range 60 {
		e.Update(0.05)
	}
	drawOn(t, e, 1, 1)
}

// TestEva2OddWidthsDoNotPanic is a regression test: a label lane
// clipped down to width 1 at certain terminal widths used to panic on
// a negative slice bound (text[:width-2] with width==1). Sweep a range
// of widths, including ones known to land a lane sliver right at the
// screen edge, to make sure that can't come back.
func TestEva2OddWidthsDoNotPanic(t *testing.T) {
	for w := 1; w <= 120; w++ {
		e := newTestEva2(w, 24)
		drawOn(t, e, w, 24)
	}
}

func TestEva2ResizeReinitsDimensions(t *testing.T) {
	e := newTestEva2(80, 24)
	e.Update(0.033)
	e.Resize(40, 12)
	if e.w != 40 || e.h != 12 {
		t.Errorf("expected w=40 h=12 after Resize, got w=%d h=%d", e.w, e.h)
	}
	drawOn(t, e, 40, 12)
}

func TestEva2SweepStateStaysWithinBounds(t *testing.T) {
	e := newTestEva2(80, 24)
	for range 2000 {
		e.Update(0.05)
		revealIndex, _ := e.sweepState(50, 100)
		if revealIndex < 0 || revealIndex > 100 {
			t.Fatalf("sweepState returned out-of-range reveal index: %f", revealIndex)
		}
	}
}
