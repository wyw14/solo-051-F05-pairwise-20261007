package optical

import (
	"testing"
	"time"

	"github.com/wyw14/cry-161/internal/model"
)

func sample(runID model.ID, nm float64) model.ThicknessSample {
	return model.ThicknessSample{RunID: runID, Frequency: 5, Transmission: 0.9, Nanometers: nm, Valid: true, At: time.Now().UTC()}
}

func guard() LayerGuard {
	return LayerGuard{TargetNm: 100, ThresholdNm: 5, ConsecutiveAnomalies: 2, RecoveryWindow: 2}
}

func TestWindowDefaultsToValidWithoutGuard(t *testing.T) {
	w := NewThicknessWindow(8)
	if got := w.Add(sample("r1", 500), 1); got.Status != SampleValid {
		t.Fatalf("expected valid without guard, got %s", got.Status)
	}
}

func TestWindowFlagsMissingSamples(t *testing.T) {
	w := NewThicknessWindow(8)
	w.ConfigureLayer(1, guard())
	bad := sample("r1", 100)
	bad.Valid = false
	if got := w.Add(bad, 1); got.Status != SampleMissing {
		t.Fatalf("expected missing, got %s", got.Status)
	}
	if got := w.Add(sample("r1", 100), 1); got.Status != SampleValid {
		t.Fatalf("missing sample must not disturb counters, got %s", got.Status)
	}
}

func TestWindowOutlierBelowTriggerDoesNotRecover(t *testing.T) {
	w := NewThicknessWindow(8)
	w.ConfigureLayer(1, guard())
	if got := w.Add(sample("r1", 120), 1); got.Status != SampleOutlier {
		t.Fatalf("expected outlier, got %s", got.Status)
	}
	if got := w.Add(sample("r1", 100), 1); got.Status != SampleValid {
		t.Fatalf("single outlier must not start recovery, got %s", got.Status)
	}
}

func TestWindowRecoveryAfterConsecutiveAnomalies(t *testing.T) {
	w := NewThicknessWindow(8)
	w.ConfigureLayer(1, guard())
	w.Add(sample("r1", 120), 1)
	if got := w.Add(sample("r1", 130), 1); got.Status != SampleOutlier {
		t.Fatalf("expected outlier, got %s", got.Status)
	}
	if got := w.Add(sample("r1", 100), 1); got.Status != SampleRecovering {
		t.Fatalf("expected recovering after consecutive anomalies, got %s", got.Status)
	}
	if got := w.Add(sample("r1", 100), 1); got.Status != SampleValid {
		t.Fatalf("expected valid once recovery window is satisfied, got %s", got.Status)
	}
}

func TestWindowOutlierDuringRecoveryRestartsWindow(t *testing.T) {
	w := NewThicknessWindow(8)
	w.ConfigureLayer(1, guard())
	w.Add(sample("r1", 120), 1)
	w.Add(sample("r1", 130), 1)
	w.Add(sample("r1", 100), 1)
	if got := w.Add(sample("r1", 140), 1); got.Status != SampleOutlier {
		t.Fatalf("expected outlier during recovery, got %s", got.Status)
	}
	if got := w.Add(sample("r1", 100), 1); got.Status != SampleRecovering {
		t.Fatalf("expected recovery window to restart, got %s", got.Status)
	}
	if got := w.Add(sample("r1", 100), 1); got.Status != SampleValid {
		t.Fatalf("expected valid after restarted window, got %s", got.Status)
	}
}

func TestWindowKeepsOutlierEvidence(t *testing.T) {
	w := NewThicknessWindow(8)
	w.ConfigureLayer(1, guard())
	w.Add(sample("r1", 120), 1)
	assessed := w.AssessedForRun("r1")
	if len(assessed) != 1 || assessed[0].Status != SampleOutlier {
		t.Fatalf("expected one outlier entry, got %+v", assessed)
	}
	if assessed[0].Sample.Nanometers != 120 {
		t.Fatalf("outlier sample must keep original evidence, got %.3f", assessed[0].Sample.Nanometers)
	}
	if values := w.Values(); len(values) != 1 || values[0].Nanometers != 120 {
		t.Fatalf("Values must keep raw samples, got %+v", values)
	}
}

func TestWindowReconfigureResetsLayerState(t *testing.T) {
	w := NewThicknessWindow(8)
	w.ConfigureLayer(1, guard())
	w.Add(sample("r1", 120), 1)
	w.ConfigureLayer(1, guard())
	if got := w.Add(sample("r1", 130), 1); got.Status != SampleOutlier {
		t.Fatalf("identical guard must keep counters, got %s", got.Status)
	}
	w.ConfigureLayer(1, LayerGuard{TargetNm: 100, ThresholdNm: 50})
	if got := w.Add(sample("r1", 130), 1); got.Status != SampleValid {
		t.Fatalf("changed guard must reset state and widen threshold, got %s", got.Status)
	}
}
