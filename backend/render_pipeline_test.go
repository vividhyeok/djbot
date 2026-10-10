package main

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalRenderUsesExactTempoTimeline(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.wav"), filepath.Join(dir, "b.wav")
	if err := writeToneWAV(a, 12); err != nil {
		t.Fatal(err)
	}
	if err := writeToneWAV(b, 12); err != nil {
		t.Fatal(err)
	}
	tracks := []TrackEntry{{Filepath: a, Filename: "a", Duration: 12}, {Filepath: b, Filename: "b", Duration: 12}}
	tr := []TransitionSpec{{AOutTime: 8, BInTime: 2, Duration: 2, SpeedA: 1.05, SpeedB: .95, Curve: "equal_power", GainA: 1, GainB: 1}}
	clips, err := BuildRenderTimeline(tracks, tr)
	if err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".wav", ".mp3"} {
		out, lrc, err := RenderFinalMix(tracks, tr, filepath.Join(dir, "mix"+ext), dir)
		if err != nil {
			t.Fatal(err)
		}
		samples, sr, err := decodeToPCM(out)
		if err != nil {
			t.Fatal(err)
		}
		want := clips[1].Start + clips[1].Duration
		if diff := math.Abs(float64(len(samples))/float64(sr) - want); diff > .002 {
			t.Fatalf("%s timeline drift %fs", ext, diff)
		}
		peak := 0.0
		for _, v := range samples {
			peak = math.Max(peak, math.Abs(float64(v)))
		}
		if peak > .98 || peak < .05 {
			t.Fatalf("invalid peak %v", peak)
		}
		index, err := os.ReadFile(lrc)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(index), "[00:08.00] b") {
			t.Fatalf("wrong cue index: %s", index)
		}
	}
	if tracks[0].Filepath != a {
		t.Fatal("renderer mutated input")
	}
	left, _ := filepath.Glob(filepath.Join(dir, "render-*"))
	if len(left) > 0 {
		t.Fatal("temporary audio leaked")
	}
}

func TestRenderFailureDoesNotPublishPartialMix(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "mix.wav")
	if err := os.WriteFile(out, []byte("previous mix"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := RenderFinalMix([]TrackEntry{{Filepath: filepath.Join(dir, "missing.wav"), Duration: 10}}, nil, out, dir)
	if err == nil {
		t.Fatal("missing source accepted")
	}
	data, _ := os.ReadFile(out)
	if string(data) != "previous mix" {
		t.Fatal("existing output destroyed")
	}
}
