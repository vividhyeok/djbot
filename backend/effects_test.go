package main

import (
	"math"
	"os/exec"
	"path/filepath"
	"testing"
)

func segmentRMS(samples []float32, sr int, start, end float64) float64 {
	a, b := int(start*float64(sr)), int(end*float64(sr))
	sum := 0.0
	for _, v := range samples[a:b] {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum / float64(b-a))
}

func TestEQAutomationOnlyChangesTransitionAudio(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.wav")
	if err := writeToneWAV(source, 12); err != nil {
		t.Fatal(err)
	}
	clip := RenderClip{Duration: 12, Regions: []RenderRegion{{0, 12, 1, 12}}}
	tr := TransitionSpec{Duration: 4, Curve: "linear", GainA: 1, GainB: 1}
	plain := filepath.Join(dir, "plain.wav")
	eq := filepath.Join(dir, "eq.wav")
	if err := renderClip(source, 0, clip, nil, &tr, plain, filepath.Join(dir, "plain.graph")); err != nil {
		t.Fatal(err)
	}
	tr.Automation.LowA = []AutomationPoint{{0, 0}, {4, 0}}
	if err := renderClip(source, 0, clip, nil, &tr, eq, filepath.Join(dir, "eq.graph")); err != nil {
		t.Fatal(err)
	}
	a, sr, err := decodeToPCM(plain)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := decodeToPCM(eq)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(segmentRMS(a, sr, 5, 6)-segmentRMS(b, sr, 5, 6)) > 1e-6 {
		t.Fatal("EQ leaked into body")
	}
	if ratio := segmentRMS(b, sr, 9, 10) / segmentRMS(a, sr, 9, 10); ratio > .7 {
		t.Fatalf("EQ not applied: ratio %v", ratio)
	}
	if len(a) != len(b) {
		t.Fatal("effect changed duration")
	}
}

func TestFilterAutomationProcessesAudio(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.wav")
	if err := writeToneWAV(source, 3); err != nil {
		t.Fatal(err)
	}
	tspec := TransitionSpec{Duration: 3, Type: "filter_fade", Curve: "equal_power", GainA: 1, FilterCutoff: 80, FilterResonance: .7}
	clip := RenderClip{Duration: 3, Regions: []RenderRegion{{0, 3, 1, 3}}}
	if err := renderClip(source, 0, clip, nil, &tspec, filepath.Join(dir, "out.wav"), filepath.Join(dir, "filter.graph")); err != nil {
		t.Fatal(err)
	}
}
