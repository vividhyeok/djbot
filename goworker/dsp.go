package main

import (
	"math"
	"math/cmplx"
)

// --- FFT ---

func nextPow2(n int) int {
	v := 1
	for v < n {
		v <<= 1
	}
	return v
}

func fft(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		out := make([]complex128, n)
		copy(out, x)
		return out
	}
	even := make([]complex128, n/2)
	odd := make([]complex128, n/2)
	for i := 0; i < n/2; i++ {
		even[i] = x[2*i]
		odd[i] = x[2*i+1]
	}
	eR := fft(even)
	oR := fft(odd)
	out := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		t := cmplx.Rect(1, -2*math.Pi*float64(k)/float64(n)) * oR[k]
		out[k] = eR[k] + t
		out[k+n/2] = eR[k] - t
	}
	return out
}

func hannWindow(n int) []float64 {
	w := make([]float64, n)
	for i := range w {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(n-1)))
	}
	return w
}

// --- Onset / BPM ---

func computeOnsetEnvelope(samples []float32, sr, frameSize, hopSize int) []float64 {
	n := len(samples)
	numFrames := (n - frameSize) / hopSize
	if numFrames <= 0 {
		return nil
	}
	fftSize := nextPow2(frameSize)
	window := hannWindow(frameSize)
	onset := make([]float64, numFrames)
	prevMag := make([]float64, fftSize/2+1)

	for i := 0; i < numFrames; i++ {
		start := i * hopSize
		frame := make([]complex128, fftSize)
		for j := 0; j < frameSize && start+j < n; j++ {
			frame[j] = complex(float64(samples[start+j])*window[j], 0)
		}
		spec := fft(frame)
		mag := make([]float64, fftSize/2+1)
		for j := 0; j <= fftSize/2; j++ {
			mag[j] = cmplx.Abs(spec[j])
		}
		flux := 0.0
		for j := range mag {
			if j < len(prevMag) {
				d := mag[j] - prevMag[j]
				if d > 0 {
					flux += d
				}
			}
		}
		onset[i] = flux
		prevMag = mag
	}
	return onset
}

func estimateBPM(samples []float32, sr int) float64 {
	hopSize := 512
	frameSize := 1024
	onset := computeOnsetEnvelope(samples, sr, frameSize, hopSize)
	if len(onset) < 100 {
		return 120.0
	}

	// Autocorrelation in BPM range 60-200
	minLag := sr * 60 / (200 * hopSize)
	maxLag := sr * 60 / (60 * hopSize)
	if maxLag >= len(onset) {
		maxLag = len(onset) - 1
	}

	bestLag := minLag
	bestCorr := -1.0
	for lag := minLag; lag <= maxLag; lag++ {
		corr := 0.0
		count := 0
		for i := 0; i+lag < len(onset); i++ {
			corr += onset[i] * onset[i+lag]
			count++
		}
		if count > 0 {
			corr /= float64(count)
		}
		if corr > bestCorr {
			bestCorr = corr
			bestLag = lag
		}
	}

	beatPeriodSec := float64(bestLag) * float64(hopSize) / float64(sr)
	if beatPeriodSec <= 0 {
		return 120.0
	}
	bpm := 60.0 / beatPeriodSec

	// Normalize to 60-200 range
	for bpm > 200 {
		bpm /= 2
	}
	for bpm < 60 {
		bpm *= 2
	}
	return math.Round(bpm*10) / 10
}

func estimateBeatTimes(duration, bpm float64) []float64 {
	if bpm <= 0 {
		bpm = 120
	}
	beatPeriod := 60.0 / bpm
	var beats []float64
	for t := 0.0; t < duration; t += beatPeriod {
		beats = append(beats, math.Round(t*1000)/1000)
	}
	return beats
}

// --- Energy ---

