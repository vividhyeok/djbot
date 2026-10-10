package main

import "math"

// Listening pace controls source retention, independently of fade duration.
// Even uncertain tempo uses musical cue candidates, not the physical file end.
func exitTarget(t TrackAnalysis, s MixSettings) float64 {
	target := math.Max(t.Duration*.75, t.Duration-16)
	switch s.ListeningMode {
	case "quick":
		target = math.Min(75, t.Duration*.55)
	case "balanced":
		target = math.Min(150, t.Duration*.78)
	}
	return math.Max(0, math.Min(target, t.Duration*.94))
}

func pacedExits(t TrackAnalysis, s MixSettings) []float64 {
	if s.ListeningMode == "" {
		return anchors(t, false)
	}
	target := exitTarget(t, s)
	best := target
	distance := math.Min(12, t.Duration*.08)
	for _, x := range t.Phrases {
		if x >= t.Duration*.25 && math.Abs(x-target) < distance {
			best, distance = x, math.Abs(x-target)
		}
	}
	return []float64{best}
}

func pacedSafeTransition(a, b TrackAnalysis, s MixSettings) TransitionSpec {
	t := safeTransition(a, b)
	if s.ListeningMode != "" {
		t.AOutTime = math.Min(pacedExits(a, s)[0], a.Duration-t.Duration)
		t.Name = "Tempo-safe crossfade"
		t.Score = scoreTransition(a, b, t)
	}
	return t
}
