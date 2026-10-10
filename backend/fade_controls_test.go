package main

import (
	"math"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRequestedBarsDoNotFallBackToOneSecond(t *testing.T) {
	for _, bars := range []int{8, 16, 32} {
		for _, confidence := range []float64{.1, .9} {
			a, b := syntheticAnalysis("a", 120, .5), syntheticAnalysis("b", 120, .5)
			a.Duration, b.Duration = 240, 240
			a.BPMConfidence = confidence
			p := GenerateMixPlanV2([]TrackAnalysis{a, b}, nil, nil, MixSettings{TransitionBars: bars, ListeningMode: "balanced", PreserveOrder: true})
			tr := p.Selections[0]
			if math.Abs(tr.Duration-float64(bars)*2) > .001 {
				t.Fatalf("bars %d confidence %v: got %.2fs", bars, confidence, tr.Duration)
			}
		}
	}
}

func TestIndependentFadeControlsChangeActualPCM(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.wav")
	if err := writeToneWAV(source, 4); err != nil {
		t.Fatal(err)
	}
	clip := RenderClip{Duration: 4, Regions: []RenderRegion{{0, 4, 1, 4}}}
	rate := 0
	render := func(incoming bool, fade float64) []float32 {
		tr := TransitionSpec{Duration: 4, Curve: "equal_power", GainA: 1, GainB: 1, FadeInSeconds: fade, FadeOutSeconds: fade}
		var in, out *TransitionSpec
		if incoming {
			in = &tr
		} else {
			out = &tr
		}
		path := filepath.Join(dir, "render.wav")
		if err := renderClip(source, 0, clip, in, out, path, filepath.Join(dir, "graph")); err != nil {
			t.Fatal(err)
		}
		pcm, sr, err := decodeToPCM(path)
		rate = sr
		if err != nil {
			t.Fatal(err)
		}
		return pcm
	}
	rms := func(p []float32) float64 {
		var sum float64
		for _, v := range p[2*rate : 2*rate+rate/5] {
			sum += float64(v) * float64(v)
		}
		return math.Sqrt(sum / float64(rate/5))
	}
	for _, incoming := range []bool{false, true} {
		full, short := render(incoming, 4), render(incoming, 1)
		if rms(full) < .01 || rms(short) > .00001 {
			t.Fatalf("incoming=%v full RMS %.5f short RMS %.5f", incoming, rms(full), rms(short))
		}
	}
}

func TestFadeCannotExceedTransition(t *testing.T) {
	_, err := BuildRenderTimeline([]TrackEntry{{Duration: 20}, {Duration: 20}}, []TransitionSpec{{AOutTime: 10, Duration: 4, FadeOutSeconds: 5}})
	if err == nil {
		t.Fatal("invalid fade accepted")
	}
}
