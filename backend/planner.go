package main

import (
	"math"
	"math/rand"
	"sort"
)

// ComputePlayBounds sets PlayStart / PlayEnd on each TrackEntry.
// PlayStart: where in the track to start playing from (BInTime from transition).
// PlayEnd: where in the track to stop playing (always full Duration - we let the
//
//	renderer handle overlaps using the xfade duration from the TransitionSpec).
func ComputePlayBounds(playlist []TrackWithAnalysis, transitions []TransitionSpec) []TrackEntry {
	n := len(playlist)
	entries := make([]TrackEntry, n)
	for i, t := range playlist {
		entries[i] = TrackEntry{
			Filepath: t.Analysis.Filepath,
			Filename: t.Filename,
			Duration: t.Analysis.Duration,
			BPM:      t.Analysis.BPM,
		}
	}
	if n == 0 {
		return entries
	}

	// First track: find a good starting point (intro or first highlight)
	firstStart := func(ta TrackAnalysis) float64 {
		dur := ta.Duration
		beats := ta.BeatTimes
		anchor := 0.0
		if len(ta.Highlights) > 0 {
			anchor = ta.Highlights[0].StartTime
		} else if len(ta.Segments) > 0 {
			for _, s := range ta.Segments {
				if s.Label == "Intro" || s.Label == "Verse" {
					anchor = s.Time
					break
				}
			}
		}
		// Snap to nearest beat
		if len(beats) > 0 && anchor > 0 {
			best, bestD := 0, math.Abs(beats[0]-anchor)
			for i, b := range beats {
				if d := math.Abs(b - anchor); d < bestD {
					bestD = d
					best = i
				}
			}
			anchor = beats[best]
		}
		if anchor > dur*0.5 {
			anchor = 0 // fallback: play from beginning if analysis misidentified
		}
		return anchor
	}

	// First track starts at intro, plays to end
	entries[0].PlayStart = firstStart(playlist[0].Analysis)
	entries[0].PlayEnd = playlist[0].Analysis.Duration

	// Middle and last tracks: start at BInTime (where DJ "drops in"), play to end
	for i := 1; i < n; i++ {
		if i-1 < len(transitions) {
			bIn := transitions[i-1].BInTime
			dur := playlist[i].Analysis.Duration
			// Clamp: BInTime must be within 0..Duration-15
			if bIn < 0 {
				bIn = 0
			}
			if bIn > dur-15.0 {
				bIn = math.Max(0, dur-15.0)
			}
			entries[i].PlayStart = bIn
		} else {
			entries[i].PlayStart = 0
		}
		entries[i].PlayEnd = playlist[i].Analysis.Duration
	}

	return entries
}

// GenerateMixPlan takes analyzed tracks and produces a full mix plan
// (transition candidates + best selections), mirroring app.py SMART MIX logic.
func GenerateMixPlan(tracks []TrackAnalysis, typeWeights map[string]float64, barWeights map[int]float64, numScenarios int) MixPlan {
	if len(tracks) < 2 {
		return MixPlan{}
	}

	// Sort by harmonic/energy flow (greedy nearest-neighbour)
	sorted := sortPlaylist(tracks)

	// Default weights
	if typeWeights == nil {
		typeWeights = map[string]float64{
			"crossfade":   0.5,
			"bass_swap":   1.6,
			"cut":         1.2,
			"filter_fade": 1.0,
			"mashup":      1.0,
		}
	}
	if barWeights == nil {
		barWeights = map[int]float64{4: 1.0, 8: 1.3}
	}

	bestScore := math.Inf(-1)
	var bestSelections []TransitionSpec
	var bestCandidates [][]TransitionSpec

	if numScenarios <= 0 {
		numScenarios = 5
	}

	for s := 0; s < numScenarios; s++ {
		scenarioScore := 0.0
		var scenarioCands [][]TransitionSpec
		var scenarioSels []TransitionSpec
		minExitTime := 0.0

		for i := 0; i < len(sorted)-1; i++ {
			cands := generateCandidates(sorted[i], sorted[i+1], typeWeights, barWeights, 8)
			best := selectBest(cands, typeWeights, barWeights, minExitTime)
			if best != nil {
				scenarioScore += typeWeights[best.Type]
				scenarioSels = append(scenarioSels, *best)
				minExitTime = best.BInTime
			}
			scenarioCands = append(scenarioCands, cands)
		}

		if scenarioScore > bestScore {
			bestScore = scenarioScore
			bestSelections = scenarioSels
			bestCandidates = scenarioCands
		}
	}

	return MixPlan{
		SortedTracks: sorted,
		Candidates:   bestCandidates,
		Selections:   bestSelections,
	}
}

