package main

import (
	"math"
	"sort"
)

func tempoConfidence(onset []float64, sr, hop int, bpm float64) float64 {
	if len(onset) < 32 || bpm <= 0 {
		return 0
	}
	lag := int(math.Round(60 * float64(sr) / (bpm * float64(hop))))
	var dot, aa, bb float64
	for i := 0; i+lag < len(onset); i++ {
		a, b := onset[i], onset[i+lag]
		dot += a * b
		aa += a * a
		bb += b * b
	}
	if aa*bb == 0 {
		return 0
	}
	return clampF(dot/math.Sqrt(aa*bb), 0, 1)
}

// trackBeats follows local onset maxima around the nominal tempo instead of
// projecting one fixed interval through the entire song.
func trackBeats(onset []float64, sr int, duration, bpm float64, hop int) []float64 {
	base := estimateBeatTimes(onset, sr, duration, bpm, hop)
	if len(base) == 0 {
		return base
	}
	period := 60 / bpm
	radius := int(0.16 * period * float64(sr) / float64(hop))
	if radius < 1 {
		radius = 1
	}
	beats := make([]float64, 0, len(base))
	last := -1.0
	for _, guess := range base {
		center := int(guess * float64(sr) / float64(hop))
		lo, hi := center-radius, center+radius
		if lo < 0 {
			lo = 0
		}
		if hi >= len(onset) {
			hi = len(onset) - 1
		}
		best := center
		bestV := -1.0
		for j := lo; j <= hi; j++ {
			if onset[j] > bestV {
				bestV = onset[j]
				best = j
			}
		}
		t := float64(best*hop) / float64(sr)
		if last < 0 || t-last > period*.55 {
			beats = append(beats, math.Round(t*1000)/1000)
			last = t
		}
	}
	return beats
}

func detectDownbeats(beats, energy []float64) []float64 {
	if len(beats) < 4 {
		return append([]float64(nil), beats...)
	}
	bestPhase, best := -1, -1.0
	for phase := 0; phase < 4; phase++ {
		var s float64
		var n int
		for i := phase; i < len(energy); i += 4 {
			s += energy[i]
			n++
		}
		if n > 0 && s/float64(n) > best {
			best = s / float64(n)
			bestPhase = phase
		}
	}
	var out []float64
	for i := bestPhase; i < len(beats); i += 4 {
		out = append(out, beats[i])
	}
	return out
}

func localMean(v []float64, from, to int) float64 {
	if from < 0 {
		from = 0
	}
	if to > len(v) {
		to = len(v)
	}
	if from >= to {
		return 0
	}
	var s float64
	for _, x := range v[from:to] {
		s += x
	}
	return s / float64(to-from)
}

// Phrase boundaries are selected from bar-level energy/onset novelty and then
// regularized to musically useful 4/8-bar spacing.
func detectPhraseBoundaries(beats, downbeats, energy []float64, onset []float64, sr, hop int, duration float64) []float64 {
	if len(downbeats) < 5 {
		return []float64{0}
	}
	type cand struct {
		i     int
		score float64
	}
	var cs []cand
	for i := 4; i < len(downbeats)-2; i++ {
		bi := nearestIndex(beats, downbeats[i])
		before := localMean(energy, bi-16, bi)
		after := localMean(energy, bi, bi+16)
		oi := int(downbeats[i] * float64(sr) / float64(hop))
		onsetPeak := localMean(onset, oi-2, oi+3)
		score := math.Abs(after-before) + onsetPeak/(onsetPeak+20)
		if i%8 == 0 {
			score += .12
		}
		cs = append(cs, cand{i, score})
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].score > cs[j].score })
	chosen := []int{0}
	minBars := 4
	for _, c := range cs {
		ok := true
		for _, x := range chosen {
			if absInt(c.i-x) < minBars {
				ok = false
				break
			}
		}
		if ok && c.score > .18 {
			chosen = append(chosen, c.i)
		}
	}
	sort.Ints(chosen)
	out := make([]float64, 0, len(chosen))
	for _, i := range chosen {
		out = append(out, downbeats[i])
	}
	if duration-out[len(out)-1] > 8 {
		out = append(out, downbeats[len(downbeats)-1])
	}
	return out
}

