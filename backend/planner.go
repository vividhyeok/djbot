package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func ComputePlayBounds(playlist []TrackWithAnalysis, transitions []TransitionSpec) []TrackEntry {
	out := make([]TrackEntry, len(playlist))
	for i, t := range playlist {
		out[i] = TrackEntry{Filepath: t.Analysis.Filepath, Filename: t.Filename, Duration: t.Analysis.Duration, BPM: t.Analysis.BPM, LoudnessDB: t.Analysis.LoudnessDB, PlayEnd: t.Analysis.Duration}
		if i > 0 && i-1 < len(transitions) {
			out[i].PlayStart = clampF(transitions[i-1].BInTime, 0, math.Max(0, t.Analysis.Duration-15))
		}
	}
	return out
}

func defaultSettings(s MixSettings) MixSettings {
	if s.TransitionBars != 8 && s.TransitionBars != 16 && s.TransitionBars != 32 {
		s.TransitionBars = 16
	}
	if s.Style == "" {
		s.Style = "auto"
	}
	if s.SearchDepth == "" {
		s.SearchDepth = "balanced"
	}
	if s.TransitionLength == 0 {
		s.TransitionLength = .5
	}
	if s.TransitionCharacter == 0 {
		s.TransitionCharacter = .5
	}
	if s.HarmonicPriority == 0 {
		s.HarmonicPriority = .5
	}
	if s.PhrasePriority == 0 {
		s.PhrasePriority = .7
	}
	if s.VocalOverlap == 0 {
		s.VocalOverlap = .7
	}
	if s.TempoFlexibility == 0 {
		s.TempoFlexibility = .5
	}
	if s.EnergyMovement == 0 {
		s.EnergyMovement = .5
	}
	return s
}
func searchBudget(depth string) (int, int) {
	switch strings.ToLower(depth) {
	case "quick":
		return 12, 4
	case "deep":
		return 96, 10
	case "max":
		return 256, 16
	default:
		return 40, 7
	}
}

// GenerateMixPlan remains source-compatible with the original API.
func GenerateMixPlan(tracks []TrackAnalysis, typeW map[string]float64, barW map[int]float64, scenarios int) MixPlan {
	depth := "balanced"
	if scenarios <= 8 {
		depth = "quick"
	} else if scenarios >= 100 {
		depth = "deep"
	}
	return GenerateMixPlanV2(tracks, typeW, barW, MixSettings{Style: "auto", SearchDepth: depth})
}

// GenerateMixPlanV2 performs a cheap beam search for set order, then exhaustive
// metadata scoring per adjacent pair. No audio is rendered during search.
func GenerateMixPlanV2(tracks []TrackAnalysis, typeW map[string]float64, barW map[int]float64, settings MixSettings) MixPlan {
	settings = defaultSettings(settings)
	if len(tracks) < 2 {
		return MixPlan{Settings: settings}
	}
	perPair, beam := searchBudget(settings.SearchDepth)
	ordered := tracks
	if !settings.PreserveOrder {
		ordered = beamOrder(tracks, settings, beam)
	}
	all := make([][]TransitionSpec, 0, len(ordered)-1)
	alts := make([][]TransitionSpec, 0, len(ordered)-1)
	sels := make([]TransitionSpec, 0, len(ordered)-1)
	evaluated := 0
	for i := 0; i < len(ordered)-1; i++ {
		cs := generateTransitionCandidatesWithSettings(ordered[i], ordered[i+1], typeW, barW, perPair, settings)
		evaluated += len(cs)
		sort.SliceStable(cs, func(a, b int) bool { return cs[a].Score.Total > cs[b].Score.Total })
		all = append(all, cs)
		if len(cs) > 0 {
			choice := 0
			if settings.Variation > 0 {
				choice = settings.Variation % min(3, len(cs))
			}
			sels = append(sels, cs[choice])
			n := 3
			if len(cs) < n {
				n = len(cs)
			}
			alts = append(alts, append([]TransitionSpec(nil), cs[:n]...))
		}
	}
	setScore := aggregateSetScore(ordered, sels, settings)
	return MixPlan{SortedTracks: ordered, Candidates: all, Selections: sels, Alternatives: alts, Score: setScore, Evaluated: evaluated, Settings: settings}
}

type orderState struct {
	seq   []int
	used  map[int]bool
	score float64
}