// --- Internal helpers ---

func idealEnergy(position float64) float64 {
	// Bell curve: 0 -> 0.7 rising, 0.7 -> 1.0 falling
	return math.Sin(position*math.Pi*0.9)*0.6 + 0.4
}

func sortPlaylist(tracks []TrackAnalysis) []TrackAnalysis {
	if len(tracks) == 0 {
		return tracks
	}

	// Precompute avgEnergy for every track once — avoids O(n² × energy_len)
	// repeated scans inside the greedy nearest-neighbour loop below.
	preEnergy := make(map[string]float64, len(tracks))
	for _, t := range tracks {
		preEnergy[t.Filepath] = avgEnergy(t)
	}

	sorted := []TrackAnalysis{tracks[0]}
	remaining := make([]TrackAnalysis, len(tracks)-1)
	copy(remaining, tracks[1:])

	for len(remaining) > 0 {
		current := sorted[len(sorted)-1]
		bestIdx := 0
		bestScore := math.Inf(-1)

		// ── Phase 3 (C-2): Energy Arc position calculation ──
		position := float64(len(sorted)) / float64(len(tracks))
		targetEnergy := idealEnergy(position)

		for i, t := range remaining {
			score := 0.0
			keyDist := camelotDistance(current.Key, t.Key)
			// Perfect match = 0, relative = 10, completely off = 60+ penalty
			score += math.Max(0, 60-float64(keyDist))

			bpmDiff := math.Abs(t.BPM - current.BPM)
			score += math.Max(0, 20-bpmDiff)

			// ── Phase 3 (C-2): Energy Arc penalty ──
			tE := preEnergy[t.Filepath]
			energyPenalty := math.Abs(tE - targetEnergy)
			// Base score for energy is max 20
			score += math.Max(0, 20-energyPenalty*20)

			// ── Phase 3 (C-3): BPM Gradient Bonus ──
			if len(sorted) >= 2 {
				prev2BPM := sorted[len(sorted)-2].BPM
				trend := current.BPM - prev2BPM
				if trend > 0 && t.BPM > current.BPM {
					score += 5.0 // bonus for maintaining upward momentum
				} else if trend < 0 && t.BPM < current.BPM {
					score += 5.0 // bonus for maintaining downward momentum
				}
			}

			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}
		sorted = append(sorted, remaining[bestIdx])
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}
	return sorted
}

func avgEnergy(t TrackAnalysis) float64 {
	if len(t.Energy) == 0 {
		return 0.5
	}
	sum := 0.0
	for _, e := range t.Energy {
		sum += e
	}
	return sum / float64(len(t.Energy))
}

var noteOrder = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

func noteIndex(key string) int {
	if len(key) < 1 {
		return 0
	}
	root := key
	if len(key) > 2 && key[1] == '#' {
		root = key[:2]
	} else {
		root = key[:1]
		// handle "C#" correctly: check if second char is '#'
		if len(key) > 1 && key[1] == '#' {
			root = key[:2]
		}
	}
	for i, n := range noteOrder {
		if n == root {
			return i
		}
	}
	return 0
}

func keyDistance(k1, k2 string) int {
	d := noteIndex(k1) - noteIndex(k2)
	if d < 0 {
		d = -d
	}
	if d > 6 {
		d = 12 - d
	}
	return d
}

var camelotMap = map[string]struct {
	num  int
	mode string
}{
	"B": {1, "B"}, "F#": {2, "B"}, "Db": {3, "B"}, "Ab": {4, "B"}, "Eb": {5, "B"}, "Bb": {6, "B"},
	"F": {7, "B"}, "C": {8, "B"}, "G": {9, "B"}, "D": {10, "B"}, "A": {11, "B"}, "E": {12, "B"},
	"G#m": {1, "A"}, "D#m": {2, "A"}, "Bbm": {3, "A"}, "Fm": {4, "A"}, "Cm": {5, "A"}, "Gm": {6, "A"},
	"Dm": {7, "A"}, "Am": {8, "A"}, "Em": {9, "A"}, "Bm": {10, "A"}, "F#m": {11, "A"}, "C#m": {12, "A"},
}