func computeRMSFrames(samples []float32, frameSize, hopSize int) []float64 {
	n := len(samples)
	numFrames := (n - frameSize) / hopSize
	if numFrames <= 0 {
		return []float64{0.5}
	}
	rms := make([]float64, numFrames)
	for i := 0; i < numFrames; i++ {
		start := i * hopSize
		sum := 0.0
		count := 0
		for j := 0; j < frameSize && start+j < n; j++ {
			v := float64(samples[start+j])
			sum += v * v
			count++
		}
		if count > 0 {
			rms[i] = math.Sqrt(sum / float64(count))
		}
	}
	return rms
}

func computeBeatEnergy(samples []float32, sr int, beatTimes []float64) []float64 {
	frameSize := 2048
	hopSize := 512
	rms := computeRMSFrames(samples, frameSize, hopSize)
	if len(beatTimes) < 2 {
		return []float64{0.5}
	}

	energy := make([]float64, len(beatTimes))
	for i, bt := range beatTimes {
		frameIdx := int(bt * float64(sr) / float64(hopSize))
		var nextFrameIdx int
		if i+1 < len(beatTimes) {
			nextFrameIdx = int(beatTimes[i+1] * float64(sr) / float64(hopSize))
		} else {
			nextFrameIdx = frameIdx + int(float64(sr)/float64(hopSize)*0.5)
		}
		if frameIdx >= len(rms) {
			frameIdx = len(rms) - 1
		}
		if nextFrameIdx > len(rms) {
			nextFrameIdx = len(rms)
		}
		if frameIdx < 0 {
			frameIdx = 0
		}
		sum := 0.0
		count := 0
		for j := frameIdx; j < nextFrameIdx; j++ {
			sum += rms[j]
			count++
		}
		if count > 0 {
			energy[i] = sum / float64(count)
		}
	}

	// Normalize
	maxE := 0.0
	for _, e := range energy {
		if e > maxE {
			maxE = e
		}
	}
	if maxE > 1e-6 {
		for i := range energy {
			energy[i] /= maxE
		}
	}
	return energy
}

func computeLoudnessDB(samples []float32) float64 {
	sum := 0.0
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	avg := sum / float64(len(samples)+1)
	return 20 * math.Log10(math.Sqrt(avg)+1e-6)
}

// --- Key Detection ---

var (
	noteNames  = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	majProfile = []float64{6.35, 2.23, 3.48, 2.33, 4.38, 4.09, 2.52, 5.19, 2.39, 3.66, 2.29, 2.88}
	minProfile = []float64{6.33, 2.68, 3.52, 5.38, 2.60, 3.53, 2.54, 4.75, 3.98, 2.69, 3.34, 3.17}
)

func detectKey(samples []float32, sr int) string {
	frameSize := 4096
	hopSize := 2048
	n := len(samples)
	numFrames := (n - frameSize) / hopSize
	if numFrames <= 0 {
		return "C Major"
	}

	fftSize := nextPow2(frameSize)
	window := hannWindow(frameSize)
	chroma := make([]float64, 12)

	for i := 0; i < numFrames; i++ {
		start := i * hopSize
		frame := make([]complex128, fftSize)
		for j := 0; j < frameSize && start+j < n; j++ {
			frame[j] = complex(float64(samples[start+j])*window[j], 0)
		}
		spec := fft(frame)
		for bin := 1; bin <= fftSize/2; bin++ {
			freq := float64(bin) * float64(sr) / float64(fftSize)
			if freq < 65 || freq > 4000 {
				continue
			}
			semitones := 12 * math.Log2(freq/261.63)
			pc := ((int(math.Round(semitones)) % 12) + 12) % 12
			chroma[pc] += cmplx.Abs(spec[bin])
		}
	}

	bestCorr := -999.0
	bestKey := "C Major"
	for rot := 0; rot < 12; rot++ {
		rolled := make([]float64, 12)
		for j := 0; j < 12; j++ {
			rolled[j] = chroma[(j+rot)%12]
		}
		corrMaj := pearson(rolled, majProfile)
		corrMin := pearson(rolled, minProfile)
		if corrMaj > bestCorr {
			bestCorr = corrMaj
			bestKey = noteNames[rot] + " Major"
		}
		if corrMin > bestCorr {
			bestCorr = corrMin
			bestKey = noteNames[rot] + " Minor"
		}
	}
	return bestKey
}