func beamOrder(tracks []TrackAnalysis, s MixSettings, width int) []TrackAnalysis {
	states := []orderState{}
	for i := range tracks {
		states = append(states, orderState{[]int{i}, map[int]bool{i: true}, 0})
	}
	for len(states[0].seq) < len(tracks) {
		var next []orderState
		for _, st := range states {
			last := st.seq[len(st.seq)-1]
			for j := range tracks {
				if st.used[j] {
					continue
				}
				u := make(map[int]bool, len(st.used)+1)
				for k, v := range st.used {
					u[k] = v
				}
				u[j] = true
				pos := float64(len(st.seq)) / float64(len(tracks)-1)
				score := st.score + pairOrderScore(tracks[last], tracks[j], targetEnergy(s.Style, pos))
				next = append(next, orderState{append(append([]int(nil), st.seq...), j), u, score})
			}
		}
		sort.Slice(next, func(i, j int) bool { return next[i].score > next[j].score })
		if len(next) > width {
			next = next[:width]
		}
		states = next
	}
	out := make([]TrackAnalysis, len(tracks))
	choice := 0
	if s.Variation > 0 {
		choice = s.Variation % min(3, len(states))
	}
	for i, x := range states[choice].seq {
		out[i] = tracks[x]
	}
	return out
}
func targetEnergy(style string, p float64) float64 {
	switch strings.ToLower(style) {
	case "hype":
		return .55 + .4*p
	case "chill":
		return .45 + .1*math.Sin(p*math.Pi)
	case "club":
		return .55 + .35*math.Sin(p*math.Pi*.85)
	case "smooth":
		return .5 + .2*p
	case "journey":
		return .4 + .5*math.Sin(p*math.Pi)
	default:
		return .45 + .45*math.Sin(p*math.Pi*.9)
	}
}
func pairOrderScore(a, b TrackAnalysis, target float64) float64 {
	return harmonyScore(a.Key, b.Key)*.35 + bpmScore(a.BPM, b.BPM)*.2 + (100-math.Abs(avgEnergy(b)-target)*100)*.35 + energyContinuity(a, b)*.1
}

func generateTransitionCandidates(a, b TrackAnalysis, typeW map[string]float64, barW map[int]float64, limit int) []TransitionSpec {
	return generateTransitionCandidatesWithSettings(a, b, typeW, barW, limit, defaultSettings(MixSettings{}))
}

func generateTransitionCandidatesWithSettings(a, b TrackAnalysis, typeW map[string]float64, barW map[int]float64, limit int, settings MixSettings) []TransitionSpec {
	if a.Duration <= 0 || b.Duration <= 0 || !finite(a.Duration) || !finite(b.Duration) {
		return nil
	}
	target, spA, spB, reliable := matchedTempo(a, b, settings.TempoFlexibility)
	if !reliable {
		return []TransitionSpec{pacedSafeTransition(a, b, settings)}
	}
	types := []string{"crossfade", "bass_swap", "filter_fade", "long_blend"}
	bars := []int{4, 8, 16, 32}
	if settings.TransitionLength < .34 {
		bars = []int{4, 8, 16}
	}
	if settings.TransitionLength > .66 {
		bars = []int{8, 16, 32}
	}
	if len(barW) > 0 {
		bars = bars[:0]
		for k := range barW {
			bars = append(bars, k)
		}
		sort.Ints(bars)
	}
	exits := pacedExits(a, settings)
	entries := anchors(b, true)
	bars = []int{defaultSettings(settings).TransitionBars}
	// Keep entry cues early enough for independent incoming and outgoing fades.
	entries = nil
	for _, cue := range b.Downbeats {
		if cue >= 0 && cue <= math.Min(8, b.Duration*.05) && len(entries) < 5 {
			entries = append(entries, cue)
		}
	}
	if len(entries) == 0 {
		entries = []float64{0}
	}
	var out []TransitionSpec
	for _, x := range exits {
		for _, y := range entries {
			for _, nbar := range bars {
				for _, typ := range types {
					dur := float64(nbar) * 4 * 60 / target
					requested := dur
					dur = math.Min(dur, transitionBudget(a, b, settings)/math.Max(spA, spB))
					if nbar < 1 {
						continue
					}
					if typ == "cut" {
						dur = math.Min(dur, 4*60/target)
					}
					x = math.Min(x, a.Duration-dur*spA)
					if x < dur*spA+math.Min(8, a.Duration*.05) || y+dur*spB > b.Duration {
						continue
					}
					t := TransitionSpec{ID: fmt.Sprintf("%s-%d-%d-%d", typ, int(x*10), int(y*10), nbar), Type: typ, Name: transitionName(typ), Duration: dur, Bars: nbar, AOutTime: x, BInTime: y, TargetBPM: target, SpeedA: spA, SpeedB: spB, Curve: "equal_power", CurveAmount: .5, GainA: 1, GainB: 1, FilterCutoff: 400, FilterResonance: .7, EchoFeedback: .35, EchoMix: .25, PhraseAligned: nearAny(x, a.Phrases, 0.08), DownbeatAligned: nearAny(x, a.Downbeats, 0.08) && nearAny(y, b.Downbeats, 0.08)}
					t.Automation = defaultAutomation(typ, dur, spA, spB)
					t.Explanation = "선택한 마디 수로 두 곡을 겹칩니다."
					if dur < requested-.01 {
						t.Bars = 0
						t.Explanation = "곡 길이와 앞뒤 전환의 충돌을 피하려고 전환을 줄였습니다. 실제 초 단위를 확인하세요."
					}
					t.Score = scoreTransitionWithSettings(a, b, t, settings)
					if w := typeW[typ]; w > 0 {
						t.Score.Total = clampF(t.Score.Total+(w-1)*2, 0, 100)
					}
					out = append(out, t)
				}
			}
		}
	}
	if len(out) == 0 {
		return []TransitionSpec{pacedSafeTransition(a, b, settings)}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score.Total > out[j].Score.Total })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