func camelotDistance(k1, k2 string) int {
	c1, ok1 := camelotMap[k1]
	c2, ok2 := camelotMap[k2]
	if !ok1 || !ok2 {
		return keyDistance(k1, k2) * 10 // fallback
	}

	distNum := c1.num - c2.num
	if distNum < 0 {
		distNum = -distNum
	}
	if distNum > 6 {
		distNum = 12 - distNum
	}

	if c1.mode == c2.mode {
		if distNum <= 1 {
			return 0 // perfect harmonic match
		}
		return (distNum - 1) * 10
	} else {
		// different modes (A vs B)
		if distNum == 0 {
			return 10 // relative major/minor
		}
		return distNum * 10
	}
}

func selectTransitionType(ta, tb TrackAnalysis, typeW map[string]float64) string {
	energyDiff := avgEnergy(tb) - avgEnergy(ta)
	keyDist := camelotDistance(ta.Key, tb.Key)
	bpmDiff := math.Abs(ta.BPM - tb.BPM)

	choices := make(map[string]float64)
	if val, ok := typeW["crossfade"]; ok {
		choices["crossfade"] = val
	} else {
		choices["crossfade"] = 0.5
	}

	if keyDist <= 10 && bpmDiff < 5.0 {
		if val, ok := typeW["mashup"]; ok {
			choices["mashup"] = val
		}
		if val, ok := typeW["bass_swap"]; ok {
			choices["bass_swap"] = val
		}
	} else if energyDiff > 0.2 {
		if val, ok := typeW["bass_swap"]; ok {
			choices["bass_swap"] = val
		}
	} else if energyDiff < -0.2 {
		if val, ok := typeW["filter_fade"]; ok {
			choices["filter_fade"] = val
		}
	} else if bpmDiff > 10.0 {
		if val, ok := typeW["cut"]; ok {
			choices["cut"] = val
		}
	}

	bestType := "crossfade"
	bestScore := math.Inf(-1)
	for t, w := range choices {
		score := w * (0.5 + rand.Float64()) // random variance
		if score > bestScore {
			bestScore = score
			bestType = t
		}
	}
	return bestType
}

func generateCandidates(ta, tb TrackAnalysis, typeW map[string]float64, barW map[int]float64, count int) []TransitionSpec {
	bars := weightedIntKeys(barW)

	durA := ta.Duration
	durB := tb.Duration
	bpmA := ta.BPM
	bpmB := tb.BPM
	targetBPM := (bpmA + bpmB) / 2

	// ed031c0b mixing behavior: ignore targetBPM stretching, keep native speed
	speedA := 1.0
	speedB := 1.0

	segsA := ta.Segments
	segsB := tb.Segments
	if len(segsA) == 0 {
		segsA = []Segment{{Time: durA - 30, Label: "Outro", Energy: 0.5}}
	}
	if len(segsB) == 0 {
		segsB = []Segment{{Time: 0, Label: "Intro", Energy: 0.5}}
	}

	beatTimesA := ta.BeatTimes
	beatTimesB := tb.BeatTimes

	var cands []TransitionSpec
	for i := 0; i < count; i++ {
		// ── Phase 1 (D-1): Auto transition type selection ──
		tType := selectTransitionType(ta, tb, typeW)
		pickedBars := bars[rand.Intn(len(bars))]

		var exitSeg Segment
		var entrySeg Segment

		// ── Phase 3 (E-2): Utilize highlights if available ──
		if len(ta.Highlights) > 0 && rand.Float64() > 0.3 {
			bestH := ta.Highlights[0]
			for _, h := range ta.Highlights {
				if h.Score > bestH.Score {
					bestH = h
				}
			}
			exitSeg = Segment{Time: bestH.EndTime, Label: "Highlight"}
		} else {
			exitSeg = pickSegment(segsA, []string{"Chorus", "Verse", "Bridge", "Outro"})
		}

		if len(tb.Highlights) > 0 && rand.Float64() > 0.3 {
			bestH := tb.Highlights[0]
			for _, h := range tb.Highlights {
				if h.Score > bestH.Score {
					bestH = h
				}
			}
			entrySeg = Segment{Time: bestH.StartTime, Label: "Highlight"}
		} else {
			entrySeg = pickSegment(segsB, []string{"Intro", "Verse", "Chorus", "Bridge"})
		}

		// Avoid boring Outro->Intro
		for exitSeg.Label == "Outro" && entrySeg.Label == "Intro" && rand.Float64() > 0.05 {
			exitSeg = pickSegment(segsA, []string{"Chorus", "Verse", "Bridge", "Outro"})
			entrySeg = pickSegment(segsB, []string{"Intro", "Verse", "Chorus", "Bridge"})
		}

		aOut := snapToPhrase(exitSeg.Time+20, ta.Phrases, beatTimesA, 16)
		bIn := snapToPhrase(entrySeg.Time, tb.Phrases, beatTimesB, 16)

		beatDur := 60.0 / targetBPM
		dur := float64(pickedBars) * 4 * beatDur

		cands = append(cands, TransitionSpec{
			Type:     tType,
			Name:     tType + " | " + exitSeg.Label + "->" + entrySeg.Label,
			Duration: dur,
			AOutTime: clampF(aOut, 0, durA),
			BInTime:  clampF(bIn, 0, durB),
			SpeedA:   speedA,
			SpeedB:   speedB,
		})
	}
	return cands
}