func pearson(a, b []float64) float64 {
	n := len(a)
	if n == 0 || n != len(b) {
		return 0
	}
	var sumA, sumB, sumAB, sumA2, sumB2 float64
	for i := 0; i < n; i++ {
		sumA += a[i]
		sumB += b[i]
		sumAB += a[i] * b[i]
		sumA2 += a[i] * a[i]
		sumB2 += b[i] * b[i]
	}
	num := float64(n)*sumAB - sumA*sumB
	den := math.Sqrt((float64(n)*sumA2 - sumA*sumA) * (float64(n)*sumB2 - sumB*sumB))
	if den < 1e-12 {
		return 0
	}
	return num / den
}

// --- Segments & Highlights ---

func classifySegments(phrases []float64, beatEnergy []float64, beatTimes []float64, duration float64, gridSize int) []Segment {
	if len(phrases) == 0 {
		return nil
	}
	phraseEnergies := make([]float64, len(phrases))
	for i := range phrases {
		bStart := i * gridSize
		bEnd := (i + 1) * gridSize
		if bEnd > len(beatEnergy) {
			bEnd = len(beatEnergy)
		}
		if bStart >= len(beatEnergy) {
			phraseEnergies[i] = 0
			continue
		}
		sum := 0.0
		for j := bStart; j < bEnd; j++ {
			sum += beatEnergy[j]
		}
		phraseEnergies[i] = sum / float64(bEnd-bStart)
	}

	sorted := make([]float64, len(phraseEnergies))
	copy(sorted, phraseEnergies)
	sortFloat64s(sorted)
	lowIdx := int(float64(len(sorted)) * 0.3)
	highIdx := int(float64(len(sorted)) * 0.7)
	lowThresh := sorted[lowIdx]
	highThresh := sorted[highIdx]

	segments := make([]Segment, len(phrases))
	for i, pTime := range phrases {
		e := phraseEnergies[i]
		label := "Verse"
		relPos := pTime / duration
		switch {
		case relPos < 0.15 && e < highThresh:
			label = "Intro"
		case relPos > 0.85 && e < highThresh:
			label = "Outro"
		case e >= highThresh:
			label = "Chorus"
		case e <= lowThresh:
			label = "Bridge"
		}
		segments[i] = Segment{Time: pTime, Label: label, Energy: e, VocalEnergy: 0.5}
	}
	return segments
}

func detectHighlights(beatTimes []float64, energy []float64) []Highlight {
	windowSize := 64
	if len(beatTimes) < windowSize || len(energy) < windowSize {
		end := 0.0
		if len(beatTimes) > 0 {
			end = beatTimes[len(beatTimes)-1]
		}
		return []Highlight{{StartTime: 0, EndTime: end, Score: 0}}
	}
	var candidates []Highlight
	for i := 0; i+windowSize <= len(energy); i += 16 {
		sum := 0.0
		for j := i; j < i+windowSize; j++ {
			sum += energy[j]
		}
		avg := sum / float64(windowSize)
		endIdx := i + windowSize - 1
		if endIdx >= len(beatTimes) {
			endIdx = len(beatTimes) - 1
		}
		candidates = append(candidates, Highlight{
			StartBeatIdx: i, EndBeatIdx: i + windowSize,
			StartTime: beatTimes[i], EndTime: beatTimes[endIdx],
			Score: avg,
		})
	}
	// Sort by score desc, take top 3
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].Score > candidates[i].Score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	return candidates
}

func sortFloat64s(a []float64) {
	for i := 0; i < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			if a[j] < a[i] {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}
