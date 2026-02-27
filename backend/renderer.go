package main

import (
	"fmt"
	"log"
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
	atempoA := buildAtempoFilter(speedA)
	atempoB := buildAtempoFilter(speedB)

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

// RenderFinalMix renders the full mixset to MP3 + LRC
func RenderFinalMix(playlist []TrackEntry, transitions []TransitionSpec, outputPath, cacheDir string) (string, string, error) {
	if len(playlist) < 2 {
		return "", "", fmt.Errorf("need at least 2 tracks")
	}

	log.Printf("[render mix] %d tracks, %d transitions", len(playlist), len(transitions))

	var segmentFiles []string
	var trackStarts []struct {
		OffsetMs int
		Name     string
	}

	totalMs := 0

	// 1. First track body: from PlayStart to transition mix point
	if len(transitions) > 0 {
		t0 := transitions[0]
		playStart0 := playlist[0].PlayStart
		bodyEnd := t0.AOutTime - t0.Duration
		bodyDur := bodyEnd - playStart0
		if bodyDur > 0 {
			seg := filepath.Join(cacheDir, fmt.Sprintf("seg_%d_%s.wav", 0, randHex(4)))
			err := extractSegment(playlist[0].Filepath, playStart0, bodyDur, 1.0, "", seg)
			if err != nil {
				return "", "", fmt.Errorf("body0: %w", err)
			}
			segmentFiles = append(segmentFiles, seg)
			trackStarts = append(trackStarts, struct {
				OffsetMs int
				Name     string
			}{0, playlist[0].Filename})
			totalMs += int(bodyDur * 1000)
		}
	}

	// 2. For each transition: render mixed zone + next track body
	for i, trans := range transitions {
		trackA := playlist[i]
		trackB := playlist[i+1]

		// Render transition zone
		transSeg := filepath.Join(cacheDir, fmt.Sprintf("trans_%d_%s.wav", i, randHex(4)))
		err := renderTransitionSegment(trackA.Filepath, trackB.Filepath, trans, transSeg)
		if err != nil {
			return "", "", fmt.Errorf("trans%d: %w", i, err)
		}
		segmentFiles = append(segmentFiles, transSeg)

		trackStarts = append(trackStarts, struct {
			OffsetMs int
			Name     string
		}{totalMs, trackB.Filename})
		totalMs += int(trans.Duration * 1000)

		// Body of track B (from after transition to PlayEnd or next transition start)
		bBodyStart := trans.BInTime + trans.Duration
		var bBodyEnd float64
		if i+1 < len(transitions) {
			nextTrans := transitions[i+1]
			bBodyEnd = nextTrans.AOutTime - nextTrans.Duration
		} else {
			// Last track: use PlayEnd if set, else Duration
			if trackB.PlayEnd > bBodyStart {
				bBodyEnd = trackB.PlayEnd
			} else {
				bBodyEnd = trackB.Duration
			}
		}
		bBodyDur := bBodyEnd - bBodyStart
		if bBodyDur > 0 {
			seg := filepath.Join(cacheDir, fmt.Sprintf("body_%d_%s.wav", i+1, randHex(4)))
			err := extractSegment(trackB.Filepath, bBodyStart, bBodyDur, 1.0, "", seg)
			if err != nil {
				return "", "", fmt.Errorf("body%d: %w", i+1, err)
			}
			segmentFiles = append(segmentFiles, seg)
			totalMs += int(bBodyDur * 1000)
		}
	}

	// 3. Concatenate all segments
	concatList := filepath.Join(cacheDir, fmt.Sprintf("concat_%s.txt", randHex(4)))
	var sb strings.Builder
	for _, f := range segmentFiles {
		sb.WriteString(fmt.Sprintf("file '%s'\n", strings.ReplaceAll(f, "'", "'\\''")))
	}
	os.WriteFile(concatList, []byte(sb.String()), 0644)

	cmd := exec.Command(ffmpegPath,
		"-y", "-f", "concat", "-safe", "0", "-i", concatList,
		"-b:a", "320k", "-q:a", "0", outputPath,
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("concat: %w", err)
	}

	// 4. Write LRC
	lrcPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".lrc"
	writeLRC(trackStarts, lrcPath)

	// 5. Cleanup temp files
	for _, f := range segmentFiles {
		os.Remove(f)
	}
	os.Remove(concatList)

	log.Printf("[done] mix: %s, lrc: %s", outputPath, lrcPath)
	return outputPath, lrcPath, nil
}

func extractSegment(inputPath string, startSec, durSec, speed float64, filterType, outputPath string) error {
	atempo := buildAtempoFilter(speed)
	filter := atempo
	if filterType == "highpass" {
		filter += ",highpass=f=300"
	} else if filterType == "lowpass" {
		filter += ",lowpass=f=400"
	}

	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-t", fmt.Sprintf("%.3f", durSec),
		"-i", inputPath,
		"-af", filter,
		"-ar", "44100", "-ac", "2",
		outputPath,
	}
	cmd := exec.Command(ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func renderTransitionSegment(trackAPath, trackBPath string, spec TransitionSpec, outputPath string) error {
	overlap := spec.Duration
	speedA := spec.SpeedA
	speedB := spec.SpeedB
	if speedA <= 0 {
		speedA = 1.0
	}
	if speedB <= 0 {
		speedB = 1.0
	}

	aStart := spec.AOutTime - overlap
	if aStart < 0 {
		aStart = 0
	}

	atempoA := buildAtempoFilter(speedA)
	atempoB := buildAtempoFilter(speedB)

	fadeDur := overlap / speedA
	if fadeDur <= 0 {
		fadeDur = 1
	}

	var fc string
	switch spec.Type {
	case "bass_swap":
		fc = fmt.Sprintf(
			"[0:a]%s,highpass=f=300,afade=t=out:st=0:d=%.2f[a];[1:a]%s,afade=t=in:d=%.2f[b];[a][b]amix=inputs=2:duration=shortest:normalize=0[out]",
			atempoA, fadeDur, atempoB, fadeDur)
	case "cut":
		// Discard track A's output with anullsink to prevent unconnected pad errors,
		// or simply don't route [0:a] anywhere. However, if any input is completely unused
		// ffmpeg might map it automatically. We'll explicitly sink it.
		fc = fmt.Sprintf("[0:a]%s,anullsink;[1:a]%s,acopy[out]", atempoA, atempoB)
	case "mashup":
		fc = fmt.Sprintf(
			"[0:a]%s,volume=-1dB[a];[1:a]%s,highpass=f=300,volume=1dB[b];[a][b]amix=inputs=2:duration=shortest:normalize=0[out]",
			atempoA, atempoB)
	default: // crossfade, filter_fade
		fc = fmt.Sprintf(
			"[0:a]%s,afade=t=out:st=0:d=%.2f[a];[1:a]%s,afade=t=in:d=%.2f[b];[a][b]amix=inputs=2:duration=shortest:normalize=0[out]",
			atempoA, fadeDur, atempoB, fadeDur)
	}

	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", aStart), "-t", fmt.Sprintf("%.3f", overlap), "-i", trackAPath,
		"-ss", fmt.Sprintf("%.3f", spec.BInTime), "-t", fmt.Sprintf("%.3f", overlap), "-i", trackBPath,
		"-filter_complex", fc,
		"-map", "[out]",
		"-ar", "44100", "-ac", "2",
		outputPath,
	}
	cmd := exec.Command(ffmpegPath, args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildAtempoFilter(speed float64) string {
	if speed <= 0 || (speed > 0.99 && speed < 1.01) {
		return "anull"
	}
	// atempo accepts 0.5 to 100.0; chain for values outside range
	var parts []string
	s := speed
	for s < 0.5 {
		parts = append(parts, "atempo=0.5")
		s /= 0.5
	}
	for s > 100.0 {
		parts = append(parts, "atempo=100.0")
		s /= 100.0
	}
	parts = append(parts, fmt.Sprintf("atempo=%.4f", s))
	return strings.Join(parts, ",")
}

func writeLRC(trackStarts []struct {
	OffsetMs int
	Name     string
}, path string) {
	var sb strings.Builder
	sb.WriteString("[ar:DJ Bot Auto Mix]\n")
	sb.WriteString("[ti:Hip-Hop Club Mix]\n")
	sb.WriteString("[al:Auto Generated]\n")
	sb.WriteString("[by:DJ Bot Go Worker]\n\n")

	for _, ts := range trackStarts {
		sec := float64(ts.OffsetMs) / 1000.0
		m := int(sec) / 60
		s := sec - float64(m*60)
		// Strip extension robustly — handles names like "artist - song.mp3"
		ext := filepath.Ext(ts.Name)
		name := ts.Name
		if ext != "" {
			name = strings.TrimSuffix(ts.Name, ext)
		}
		sb.WriteString(fmt.Sprintf("[%02d:%05.2f] %s\n", m, s, name))
	}
	os.WriteFile(path, []byte(sb.String()), 0644)
	log.Printf("[lrc] %d tracks written to %s", len(trackStarts), path)
}

func init() {
	mrand.New(mrand.NewSource(0)) // suppress unused import
}
