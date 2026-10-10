package main

import (
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGeneratedSetsHaveRenderableTimeline(t *testing.T) {
	for _, dur := range []float64{.4, 3, 12, 180} {
		for _, count := range []int{2, 3, 12} {
			tracks := make([]TrackAnalysis, count)
			for i := range tracks {
				tracks[i] = syntheticAnalysis(fmt.Sprintf("track-%d", i), 120+float64(i%4)*2, .5)
				tracks[i].Duration = dur
				if dur < 30 {
					tracks[i].Phrases = nil
					tracks[i].Downbeats = nil
					tracks[i].BeatTimes = nil
				}
			}
			p := GenerateMixPlanV2(tracks, nil, nil, MixSettings{PreserveOrder: true})
			entries := make([]TrackEntry, count)
			for i, a := range p.SortedTracks {
				entries[i] = TrackEntry{Duration: a.Duration}
			}
			if _, err := BuildRenderTimeline(entries, p.Selections); err != nil {
				t.Errorf("duration=%v count=%d: %v", dur, count, err)
			}
		}
	}
}

func TestTwelveTrackPlanRendersWithoutDrift(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.wav")
	if err := writeToneWAV(source, 3); err != nil {
		t.Fatal(err)
	}
	tracks := make([]TrackAnalysis, 12)
	for i := range tracks {
		tracks[i] = TrackAnalysis{Filepath: source, Duration: 3, BPM: 120, BPMConfidence: .2, Key: "A Minor"}
	}
	plan := GenerateMixPlanV2(tracks, nil, nil, MixSettings{PreserveOrder: true})
	entries := make([]TrackEntry, 12)
	for i := range entries {
		entries[i] = TrackEntry{Filepath: source, Filename: fmt.Sprintf("track-%d", i), Duration: 3}
	}
	clips, err := BuildRenderTimeline(entries, plan.Selections)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := RenderFinalMix(entries, plan.Selections, filepath.Join(dir, "mix.wav"), dir)
	if err != nil {
		t.Fatal(err)
	}
	pcm, sr, err := decodeToPCM(out)
	if err != nil {
		t.Fatal(err)
	}
	want := clips[11].Start + clips[11].Duration
	if math.Abs(float64(len(pcm))/float64(sr)-want) > .002 {
		t.Fatal("multi-track duration drift")
	}
	for _, v := range pcm {
		if !finite(float64(v)) || math.Abs(float64(v)) > .98 {
			t.Fatal("invalid or clipped samples")
		}
	}
	for _, clip := range clips {
		if segmentRMS(pcm, sr, clip.Start+.7, clip.Start+1.2) < .01 {
			t.Fatal("missing track audio")
		}
	}
}
