package eva1

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/phlx0/drift/internal/config"
	"github.com/phlx0/drift/internal/scene"
)

func newTestEva1(w, h int) *Eva1 {
	e := New(config.Default().Scene.Eva1)
	e.Init(w, h, scene.Themes["cosmic"])
	return e
}

func drawOn(t *testing.T, e *Eva1, w, h int) {
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

func TestEva1InitBuildsAllTubes(t *testing.T) {
	e := newTestEva1(80, 24)
	if len(e.tubes) != tubeCount {
		t.Errorf("expected %d tubes after Init, got %d", tubeCount, len(e.tubes))
	}
}

func TestEva1UpdateDoesNotPanic(t *testing.T) {
	e := newTestEva1(80, 24)
	for range 120 {
		e.Update(0.033)
	}
}

func TestEva1DrawDoesNotPanic(t *testing.T) {
	e := newTestEva1(80, 24)
	drawOn(t, e, 80, 24)
}

func TestEva1SmallTerminalDoesNotPanic(t *testing.T) {
	e := newTestEva1(8, 4)
	drawOn(t, e, 8, 4)
}

func TestEva1TinyTerminalDoesNotPanic(t *testing.T) {
	e := newTestEva1(1, 1)
	for range 60 {
		e.Update(0.05)
	}
	drawOn(t, e, 1, 1)
}

func TestEva1ResizeReinitsBuffers(t *testing.T) {
	e := newTestEva1(80, 24)
	e.Update(0.033)
	e.Resize(40, 12)
	if e.w != 40 || e.h != 12 {
		t.Errorf("expected w=40 h=12 after Resize, got w=%d h=%d", e.w, e.h)
	}
	drawOn(t, e, 40, 12)
}

func TestEva1PixelBufferMatchesDimensions(t *testing.T) {
	e := newTestEva1(80, 24)
	if len(e.pixels) != e.pw {
		t.Errorf("expected %d pixel columns, got %d", e.pw, len(e.pixels))
	}
	for i, col := range e.pixels {
		if len(col) != e.ph {
			t.Fatalf("column %d: expected %d rows, got %d", i, e.ph, len(col))
		}
	}
}
