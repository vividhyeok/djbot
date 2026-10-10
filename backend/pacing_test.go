package main

import (
	"fmt"
	"testing"
)

func TestPacingRetainsDifferentAmountsWithoutRelyingOnTempo(t *testing.T) {
	a := TrackAnalysis{Duration: 240, BPM: 120, BPMConfidence: .2}
	b := TrackAnalysis{Duration: 240, BPM: 175, BPMConfidence: .2}
	q := pacedSafeTransition(a, b, MixSettings{ListeningMode: "quick"})
	m := pacedSafeTransition(a, b, MixSettings{ListeningMode: "balanced"})
	l := pacedSafeTransition(a, b, MixSettings{ListeningMode: "long"})
	if !(q.AOutTime < m.AOutTime && m.AOutTime < l.AOutTime && l.AOutTime+l.Duration <= a.Duration) {
		t.Fatalf("pacing cues %v %v %v", q, m, l)
	}
	if q.SpeedA != 1 || q.SpeedB != 1 {
		t.Fatal("uncertain tempo must not stretch")
	}
}

func TestPacedTwelveTrackPlansAreRenderable(t *testing.T) {
	for _, mode := range []string{"quick", "balanced", "long"} {
		for _, duration := range []float64{.4, 12, 90, 180, 600} {
			for _, confidence := range []float64{.2, .9} {
				tracks := make([]TrackAnalysis, 12)
				for i := range tracks {
					tracks[i] = syntheticAnalysis(fmt.Sprint(i), 120+float64(i%3)*2, .5)
					tracks[i].Duration = duration
					tracks[i].BPMConfidence = confidence
				}
				for variation := 0; variation < 3; variation++ {
					plan := GenerateMixPlanV2(tracks, nil, nil, MixSettings{ListeningMode: mode, Variation: variation})
					entries := make([]TrackEntry, 12)
					for i, a := range plan.SortedTracks {
						entries[i] = TrackEntry{Duration: a.Duration}
					}
					if _, err := BuildRenderTimeline(entries, plan.Selections); err != nil {
						t.Fatalf("%s duration %v confidence %v: %v", mode, duration, confidence, err)
					}
				}
			}
		}
	}
}