func phraseConfidence(phrases []float64, duration, tempoConfidence float64) float64 {
	if len(phrases) < 2 || duration <= 0 {
		return .15
	}
	density := float64(len(phrases)-1) / math.Max(1, duration/30)
	regularity := clampF(1-math.Abs(1-density)*.45, .25, 1)
	return clampF(.2+.45*tempoConfidence+.25*regularity, .15, .8)
}
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func nearestIndex(v []float64, x float64) int {
	if len(v) == 0 {
		return 0
	}
	i := sort.SearchFloat64s(v, x)
	if i >= len(v) {
		return len(v) - 1
	}
	if i > 0 && x-v[i-1] < v[i]-x {
		return i - 1
	}
	return i
}

func extractWaveform(samples []float32, points int) []float64 {
	if points < 1 {
		points = 800
	}
	out := make([]float64, points)
	step := len(samples) / points
	if step < 1 {
		step = 1
	}
	for i := range out {
		a := i * step
		b := a + step
		if b > len(samples) {
			b = len(samples)
		}
		var peak float64
		for _, s := range samples[a:b] {
			v := math.Abs(float64(s))
			if v > peak {
				peak = v
			}
		}
		out[i] = math.Round(peak*1000) / 1000
	}
	return out
}

func spectralSummary(samples []float32, sr int, beats []float64) []SpectralFeature {
	const size = 2048
	win := hannWindow(size)
	stride := 16
	if len(beats) < 16 {
		stride = 1
	}
	var out []SpectralFeature
	for i := 0; i < len(beats); i += stride {
		start := int(beats[i] * float64(sr))
		if start+size > len(samples) {
			break
		}
		frame := make([]complex128, size)
		for j := 0; j < size; j++ {
			frame[j] = complex(float64(samples[start+j])*win[j], 0)
		}
		sp := fft(frame)
		var total, weighted, bass, mid, high float64
		for k := 1; k < size/2; k++ {
			m := cmplxAbs(sp[k])
			f := float64(k*sr) / size
			total += m
			weighted += m * f
			if f < 250 {
				bass += m
			} else if f < 4000 {
				mid += m
			} else {
				high += m
			}
		}
		if total < 1e-9 {
			total = 1
		}
		out = append(out, SpectralFeature{Time: beats[i], Centroid: weighted / total, BassRatio: bass / total, MidRatio: mid / total, HighRatio: high / total})
	}
	return out
}
func cmplxAbs(c complex128) float64 { return math.Hypot(real(c), imag(c)) }

func classifySections(phrases []float64, energy []float64, beats []float64, duration float64) []Segment {
	if len(phrases) == 0 {
		return nil
	}
	vals := make([]float64, len(phrases))
	for i, p := range phrases {
		a := nearestIndex(beats, p)
		end := len(energy)
		if i+1 < len(phrases) {
			end = nearestIndex(beats, phrases[i+1])
		}
		vals[i] = localMean(energy, a, end)
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	lo, hi := sorted[len(sorted)/3], sorted[len(sorted)*2/3]
	out := make([]Segment, len(phrases))
	for i, p := range phrases {
		end := duration
		if i+1 < len(phrases) {
			end = phrases[i+1]
		}
		rel := p / duration
		e := vals[i]
		label := "verse"
		switch {
		case rel < .12:
			label = "intro"
		case rel > .86:
			label = "outro"
		case e >= hi && rel > .2:
			label = "drop"
		case e >= hi:
			label = "chorus"
		case e <= lo:
			label = "breakdown"
		}
		vocal := clampF(.35+(e-.5)*.35, 0, 1)
		out[i] = Segment{Start: p, End: end, Time: p, Label: label, Energy: e, VocalEnergy: vocal, Confidence: .65}
	}
	return out
}

func keyWithConfidence(samples []float32, sr int) (string, float64) {
	key := detectKey(samples, sr) // confidence from agreement across thirds
	if len(samples) < sr*6 {
		return key, .25
	}
	keys := map[string]int{}
	for part := 0; part < 3; part++ {
		a := len(samples) * part / 3
		b := len(samples) * (part + 1) / 3
		keys[detectKey(samples[a:b], sr)]++
	}
	return key, clampF(float64(keys[key])/3*.8+.2, 0, 1)
}