func anchors(t TrackAnalysis, entry bool) []float64 {
	src := t.Phrases
	if len(src) == 0 {
		src = t.Downbeats
	}
	if len(src) == 0 {
		src = t.BeatTimes
	}
	var out []float64
	for _, x := range src {
		p := x / t.Duration
		if entry && p <= .18 {
			out = append(out, x)
		} else if !entry && p >= .7 && p <= .94 {
			out = append(out, x)
		}
	}
	if len(out) == 0 {
		if entry {
			return []float64{0}
		}
		return []float64{math.Max(t.Duration*.75, t.Duration-30)}
	}
	if len(out) > 8 {
		step := len(out) / 8
		z := []float64{}
		for i := 0; i < len(out); i += step {
			z = append(z, out[i])
		}
		out = z
	}
	return out
}
func transitionName(t string) string {
	return map[string]string{"crossfade": "Smooth Crossfade", "bass_swap": "Bass Swap", "filter_fade": "Filter Fade", "echo_out": "Echo Out", "cut": "Beat Cut", "drop": "Drop Transition", "long_blend": "Long Blend", "mashup": "Mashup Blend"}[t]
}
func weightedTempo(a, b float64) float64 {
	if a <= 0 || b <= 0 || math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
		return 120
	}
	for b > a*1.45 {
		b /= 2
	}
	for a > b*1.45 {
		a /= 2
	}
	return (a + b) / 2
}
func nearAny(x float64, v []float64, tol float64) bool {
	for _, y := range v {
		if math.Abs(x-y) <= tol {
			return true
		}
	}
	return false
}

func defaultAutomation(typ string, dur, sa, sb float64) Automation {
	a := Automation{VolumeA: []AutomationPoint{{0, 1}, {dur, 0}}, VolumeB: []AutomationPoint{{0, 0}, {dur, 1}}, Tempo: []AutomationPoint{{0, sa}, {dur, sb}}}
	switch typ {
	case "bass_swap":
		a.LowA = []AutomationPoint{{0, 1}, {dur * .5, 1}, {dur * .52, 0}}
		a.LowB = []AutomationPoint{{0, 0}, {dur * .5, 0}, {dur * .52, 1}}
	case "filter_fade":
		a.FilterA = []AutomationPoint{{0, 1}, {dur, 0}}
		a.FilterB = []AutomationPoint{{0, .2}, {dur, 1}}
	case "cut":
		a.VolumeA = []AutomationPoint{{0, 1}, {dur, 1}, {dur, 0}}
		a.VolumeB = []AutomationPoint{{0, 0}, {dur, 1}}
	}
	return a
}

func scoreTransition(a, b TrackAnalysis, t TransitionSpec) ScoreBreakdown {
	return scoreTransitionWithSettings(a, b, t, defaultSettings(MixSettings{}))
}

func confidenceOr(v, fallback float64) float64 {
	if v <= 0 {
		return fallback
	}
	return clampF(v, 0, 1)
}

