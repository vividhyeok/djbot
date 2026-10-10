package main

import (
	"math"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPreviewMatchesExportTransitionSamples(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	dir := t.TempDir()
	aPath, bPath := filepath.Join(dir, "a.wav"), filepath.Join(dir, "b.wav")
	if err := writeToneWAV(aPath, 20); err != nil {
		t.Fatal(err)
	}
	if err := writeToneWAV(bPath, 20); err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzeTrack(aPath, dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := AnalyzeTrack(bPath, dir)
	if err != nil {
		t.Fatal(err)
	}
	tr := TransitionSpec{AOutTime: 12, BInTime: 2, Duration: 3, SpeedA: 1, SpeedB: 1, Curve: "equal_power", GainA: 1, GainB: 1, Type: "bass_swap"}
	playlist := []TrackEntry{{Filepath: aPath, Duration: 20, LoudnessDB: a.LoudnessDB}, {Filepath: bPath, Duration: 20, LoudnessDB: b.LoudnessDB}}
	full, _, err := RenderFinalMix(playlist, []TransitionSpec{tr}, filepath.Join(dir, "full.wav"), dir)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := RenderPreview(aPath, bPath, tr, dir)
	if err != nil {
		t.Fatal(err)
	}
	x, sr, err := decodeToPCM(full)
	if err != nil {
		t.Fatal(err)
	}
	y, _, err := decodeToPCM(preview)
	if err != nil {
		t.Fatal(err)
	}
	// Preview begins at source second 4. Exclude boundary ramps/limiter warmup;
	// compare the complete transition plus one second on both sides.
	maxError := 0.0
	for i := 7 * sr; i < 12*sr; i++ {
		maxError = math.Max(maxError, math.Abs(float64(y[i]-x[i+4*sr])))
	}
	if maxError > 2e-4 {
		t.Fatalf("preview/export sample mismatch: %.8f", maxError)
	}
}
