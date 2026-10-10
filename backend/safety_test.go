package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCacheClearPreservesProjectMaterialsAndExports(t *testing.T) {
	oldCache, oldUploads, oldOutput := cacheDir, uploadsDir, outputDir
	t.Cleanup(func() { cacheDir, uploadsDir, outputDir = oldCache, oldUploads, oldOutput })
	cacheDir = t.TempDir()
	uploadsDir, outputDir = filepath.Join(cacheDir, "uploads"), filepath.Join(cacheDir, "output")
	for _, dir := range []string{uploadsDir, outputDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	preserved := []string{filepath.Join(uploadsDir, "source.wav"), filepath.Join(outputDir, "mix.wav")}
	derived := []string{filepath.Join(cacheDir, "preview_123.mp3"), filepath.Join(cacheDir, "123_analysis_v2.json")}
	for _, path := range append(preserved, derived...) {
		if err := os.WriteFile(path, []byte("fixture"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	handleCacheClear(httptest.NewRecorder(), httptest.NewRequest("POST", "/cache/clear", nil))
	for _, path := range preserved {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("user data lost: %s", path)
		}
	}
	for _, path := range derived {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("cache retained: %s", path)
		}
	}
}

func TestShortAnalysisInputsDoNotPanic(t *testing.T) {
	if got := extractWaveform([]float32{.5}, 1000); len(got) != 1000 || got[0] != .5 {
		t.Fatal("invalid waveform")
	}
	if got := detectDownbeats([]float64{0, 1, 2, 3, 4}, nil); len(got) == 0 {
		t.Fatal("missing fallback downbeats")
	}
	if got := weightedTempo(0, 120); got != 120 {
		t.Fatal("invalid fallback tempo")
	}
}