func scoreTransitionWithSettings(a, b TrackAnalysis, t TransitionSpec, settings MixSettings) ScoreBreakdown {
	s := ScoreBreakdown{}
	s.Beat = alignmentScore(t.AOutTime, a.BeatTimes)
	s.Downbeat = (alignmentScore(t.AOutTime, a.Downbeats) + alignmentScore(t.BInTime, b.Downbeats)) / 2
	s.Phrase = (alignmentScore(t.AOutTime, a.Phrases) + alignmentScore(t.BInTime, b.Phrases)) / 2
	s.Harmony = harmonyScore(a.Key, b.Key)
	s.BPM = bpmScore(a.BPM, b.BPM)
	ratio := math.Max(t.SpeedA, 1/t.SpeedA) * math.Max(t.SpeedB, 1/t.SpeedB)
	s.Tempo = clampF(110-(ratio-1)*180, 0, 100)
	s.Energy = energyAtScore(a, t.AOutTime, b, t.BInTime)
	s.Loudness = clampF(100-math.Abs(a.LoudnessDB-b.LoudnessDB)*7, 0, 100)
	s.Spectral = spectralCompatibility(a, t.AOutTime, b, t.BInTime)
	s.Vocal = vocalCompatibility(a, t.AOutTime, b, t.BInTime)
	s.Smoothness = (s.Beat + s.Downbeat + s.Phrase + s.Tempo + s.Loudness) / 5
	downConf := math.Min(confidenceOr(a.DownbeatConfidence, .55), confidenceOr(b.DownbeatConfidence, .55))
	phraseConf := math.Min(confidenceOr(a.PhraseConfidence, .5), confidenceOr(b.PhraseConfidence, .5))
	vocalConf := math.Min(confidenceOr(a.VocalConfidence, .35), confidenceOr(b.VocalConfidence, .35))
	harmonyW := .08 + .08*settings.HarmonicPriority
	phraseW := (.06 + .10*settings.PhrasePriority) * phraseConf
	tempoW := .05 + .08*(1-settings.TempoFlexibility)
	vocalW := (.03 + .08*settings.VocalOverlap) * vocalConf
	downW := .10 * downConf
	baseW := .13 + downW + phraseW + harmonyW + .07 + tempoW + .11 + .07 + .07 + vocalW
	s.Total = (s.Beat*.13 + s.Downbeat*downW + s.Phrase*phraseW + s.Harmony*harmonyW + s.BPM*.07 + s.Tempo*tempoW + s.Energy*.11 + s.Loudness*.07 + s.Spectral*.07 + s.Vocal*vocalW) / baseW
	return roundScore(s)
}
func alignmentScore(x float64, v []float64) float64 {
	if len(v) == 0 {
		return 50
	}
	i := nearestIndex(v, x)
	d := math.Abs(v[i] - x)
	return clampF(100-d*120, 0, 100)
}
func normalizeKey(k string) string {
	k = strings.ReplaceAll(k, " Major", "")
	k = strings.ReplaceAll(k, " Minor", "m")
	return k
}
func harmonyScore(a, b string) float64 {
	d := camelotDistance(normalizeKey(a), normalizeKey(b))
	return clampF(100-float64(d)*1.7, 0, 100)
}
func bpmScore(a, b float64) float64 {
	if a <= 0 || b <= 0 {
		return 50
	}
	for b > a*1.45 {
		b /= 2
	}
	for a > b*1.45 {
		a /= 2
	}
	return clampF(100-math.Abs(a-b)*5, 0, 100)
}
func energyAt(t TrackAnalysis, x float64) float64 {
	if len(t.Energy) == 0 || len(t.BeatTimes) == 0 {
		return .5
	}
	i := nearestIndex(t.BeatTimes, x)
	if i >= len(t.Energy) {
		i = len(t.Energy) - 1
	}
	return t.Energy[i]
}
func energyAtScore(a TrackAnalysis, x float64, b TrackAnalysis, y float64) float64 {
	return clampF(100-math.Abs(energyAt(a, x)-energyAt(b, y))*100, 0, 100)
}
func energyContinuity(a, b TrackAnalysis) float64 {
	return clampF(100-math.Abs(avgEnergy(a)-avgEnergy(b))*100, 0, 100)
}
func avgEnergy(t TrackAnalysis) float64 {
	if len(t.Energy) == 0 {
		return .5
	}
	var s float64
	for _, x := range t.Energy {
		s += x
	}
	return s / float64(len(t.Energy))
}
func nearestSpectral(t TrackAnalysis, x float64) SpectralFeature {
	if len(t.Spectral) == 0 {
		return SpectralFeature{BassRatio: .33, MidRatio: .34, HighRatio: .33}
	}
	best := 0
	for i := range t.Spectral {
		if math.Abs(t.Spectral[i].Time-x) < math.Abs(t.Spectral[best].Time-x) {
			best = i
		}
	}
	return t.Spectral[best]
}
func spectralCompatibility(a TrackAnalysis, x float64, b TrackAnalysis, y float64) float64 {
	p, q := nearestSpectral(a, x), nearestSpectral(b, y)
	collision := math.Min(p.BassRatio, q.BassRatio)*.5 + math.Min(p.MidRatio, q.MidRatio)*.35 + math.Min(p.HighRatio, q.HighRatio)*.15
	return clampF(105-collision*85, 0, 100)
}
func segmentAt(t TrackAnalysis, x float64) Segment {
	for _, s := range t.Segments {
		if x >= s.Start && x < s.End {
			return s
		}
	}
	return Segment{VocalEnergy: .5}
}
func vocalCompatibility(a TrackAnalysis, x float64, b TrackAnalysis, y float64) float64 {
	v := segmentAt(a, x).VocalEnergy * segmentAt(b, y).VocalEnergy
	return clampF(100-v*80, 0, 100)
}
func roundScore(s ScoreBreakdown) ScoreBreakdown {
	p := func(x float64) float64 { return math.Round(x*10) / 10 }
	s.Total = p(s.Total)
	s.Beat = p(s.Beat)
	s.Downbeat = p(s.Downbeat)
	s.Phrase = p(s.Phrase)
	s.Harmony = p(s.Harmony)
	s.BPM = p(s.BPM)
	s.Tempo = p(s.Tempo)
	s.Energy = p(s.Energy)
	s.Loudness = p(s.Loudness)
	s.Spectral = p(s.Spectral)
	s.Vocal = p(s.Vocal)
	s.Smoothness = p(s.Smoothness)
	return s
}
func aggregateSetScore(ts []TrackAnalysis, tr []TransitionSpec, s MixSettings) ScoreBreakdown {
	var out ScoreBreakdown
	if len(tr) == 0 {
		return out
	}
	types := map[string]bool{}
	var arc float64
	for i, x := range tr {
		out.Beat += x.Score.Beat
		out.Downbeat += x.Score.Downbeat
		out.Phrase += x.Score.Phrase
		out.Harmony += x.Score.Harmony
		out.Energy += x.Score.Energy
		out.Spectral += x.Score.Spectral
		out.Total += x.Score.Total
		types[x.Type] = true
		if i < len(ts) {
			arc += 100 - math.Abs(avgEnergy(ts[i])-targetEnergy(s.Style, float64(i)/float64(len(ts)-1)))*100
		}
	}
	n := float64(len(tr))
	out.Beat /= n
	out.Downbeat /= n
	out.Phrase /= n
	out.Harmony /= n
	out.Energy /= n
	out.Spectral /= n
	out.Total /= n
	out.Diversity = clampF(float64(len(types))/math.Min(n, 4)*100, 0, 100)
	out.SetArc = clampF(arc/n, 0, 100)
	out.Total = out.Total*.8 + out.Diversity*.08 + out.SetArc*.12
	return roundScore(out)
}

