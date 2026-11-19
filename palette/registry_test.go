package palette

import (
	"fmt"
	"sync"
	"testing"

	"github.com/timzifer/figure/ir"
)

// A registration beside a lookup on another goroutine is what a program that
// registers a palette while a spec is being written does. Under -race this
// fails without the registry's lock; without -race it can still die with
// "concurrent map read and map write".
func TestTheRegistriesCanBeUsedConcurrently(t *testing.T) {
	var wg sync.WaitGroup
	for g := range 4 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := range 50 {
				name := fmt.Sprintf("concurrent-%d-%d", g, i)
				RegisterRamp(name, Ramp{ir.RGB(0, 0, 0), ir.RGB(255, 255, 255)})
				RegisterQualitative(name, Qualitative{ir.RGB(1, 2, 3)})
				RegisterMatrix(name, [][]ir.Color{{ir.RGB(1, 2, 3)}})
			}
		}()
		go func() {
			defer wg.Done()
			for range 50 {
				RampByName("viridis")
				RampName(Viridis)
				RampNames()
				QualitativeByName("okabeito")
				QualitativeName(OkabeIto)
				QualitativeNames()
				MatrixByName("bluered")
				MatrixName(BivariateBlueRed)
			}
		}()
	}
	wg.Wait()
}

// Editing what a lookup handed back must not repaint the built-in for every
// chart in the process.
func TestALookupIsACopy(t *testing.T) {
	want := Viridis[0]
	r, ok := RampByName("viridis")
	if !ok {
		t.Fatal("viridis is not registered")
	}
	r[0] = ir.RGB(255, 0, 0)
	if Viridis[0] != want {
		t.Errorf("editing RampByName's result changed Viridis[0] to %v", Viridis[0])
	}
	if again, _ := RampByName("viridis"); again[0] != want {
		t.Errorf("editing RampByName's result changed the registered ramp to %v", again[0])
	}

	wantQ := OkabeIto[0]
	q, _ := QualitativeByName("okabeito")
	q[0] = ir.RGB(255, 0, 0)
	if OkabeIto[0] != wantQ {
		t.Errorf("editing QualitativeByName's result changed OkabeIto[0] to %v", OkabeIto[0])
	}

	wantM := BivariateBlueRed[0][0]
	m, _ := MatrixByName("bluered")
	m[0][0] = ir.RGB(255, 0, 0)
	if BivariateBlueRed[0][0] != wantM {
		t.Errorf("editing a row of MatrixByName's result changed BivariateBlueRed[0][0] to %v", BivariateBlueRed[0][0])
	}
}

// A caller who registers a slice and goes on editing it must not change the
// registered entry behind the registry's back.
func TestARegistrationIsACopy(t *testing.T) {
	r := Ramp{ir.RGB(10, 20, 30), ir.RGB(40, 50, 60)}
	RegisterRamp("copied-ramp", r)
	r[0] = ir.RGB(255, 0, 0)
	if got, _ := RampByName("copied-ramp"); got[0] != ir.RGB(10, 20, 30) {
		t.Errorf("editing a registered ramp afterwards changed the registry to %v", got[0])
	}

	q := Qualitative{ir.RGB(10, 20, 30)}
	RegisterQualitative("copied-qualitative", q)
	q[0] = ir.RGB(255, 0, 0)
	if got, _ := QualitativeByName("copied-qualitative"); got[0] != ir.RGB(10, 20, 30) {
		t.Errorf("editing a registered palette afterwards changed the registry to %v", got[0])
	}

	m := [][]ir.Color{{ir.RGB(10, 20, 30)}}
	RegisterMatrix("copied-matrix", m)
	m[0][0] = ir.RGB(255, 0, 0)
	if got, _ := MatrixByName("copied-matrix"); got[0][0] != ir.RGB(10, 20, 30) {
		t.Errorf("editing a registered matrix afterwards changed the registry to %v", got[0][0])
	}
}
