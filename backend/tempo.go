package main

import "math"

// matchedTempo treats half/double-time detections as metrical alternatives,
// never as a request to play a recording at half or double speed.
func matchedTempo(a, b TrackAnalysis, flexibility float64) (target, speedA, speedB float64, reliable bool) {
	if a.BPM < 30 || b.BPM < 30 || a.BPM > 300 || b.BPM > 300 ||
		!finite(a.BPM) || !finite(b.BPM) || a.BPMConfidence < .55 || b.BPMConfidence < .55 {
		return 120, 1, 1, false
	}
	effectiveB := b.BPM
	best := math.Abs(math.Log(b.BPM / a.BPM))
	for _, scale := range []float64{.5, 2} {
		v := b.BPM * scale
		if d := math.Abs(math.Log(v / a.BPM)); d < best {
			best, effectiveB = d, v
		}
	}
	target = math.Sqrt(a.BPM * effectiveB)
	speedA, speedB = target/a.BPM, target/effectiveB
	limit := .025 + .055*clampF(flexibility, 0, 1)
	if math.Abs(speedA-1) > limit || math.Abs(speedB-1) > limit {
		return target, 1, 1, false
	}
	return target, speedA, speedB, true
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func safeTransition(a, b TrackAnalysis) TransitionSpec {
	d := math.Min(2, math.Min(a.Duration, b.Duration)*.2)
	t := TransitionSpec{ID: "safe-crossfade", Type: "crossfade", Name: "Short safe crossfade",
		Duration: d, AOutTime: math.Max(0, a.Duration-d), BInTime: 0,
		SpeedA: 1, SpeedB: 1, TargetBPM: 120, Curve: "equal_power", GainA: 1, GainB: 1}
	t.Score = scoreTransition(a, b, t)
	return t
}