var noteOrder = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

func noteIndex(k string) int {
	root := strings.TrimSuffix(normalizeKey(k), "m")
	flats := map[string]string{"Db": "C#", "Eb": "D#", "Gb": "F#", "Ab": "G#", "Bb": "A#"}
	if x := flats[root]; x != "" {
		root = x
	}
	for i, n := range noteOrder {
		if n == root {
			return i
		}
	}
	return 0
}
func keyDistance(a, b string) int {
	d := absInt(noteIndex(a) - noteIndex(b))
	if d > 6 {
		d = 12 - d
	}
	return d
}

var camelotMap = map[string]struct {
	num  int
	mode string
}{"B": {1, "B"}, "F#": {2, "B"}, "Db": {3, "B"}, "Ab": {4, "B"}, "Eb": {5, "B"}, "Bb": {6, "B"}, "F": {7, "B"}, "C": {8, "B"}, "G": {9, "B"}, "D": {10, "B"}, "A": {11, "B"}, "E": {12, "B"}, "G#m": {1, "A"}, "D#m": {2, "A"}, "Bbm": {3, "A"}, "Fm": {4, "A"}, "Cm": {5, "A"}, "Gm": {6, "A"}, "Dm": {7, "A"}, "Am": {8, "A"}, "Em": {9, "A"}, "Bm": {10, "A"}, "F#m": {11, "A"}, "C#m": {12, "A"}}

func camelotDistance(a, b string) int {
	x, ok := camelotMap[a]
	y, ok2 := camelotMap[b]
	if !ok || !ok2 {
		return keyDistance(a, b) * 10
	}
	d := absInt(x.num - y.num)
	if d > 6 {
		d = 12 - d
	}
	if x.mode == y.mode {
		if d <= 1 {
			return 0
		}
		return (d - 1) * 10
	}
	if d == 0 {
		return 10
	}
	return d * 10
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
