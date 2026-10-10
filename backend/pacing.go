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
	t.TargetBPM = 120
	if a.BPMConfidence >= .55 && a.BPM >= 60 && a.BPM <= 200 {
		t.TargetBPM = a.BPM
	}
	t.Bars = defaultSettings(s).TransitionBars
	requested := float64(t.Bars) * 240 / t.TargetBPM
	t.Duration = math.Min(requested, transitionBudget(a, b, s))
	t.AOutTime = math.Min(pacedExits(a, s)[0], a.Duration-t.Duration)
	t.Curve = "sequential"
	t.Name = "박자 충돌을 줄이는 순차 페이드"
	t.Explanation = "BPM 차이 또는 분석 불확실성 때문에 속도는 유지하고, 앞 곡을 줄인 뒤 다음 곡을 올립니다. 중간 음량이 낮아질 수 있습니다. 마디는 기준 BPM으로 환산한 길이입니다."
	if t.Duration < requested-.01 {
		t.Bars = 0
		t.Explanation += " 짧은 곡/전환 간격 때문에 길이가 제한됐습니다."
	}
	t.Score = scoreTransition(a, b, t)
	return t
}

func transitionBudget(a, b TrackAnalysis, s MixSettings) float64 {
	budget := math.Min(a.Duration, b.Duration) * .4
	if s.ListeningMode == "quick" {
		budget = math.Min(budget, math.Min(exitTarget(a, s), exitTarget(b, s))*.9)
	}
	return budget
}