func selectBest(cands []TransitionSpec, typeW map[string]float64, barW map[int]float64, minExit float64) *TransitionSpec {
	if len(cands) == 0 {
		return nil
	}
	type scored struct {
		s float64
		c TransitionSpec
	}
	var list []scored
	for _, c := range cands {
		w := typeW[c.Type]
		if w == 0 {
			w = 1.0
		}
		penalty := 0.0
		if c.AOutTime < minExit+4 {
			penalty = -500
		}
		score := w + penalty + rand.Float64()*0.01
		list = append(list, scored{score, c})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].s > list[j].s })
	best := list[0].c
	return &best
}

func pickSegment(segs []Segment, labels []string) Segment {
	var pool []Segment
	for _, s := range segs {
		// ── Phase 2 (B-3): Avoid heavy vocals during entry/exit ──
		if s.VocalEnergy > 0.6 {
			continue // heavy vocal, might clash
		}
		for _, l := range labels {
			if s.Label == l {
				pool = append(pool, s)
				break
			}
		}
	}
	// Fallback if all were rejected
	if len(pool) == 0 {
		for _, s := range segs {
			for _, l := range labels {
				if s.Label == l {
					pool = append(pool, s)
					break
				}
			}
		}
	}

	if len(pool) == 0 && len(segs) > 0 {
		return segs[rand.Intn(len(segs))]
	}
	if len(pool) == 0 {
		return Segment{Time: 0, Label: "Verse", Energy: 0.5}
	}
	return pool[rand.Intn(len(pool))]
}

func snapToPhrase(timeSec float64, phrases []float64, beats []float64, grid int) float64 {
	if len(phrases) > 0 {
		best, bestD := phrases[0], math.Abs(phrases[0]-timeSec)
		for _, p := range phrases[1:] {
			if d := math.Abs(p - timeSec); d < bestD {
				bestD = d
				best = p
			}
		}
		// Snapping is acceptable if the closest phrase boundary is within ~15 seconds.
		if bestD < 15.0 {
			return best
		}
	}
	return snapGrid(timeSec, beats, grid)
}

func snapGrid(timeSec float64, beats []float64, grid int) float64 {
	if len(beats) == 0 {
		return timeSec
	}
	best, bestD := 0, math.Abs(beats[0]-timeSec)
	for i, b := range beats {
		if d := math.Abs(b - timeSec); d < bestD {
			bestD = d
			best = i
		}
	}
	snapped := int(math.Round(float64(best)/float64(grid))) * grid
	if snapped >= len(beats) {
		snapped = (len(beats) - 1) / grid * grid
	}
	if snapped < 0 {
		snapped = 0
	}
	if snapped >= len(beats) {
		snapped = len(beats) - 1
	}
	return beats[snapped]
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func weightedKeys(m map[string]float64) []string {
	var keys []string
	for k, w := range m {
		n := int(math.Round(w * 10))
		if n < 1 {
			n = 1
		}
		for i := 0; i < n; i++ {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return []string{"crossfade"}
	}
	return keys
}

func weightedIntKeys(m map[int]float64) []int {
	var keys []int
	for k, w := range m {
		n := int(math.Round(w * 10))
		if n < 1 {
			n = 1
		}
		for i := 0; i < n; i++ {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return []int{8}
	}
	return keys
}
