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

func sortPlaylist(tracks []TrackAnalysis) []TrackAnalysis {
	if len(tracks) == 0 {
		return tracks
	}
	sorted := []TrackAnalysis{tracks[0]}
	remaining := make([]TrackAnalysis, len(tracks)-1)
	copy(remaining, tracks[1:])

	for len(remaining) > 0 {
		current := sorted[len(sorted)-1]
		bestIdx := 0
		bestScore := math.Inf(-1)
		for i, t := range remaining {
			score := 0.0
			keyDist := keyDistance(current.Key, t.Key)
			score += math.Max(0, 60-float64(keyDist)*10)
			bpmDiff := math.Abs(t.BPM - current.BPM)
			score += math.Max(0, 20-bpmDiff)
			score += avgEnergy(t) * 20
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
		// ed031c0b mixing behavior: force native crossfade only
		tType := "crossfade"
		pickedBars := bars[rand.Intn(len(bars))]

		exitSeg := pickSegment(segsA, []string{"Chorus", "Verse", "Bridge", "Outro"})
		entrySeg := pickSegment(segsB, []string{"Intro", "Verse", "Chorus", "Bridge"})

		// Avoid boring Outro->Intro
		for exitSeg.Label == "Outro" && entrySeg.Label == "Intro" && rand.Float64() > 0.05 {
			exitSeg = pickSegment(segsA, []string{"Chorus", "Verse", "Bridge", "Outro"})
			entrySeg = pickSegment(segsB, []string{"Intro", "Verse", "Chorus", "Bridge"})
		}

		aOut := snapGrid(exitSeg.Time+20, beatTimesA, 16)
		bIn := snapGrid(entrySeg.Time, beatTimesB, 16)

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
		for _, l := range labels {
			if s.Label == l {
				pool = append(pool, s)
				break
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
