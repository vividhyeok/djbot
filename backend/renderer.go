package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"crypto/rand"
	mrand "math/rand"
)

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// RenderPreview renders a transition preview using ffmpeg filter_complex
func RenderPreview(trackAPath, trackBPath string, spec TransitionSpec, cacheDir string) (string, error) {
	margin := 10.0
	overlap := spec.Duration
	if overlap <= 0 {
		overlap = 10
	}
	tOut := spec.AOutTime
	tIn := spec.BInTime
	speedA := spec.SpeedA
	speedB := spec.SpeedB
	if speedA <= 0 {
		speedA = 1.0
	}
	if speedB <= 0 {
		speedB = 1.0
	}

	// A: from (tOut - margin) for (margin + overlap) seconds
	aStart := tOut - margin
	if aStart < 0 {
		aStart = 0
	}
	aDur := margin + overlap

	// B: from tIn for (overlap + margin) seconds
	bStart := tIn
	bDur := overlap + margin

	// Delay for B in the mix (margin scaled by speed)
	delayMs := int(margin / speedA * 1000)
	fadeDur := overlap / speedA

	// Build filter_complex based on transition type
	var filterComplex string

	// Speed filter (atempo only supports 0.5-100.0, chain for larger changes)
	// Speed filter (atempo only supports 0.5-100.0, chain for larger changes)
	atempoA := buildAtempoFilter(speedA, 0.0)
	atempoB := buildAtempoFilter(speedB, spec.PitchStepB)

	switch spec.Type {
	case "bass_swap":
		filterComplex = fmt.Sprintf(
			"[0:a]%s,highpass=f=300,afade=t=out:st=%.2f:d=%.2f[a];"+
				"[1:a]%s,adelay=%d|%d,afade=t=in:d=%.2f[b];"+
				"[a][b]amix=inputs=2:duration=longest:normalize=0[out]",
			atempoA, margin/speedA, fadeDur,
			atempoB, delayMs, delayMs, fadeDur,
		)
	case "cut":
		cutPoint := margin / speedA
		filterComplex = fmt.Sprintf(
			"[0:a]%s,atrim=0:%.2f[a];[1:a]%s[b];[a][b]concat=n=2:v=0:a=1[out]",
			atempoA, cutPoint, atempoB,
		)
	case "filter_fade":
		filterComplex = fmt.Sprintf(
			"[0:a]%s,lowpass=f=400,afade=t=out:st=%.2f:d=%.2f[a];"+
				"[1:a]%s,adelay=%d|%d,afade=t=in:d=%.2f[b];"+
				"[a][b]amix=inputs=2:duration=longest:normalize=0[out]",
			atempoA, margin/speedA, fadeDur,
			atempoB, delayMs, delayMs, fadeDur,
		)
	case "mashup":
		filterComplex = fmt.Sprintf(
			"[0:a]%s,volume=-1dB[a];"+
				"[1:a]%s,highpass=f=300,volume=1dB,adelay=%d|%d[b];"+
				"[a][b]amix=inputs=2:duration=longest:normalize=0[out]",
			atempoA, atempoB, delayMs, delayMs,
		)
	default: // crossfade
		filterComplex = fmt.Sprintf(
			"[0:a]%s,afade=t=out:st=%.2f:d=%.2f[a];"+
				"[1:a]%s,adelay=%d|%d,afade=t=in:d=%.2f[b];"+
				"[a][b]amix=inputs=2:duration=longest:normalize=0[out]",
			atempoA, margin/speedA, fadeDur,
			atempoB, delayMs, delayMs, fadeDur,
		)
	}

	outputPath := filepath.Join(cacheDir, fmt.Sprintf("preview_%s_%d_%s.mp3",
		spec.Type, int(tOut), randHex(4)))

	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.2f", aStart), "-t", fmt.Sprintf("%.2f", aDur), "-i", trackAPath,
		"-ss", fmt.Sprintf("%.2f", bStart), "-t", fmt.Sprintf("%.2f", bDur), "-i", trackBPath,
		"-filter_complex", filterComplex,
		"-map", "[out]",
		"-b:a", "192k",
		outputPath,
	}

	log.Printf("[render preview] %s -> %s (%s)", filepath.Base(trackAPath), filepath.Base(trackBPath), spec.Type)

	cmd := exec.Command(ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg preview: %w", err)
	}
	return outputPath, nil
}

