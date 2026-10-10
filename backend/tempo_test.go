package main

import (
	"math"
	"testing"
)

func TestHalfTimeDoesNotHalvePlayback(t *testing.T) {
	a, b := syntheticAnalysis("a", 140, .5), syntheticAnalysis("b", 70, .5)
	target, sa, sb, ok := matchedTempo(a, b, .5)
	if !ok || target != 140 || sa != 1 || sb != 1 {
		t.Fatalf("target=%v speeds=%v/%v reliable=%v", target, sa, sb, ok)
	}
}

func TestMatchedBeatsHaveEqualOutputPeriod(t *testing.T) {
	a, b := syntheticAnalysis("a", 120, .5), syntheticAnalysis("b", 124, .5)
	_, sa, sb, ok := matchedTempo(a, b, .5)
	if !ok || math.Abs(60/a.BPM/sa-60/b.BPM/sb) > 1e-9 {
		t.Fatal("beat periods differ")
	}
}

func TestUncertainAndDistantTempoUseLongNaturalSpeedSequentialFade(t *testing.T) {
	for _, bpm := range []float64{0, 95, 170} {
		a, b := syntheticAnalysis("a", 120, .5), syntheticAnalysis("b", 120, .5)
		b.BPM = bpm
		cs := generateTransitionCandidates(a, b, nil, nil, 12)
		if len(cs) != 1 || cs[0].SpeedA != 1 || cs[0].SpeedB != 1 || cs[0].Duration < 16 || cs[0].Curve != "sequential" {
			t.Fatalf("unsafe fallback: %+v", cs)
		}
	}
	a, b := syntheticAnalysis("a", 120, .5), syntheticAnalysis("b", 120, .5)
	a.BPMConfidence = .1
	if _, _, _, ok := matchedTempo(a, b, .5); ok {
		t.Fatal("uncertain tempo treated as reliable")
	}
}

func TestShortTracksAlwaysHaveValidTransition(t *testing.T) {
	a, b := syntheticAnalysis("a", 120, .5), syntheticAnalysis("b", 120, .5)
	a.Duration, b.Duration = .4, .7
	a.Phrases, b.Phrases = nil, nil
	plan := GenerateMixPlanV2([]TrackAnalysis{a, b}, nil, nil, MixSettings{PreserveOrder: true})
	if len(plan.Selections) != 1 {
		t.Fatal("missing transition")
	}
	x := plan.Selections[0]
	if x.AOutTime+x.Duration*x.SpeedA > a.Duration+1e-9 || x.BInTime+x.Duration*x.SpeedB > b.Duration+1e-9 {
		t.Fatal("transition exceeds source")
	}
}
