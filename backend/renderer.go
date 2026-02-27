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
			"-af", "loudnorm=I=-14:TP=-2.0:LRA=11",
			wavPath,
		)
		if err := cmd.Run(); err != nil {
			log.Printf("Warning: failed to convert to wav: %v", err)
		} else {
			playlist[i].Filepath = wavPath
			wavMap = append(wavMap, wavPath)
		}
	}

	// Calculate absolute times on the master timeline
	// Each slice needs [StartTimeSec, EndTimeSec] from its source,
	// and AbsoluteMs to be delayed by.
	type timelineNode struct {
		Path      string
		StartSec  float64
		EndSec    float64
		DelayMs   int     // Absolute position in the final mix
		TransType string  // entry transition type for THIS node
		EntryFade float64 // entry fade duration in seconds
		ExitType  string  // exit transition type for THIS node
		ExitFade  float64 // exit fade duration in seconds
	}

	nodes := make([]timelineNode, len(playlist))
	var trackStarts []struct {
		OffsetMs int
		Name     string
	}

	currentOffsetMs := 0
	prevChunkMs := 0

	for i := 0; i < len(playlist); i++ {
		t := playlist[i]

		startSec := t.PlayStart
		endSec := t.PlayEnd

		// PlayEnd must always be >= Duration because we set it to Duration in ComputePlayBounds.
		// But just in case, guarantee it's the full track if zero/negative.
		if endSec <= 0 {
			endSec = t.Duration
		}

		// CRITICAL: startSec must NEVER be >= endSec. Clamp hard.
		if startSec < 0 {
			startSec = 0
		}
		if startSec >= endSec-15.0 {
			startSec = math.Max(0, endSec-15.0)
		}

		// Total physical audio chunk length we decode from this track
		chunkPhysicalDurSec := endSec - startSec

		var xfadeDurMs int = 0
		var transType string = "crossfade"

		if i > 0 {
			trans := transitions[i-1]
			transType = trans.Type
			// Crossfade duration in milliseconds from the plan
			xfadeDurMs = int(math.Round(trans.Duration * 1000.0))

			// Floor crossfade at 4 seconds minimum for audible effect
			if xfadeDurMs < 4000 {
				xfadeDurMs = 4000
			}

			// Clamp: can't overlap MORE than what has been rendered so far (prev chunk)
			// AND can't overlap more than B's total duration - 5s
			maxByPrev := prevChunkMs - 1000
			maxByB := int(chunkPhysicalDurSec*1000.0) - 5000
			if xfadeDurMs > maxByPrev && maxByPrev > 0 {
				xfadeDurMs = maxByPrev
			}
			if xfadeDurMs > maxByB && maxByB > 0 {
				xfadeDurMs = maxByB
			}
			if xfadeDurMs < 0 {
				xfadeDurMs = 0
			}

			// Pydub core mechanism: overlap = subtract crossfade from timeline position
			currentOffsetMs -= xfadeDurMs
			if currentOffsetMs < 0 {
				currentOffsetMs = 0
			}

			fadeSec := float64(xfadeDurMs) / 1000.0
			nodes[i].EntryFade = fadeSec
			nodes[i].TransType = transType
			nodes[i-1].ExitFade = fadeSec
			nodes[i-1].ExitType = transType
		}

		log.Printf("[timeline] track[%d] %s: start=%.1fs end=%.1fs chunk=%.1fs offset=%dms xfade=%dms",
			i, t.Filename, startSec, endSec, chunkPhysicalDurSec, currentOffsetMs, xfadeDurMs)

		trackStarts = append(trackStarts, struct {
			OffsetMs int
			Name     string
		}{currentOffsetMs, t.Filename})

		nodes[i].Path = t.Filepath
		nodes[i].StartSec = startSec
		nodes[i].EndSec = endSec
		nodes[i].DelayMs = currentOffsetMs

		prevChunkMs = int(math.Round(chunkPhysicalDurSec * 1000.0))
		currentOffsetMs += prevChunkMs
	}

	// -----------------------------------------------------
	// PCM Canvas Mixed Rendering (Pydub Equivalent)
	// -----------------------------------------------------
	var canvas []float32

	for i, n := range nodes {
		durRaw := n.EndSec - n.StartSec
		if durRaw < 0 {
			durRaw = 0
		}

		baseFilter := fmt.Sprintf("atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS", n.StartSec, n.EndSec)

		// Transition effects
		entryFilter := ""
		if n.EntryFade > 0 {
			switch n.TransType {
			case "bass_swap":
				entryFilter = fmt.Sprintf(",afade=t=in:d=%.3f", n.EntryFade)
			case "filter_fade":
				entryFilter = fmt.Sprintf(",afade=t=in:d=%.3f", n.EntryFade)
			case "mashup":
				entryFilter = ",highpass=f=300,volume=1dB"
			case "cut":
				// immediate in
			default:
				entryFilter = fmt.Sprintf(",afade=t=in:d=%.3f", n.EntryFade)
			}
		}

		exitFilter := ""
		if n.ExitFade > 0 {
			fadeStart := durRaw - n.ExitFade
			if fadeStart < 0 {
				fadeStart = 0
			}

			switch n.ExitType {
			case "bass_swap":
				exitFilter = fmt.Sprintf(",highpass=f=300,afade=t=out:st=%.3f:d=%.3f", fadeStart, n.ExitFade)
			case "filter_fade":
				exitFilter = fmt.Sprintf(",lowpass=f=400,afade=t=out:st=%.3f:d=%.3f", fadeStart, n.ExitFade)
			case "mashup":
				exitFilter = ",volume=-1dB"
			case "cut":
				exitFilter = fmt.Sprintf(",afade=t=out:st=%.3f:d=0.01", fadeStart)
			default:
				exitFilter = fmt.Sprintf(",afade=t=out:st=%.3f:d=%.3f", fadeStart, n.ExitFade)
			}
		}

		filterChain := baseFilter + entryFilter + exitFilter
		pcmPath := filepath.Join(cacheDir, fmt.Sprintf("chunk_%d_%s.pcm", i, randHex(4)))

		// Render this specific filtered chunk to raw PCM floats
		cmdArgs := []string{
			"-y", "-i", n.Path,
			"-map_metadata", "-1",
			"-af", filterChain,
			"-f", "f32le", "-ar", "44100", "-ac", "2",
			pcmPath,
		}

		cmdRaw := exec.Command(ffmpegPath, cmdArgs...)
		cmdRaw.Stderr = os.Stderr
		if err := cmdRaw.Run(); err != nil {
			log.Printf("Warning: failed to extract PCM chunk %d: %v", i, err)
			continue
		}

		// Read PCM into memory
		b, err := os.ReadFile(pcmPath)
		if err != nil {
			log.Printf("Warning: failed to read PCM chunk %d: %v", i, err)
			continue
		}

		pcmFloatCount := len(b) / 4
		offsetSamples := int(float64(n.DelayMs)/1000.0*44100.0) * 2
		requiredLen := offsetSamples + pcmFloatCount

		// Expand canvas if necessary
		if len(canvas) < requiredLen {
			newCanvas := make([]float32, requiredLen)
			copy(newCanvas, canvas)
			canvas = newCanvas
		}

		// Additive Mixing (Overlay)
		for j := 0; j < pcmFloatCount; j++ {
			val := math.Float32frombits(binary.LittleEndian.Uint32(b[j*4 : j*4+4]))
			canvas[offsetSamples+j] += val
		}

		os.Remove(pcmPath)
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