// RenderFinalMix renders the full mixset to MP3 + LRC using a single native FFmpeg filter_complex
func RenderFinalMix(playlist []TrackEntry, transitions []TransitionSpec, outputPath, cacheDir string) (string, string, error) {
	if len(playlist) < 2 {
		return "", "", fmt.Errorf("need at least 2 tracks")
	}

	log.Printf("[render mix] %d tracks, %d transitions (Go Native Mega filter_complex)", len(playlist), len(transitions))

	// Ensure tracks are pre-converted/normalized so timing is guaranteed length
	var wavMap []string
	for i, t := range playlist {
		wavPath := filepath.Join(cacheDir, fmt.Sprintf("norm_%s.wav", randHex(6)))
		cmd := exec.Command(ffmpegPath, "-y", "-i", t.Filepath,
			"-map_metadata", "-1",
			"-ar", "44100", "-ac", "2", "-sample_fmt", "s16",
			// dynaudnorm: does NOT change file duration (unlike loudnorm single-pass),
			// so prevChunkMs iTheory remains accurate enough for xfade clamping.
			"-af", "dynaudnorm=f=150:g=15:p=0.95",
			wavPath,
		)
		if err := cmd.Run(); err != nil {
			log.Printf("Warning: failed to convert to wav: %v", err)
		} else {
			playlist[i].Filepath = wavPath
			wavMap = append(wavMap, wavPath)
		}
	}

	// -------------------------------------------------------
	// Single-loop: timeline planning + PCM overlay combined
	// prevActualChunkMs is derived from the real FFmpeg output
	// (byte count) instead of the theory value, eliminating
	// the LRC drift that accumulates over many tracks.
	// -------------------------------------------------------
	var canvas []float32
	var trackStarts []struct {
		OffsetMs int
		Name     string
	}

	currentOffsetMs := 0
	prevActualChunkMs := 0 // real PCM length of the **previous** track (ms)

	// We still need to know entry/exit fade per track upfront because
	// the exit fade of track[i] depends on knowing the crossfade of
	// track[i+1], which hasn't been processed yet.
	// Solution: compute all fades in a tiny pre-pass (no FFmpeg calls),
	// then run the single PCM loop.
	type fadeInfo struct {
		EntryFade float64
		EntryType string
		ExitFade  float64
		ExitType  string
	}
	fades := make([]fadeInfo, len(playlist))

	// Pre-pass: compute xfade durations using theory lengths only for clamping.
	// These determine the fade envelopes applied to each chunk.
	{
		prevTheoryMs := 0
		for i := 0; i < len(playlist); i++ {
			t := playlist[i]
			startSec := t.PlayStart
			endSec := t.PlayEnd
			if endSec <= 0 {
				endSec = t.Duration
			}
			if startSec < 0 {
				startSec = 0
			}
			if startSec >= endSec-15.0 {
				startSec = math.Max(0, endSec-15.0)
			}
			chunkTheorySec := endSec - startSec

			if i > 0 {
				trans := transitions[i-1]
				xfadeMs := int(math.Round(trans.Duration * 1000.0))
				if xfadeMs < 4000 {
					xfadeMs = 4000
				}
				maxByPrev := prevTheoryMs - 1000
				maxByB := int(chunkTheorySec*1000.0) - 5000
				if xfadeMs > maxByPrev && maxByPrev > 0 {
					xfadeMs = maxByPrev
				}
				if xfadeMs > maxByB && maxByB > 0 {
					xfadeMs = maxByB
				}
				if xfadeMs < 0 {
					xfadeMs = 0
				}
				fadeSec := float64(xfadeMs) / 1000.0
				fades[i].EntryFade = fadeSec
				fades[i].EntryType = trans.Type
				fades[i-1].ExitFade = fadeSec
				fades[i-1].ExitType = trans.Type
			}
			prevTheoryMs = int(math.Round(chunkTheorySec * 1000.0))
		}
	}

	// Main single loop: for each track, clamp xfade using prevActualChunkMs,
	// extract PCM, record LRC from real offsetSamples, mix into canvas.
	for i := 0; i < len(playlist); i++ {
		t := playlist[i]

		startSec := t.PlayStart
		endSec := t.PlayEnd
		if endSec <= 0 {
			endSec = t.Duration
		}
		if startSec < 0 {
			startSec = 0
		}
		if startSec >= endSec-15.0 {
			startSec = math.Max(0, endSec-15.0)
		}
		chunkTheorySec := endSec - startSec

		// ── Step 1: xfade clamping (actual prev chunk length) ──────────────
		if i > 0 {
			trans := transitions[i-1]
			xfadeMs := int(math.Round(trans.Duration * 1000.0))
			if xfadeMs < 4000 {
				xfadeMs = 4000
			}
			// Use prevActualChunkMs (real PCM size) — not theory
			maxByPrev := prevActualChunkMs - 1000
			maxByB := int(chunkTheorySec*1000.0) - 5000
			if xfadeMs > maxByPrev && maxByPrev > 0 {
				xfadeMs = maxByPrev
			}
			if xfadeMs > maxByB && maxByB > 0 {
				xfadeMs = maxByB
			}
			if xfadeMs < 0 {
				xfadeMs = 0
			}

			// ── Step 2: overlay position ───────────────────────────────────
			currentOffsetMs -= xfadeMs
			if currentOffsetMs < 0 {
				currentOffsetMs = 0
			}
		}

		log.Printf("[render] track[%d] %s: start=%.1fs end=%.1fs offset=%dms (prevActual=%dms)",
			i, t.Filename, startSec, endSec, currentOffsetMs, prevActualChunkMs)

		// Build FFmpeg filter chain (entry/exit fades from pre-pass)
		f := fades[i]
		durRaw := endSec - startSec
		if durRaw < 0 {
			durRaw = 0
		}

		baseFilter := fmt.Sprintf("atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS", startSec, endSec)

		entryFilter := ""
		if f.EntryFade > 0 {
			switch f.EntryType {
			case "bass_swap":
				entryFilter = fmt.Sprintf(",afade=t=in:d=%.3f", f.EntryFade)
			case "filter_fade":
				entryFilter = fmt.Sprintf(",afade=t=in:d=%.3f", f.EntryFade)
			case "mashup":
				entryFilter = ",highpass=f=300,volume=1dB"
			case "cut":
				// immediate in — no filter
			default:
				entryFilter = fmt.Sprintf(",afade=t=in:d=%.3f", f.EntryFade)
			}
		}

		exitFilter := ""
		if f.ExitFade > 0 {
			fadeStart := durRaw - f.ExitFade
			if fadeStart < 0 {
				fadeStart = 0
			}
			switch f.ExitType {
			case "bass_swap":
				exitFilter = fmt.Sprintf(",highpass=f=300,afade=t=out:st=%.3f:d=%.3f", fadeStart, f.ExitFade)
			case "filter_fade":
				exitFilter = fmt.Sprintf(",lowpass=f=400,afade=t=out:st=%.3f:d=%.3f", fadeStart, f.ExitFade)
			case "mashup":
				exitFilter = ",volume=-1dB"
			case "cut":
				exitFilter = fmt.Sprintf(",afade=t=out:st=%.3f:d=0.01", fadeStart)
			default:
				exitFilter = fmt.Sprintf(",afade=t=out:st=%.3f:d=%.3f", fadeStart, f.ExitFade)
			}
		}

		filterChain := baseFilter + entryFilter + exitFilter
		pcmPath := filepath.Join(cacheDir, fmt.Sprintf("chunk_%d_%s.pcm", i, randHex(4)))

		// ── Step 3: FFmpeg → PCM ───────────────────────────────────────────
		cmdRaw := exec.Command(ffmpegPath,
			"-y", "-i", t.Filepath,
			"-map_metadata", "-1",
			"-af", filterChain,
			"-f", "f32le", "-ar", "44100", "-ac", "2",
			pcmPath,
		)
		cmdRaw.Stderr = os.Stderr
		if err := cmdRaw.Run(); err != nil {
			log.Printf("Warning: failed to extract PCM chunk %d: %v", i, err)
			continue
		}

		// ── Step 4: read PCM → real sample count ───────────────────────────
		b, err := os.ReadFile(pcmPath)
		if err != nil {
			log.Printf("Warning: failed to read PCM chunk %d: %v", i, err)
			continue
		}
		pcmFloatCount := len(b) / 4

		// ── Step 5: LRC trackStarts — from real currentOffsetMs ───────────
		trackStarts = append(trackStarts, struct {
			OffsetMs int
			Name     string
		}{currentOffsetMs, t.Filename})

		// ── Step 6: canvas additive overlay ───────────────────────────────
		offsetSamples := int(float64(currentOffsetMs)/1000.0*44100.0) * 2
		requiredLen := offsetSamples + pcmFloatCount
		if len(canvas) < requiredLen {
			newCanvas := make([]float32, requiredLen)
			copy(newCanvas, canvas)
			canvas = newCanvas
		}
		for j := 0; j < pcmFloatCount; j++ {
			val := math.Float32frombits(binary.LittleEndian.Uint32(b[j*4 : j*4+4]))
			canvas[offsetSamples+j] += val
		}
		os.Remove(pcmPath)

		// ── Step 7: prevActualChunkMs from real byte count ─────────────────
		prevActualChunkMs = pcmFloatCount * 1000 / (44100 * 2)

		// ── Step 8: advance timeline ───────────────────────────────────────
		currentOffsetMs += prevActualChunkMs
	}

	// -----------------------------------------------------
	// Write Master Canvas to Disk & Encode Final MP3
	// -----------------------------------------------------
	finalPcmPath := filepath.Join(cacheDir, fmt.Sprintf("final_canvas_%s.pcm", randHex(4)))

	// Pre-allocate byte array for max speed
	outPcmBytes := make([]byte, len(canvas)*4)
	for j := 0; j < len(canvas); j++ {
		binary.LittleEndian.PutUint32(outPcmBytes[j*4:j*4+4], math.Float32bits(canvas[j]))
	}

	if err := os.WriteFile(finalPcmPath, outPcmBytes, 0644); err != nil {
		return "", "", fmt.Errorf("failed to drop master PCM to disk: %w", err)
	}

	log.Printf("[ffmpeg] encoding final mp3 from master PCM overlay...")
	encodeArgs := []string{
		"-y",
		"-f", "f32le", "-ar", "44100", "-ac", "2",
		"-i", finalPcmPath,
		"-af", "volume=-2.0dB", // prevent clipping from additive mixing
		"-b:a", "320k", "-q:a", "0",
		outputPath,
	}

	encCmd := exec.Command(ffmpegPath, encodeArgs...)
	encCmd.Stderr = os.Stderr
	encCmd.Stdout = os.Stdout
	if err := encCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to encode final mp3: %w", err)
	}

	os.Remove(finalPcmPath)

	// Write LRC
	lrcPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".lrc"
	var lrcSb strings.Builder
	lrcSb.WriteString("[ar:DJ Bot Auto Mix]\n[ti:Go Native PCM Canvas Mix]\n[al:Auto Generated]\n[by:DJ Bot]\n\n")

	for _, ts := range trackStarts {
		sec := float64(ts.OffsetMs) / 1000.0
		m := int(sec) / 60
		s := sec - float64(m*60)
		ext := filepath.Ext(ts.Name)
		name := ts.Name
		if ext != "" {
			name = strings.TrimSuffix(ts.Name, ext)
		}
		lrcSb.WriteString(fmt.Sprintf("[%02d:%05.2f] %s\n", m, s, name))
	}
	os.WriteFile(lrcPath, []byte(lrcSb.String()), 0644)

	// Cleanup Normalized WAVs
	for _, wPath := range wavMap {
		os.Remove(wPath)
	}

	log.Printf("[done] canvas overlay successfully created mix: %s, lrc: %s", outputPath, lrcPath)
	return outputPath, lrcPath, nil
}

func buildAtempoFilter(speed float64, pitchStep float64) string {
	filter := ""

	// Handle tempo speed change
	if speed > 0 && !(speed > 0.99 && speed < 1.01) {
		filter += fmt.Sprintf("atempo=%.4f", speed)
	}

	// Handle pitch shifting natively if requested
	if pitchStep != 0.0 {
		if filter != "" {
			filter += ","
		}
		filter += fmt.Sprintf("rubberband=pitch=%.2f", pitchStep)
	}

	if filter == "" {
		return "anull"
	}
	return filter
}

func init() {
	mrand.New(mrand.NewSource(0)) // suppress unused import
}
