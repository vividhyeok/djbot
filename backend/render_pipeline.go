package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const renderSampleRate = 44100

func sampleCount(seconds float64) int64 { return int64(math.Round(seconds * renderSampleRate)) }

func runAudioCommand(args ...string) error {
	cmd := exec.Command(ffmpegPath, append([]string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y"}, args...)...)
	hideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("audio processing: %w: %s", err, stderr.String())
	}
	return nil
}

// Stage each source independently to float PCM. No complete mix is held in Go
// memory, and decoder failure aborts the export instead of silently losing a song.
func renderClip(path string, loudness float64, clip RenderClip, incoming, outgoing *TransitionSpec, output, graphPath string) error {
	var graph []string
	var labels strings.Builder
	for j, r := range clip.Regions {
		label := fmt.Sprintf("r%d", j)
		filter := fmt.Sprintf("[0:a]atrim=start=%.9f:end=%.9f,asetpts=PTS-STARTPTS,aresample=%d,aformat=sample_fmts=fltp:channel_layouts=stereo,atempo=%.9f,apad,atrim=end_sample=%d[%s]", r.SourceStart, r.SourceEnd, renderSampleRate, r.Speed, sampleCount(r.Duration), label)
		graph = append(graph, filter)
		fmt.Fprintf(&labels, "[%s]", label)
	}
	graph = append(graph, fmt.Sprintf("%sconcat=n=%d:v=0:a=1[clip]", labels.String(), len(clip.Regions)))
	filter := "[clip]anull"
	// Analysis currently measures RMS dBFS, not integrated LUFS. Apply one
	// conservative static compensation; never normalize again after this stage.
	if finite(loudness) && loudness < -1 && loudness > -70 {
		filter += fmt.Sprintf(",volume=%.6fdB", clampF(-18-loudness, -8, 8))
	}
	if incoming != nil {
		filter += fadeExpression(*incoming, true, 0)
	}
	if outgoing != nil {
		filter += fadeExpression(*outgoing, false, clip.Duration-outgoing.Duration)
	}
	// Short edge ramps prevent a source trim from introducing a sample step.
	filter += fmt.Sprintf(",afade=t=in:d=0.003,afade=t=out:st=%.9f:d=0.003,apad,atrim=end_sample=%d[out]", math.Max(0, clip.Duration-.003), sampleCount(clip.Duration))
	graph = append(graph, filter)
	if err := os.WriteFile(graphPath, []byte(strings.Join(graph, ";")), 0600); err != nil {
		return err
	}
	return runAudioCommand("-i", path, "-filter_complex_script", graphPath, "-map", "[out]", "-ar", "44100", "-c:a", "pcm_f32le", output)
}

func RenderFinalMix(playlist []TrackEntry, transitions []TransitionSpec, outputPath, cacheDir string) (string, string, error) {
	clips, err := BuildRenderTimeline(playlist, transitions)
	if err != nil {
		return "", "", err
	}
	ext := strings.ToLower(filepath.Ext(outputPath))
	if ext != ".wav" && ext != ".mp3" {
		return "", "", fmt.Errorf("output must be .wav or .mp3")
	}
	for _, track := range playlist {
		if samePath(track.Filepath, outputPath) {
			return "", "", fmt.Errorf("output cannot overwrite a source track")
		}
		if st, err := os.Stat(track.Filepath); err != nil || st.IsDir() {
			return "", "", fmt.Errorf("source unavailable: %s", track.Filename)
		}
	}
	work, err := os.MkdirTemp(cacheDir, "render-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(work)
	args := []string{}
	var graph []string
	var labels strings.Builder
	for i, track := range playlist {
		var in, out *TransitionSpec
		if i > 0 {
			in = &transitions[i-1]
		}
		if i < len(transitions) {
			out = &transitions[i]
		}
		staged := filepath.Join(work, fmt.Sprintf("clip-%d.wav", i))
		if err := renderClip(track.Filepath, track.LoudnessDB, clips[i], in, out, staged, filepath.Join(work, "clip.ffgraph")); err != nil {
			return "", "", fmt.Errorf("track %d (%s): %w", i+1, track.Filename, err)
		}
		args = append(args, "-i", staged)
		graph = append(graph, fmt.Sprintf("[%d:a]adelay=%dS:all=1[c%d]", i, sampleCount(clips[i].Start), i))
		fmt.Fprintf(&labels, "[c%d]", i)
	}
	total := clips[len(clips)-1].Start + clips[len(clips)-1].Duration
	graph = append(graph, fmt.Sprintf("%samix=inputs=%d:duration=longest:normalize=0,alimiter=limit=0.89:attack=5:release=50:level=false:latency=true,apad,atrim=end_sample=%d[out]", labels.String(), len(clips), sampleCount(total)))
	graphPath := filepath.Join(work, "mix.ffgraph")
	if err = os.WriteFile(graphPath, []byte(strings.Join(graph, ";")), 0600); err != nil {
		return "", "", err
	}
	// Render beside the destination so publication is a same-filesystem rename.
	tmp, err := os.CreateTemp(filepath.Dir(outputPath), ".djbot-*"+ext)
	if err != nil {
		return "", "", err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)
	args = append(args, "-filter_complex_script", graphPath, "-map", "[out]", "-ar", "44100")
	if ext == ".wav" {
		args = append(args, "-c:a", "pcm_s24le")
	} else {
		args = append(args, "-c:a", "libmp3lame", "-b:a", "320k")
	}
	args = append(args, tmpPath)
	if err = runAudioCommand(args...); err != nil {
		return "", "", err
	}
	if err = os.Rename(tmpPath, outputPath); err != nil {
		return "", "", err
	}
	lrcPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".lrc"
	var lrc strings.Builder
	lrc.WriteString("[ar:DJBOT]\n[ti:Auto Mix]\n")
	for i, c := range clips {
		fmt.Fprintf(&lrc, "[%02d:%05.2f] %s\n", int(c.Start)/60, math.Mod(c.Start, 60), strings.ReplaceAll(playlist[i].Filename, "\n", " "))
	}
	if err = os.WriteFile(lrcPath, []byte(lrc.String()), 0644); err != nil {
		return outputPath, "", fmt.Errorf("audio saved but track index failed: %w", err)
	}
	return outputPath, lrcPath, nil
}

// Sample-evaluated gains avoid volume-frame stair steps. Gain edits are scoped
// to the transition; the natural-speed body retains its normalized level.
func fadeExpression(t TransitionSpec, incoming bool, start float64) string {
	x := fmt.Sprintf("clip((t-%.9f)/%.9f,0,1)", start, t.Duration)
	power := 1.0
	switch t.Curve {
	case "sharp":
		power = .45
	case "smooth":
		power = 2.1
	case "custom":
		power = .35 + clampF(t.CurveAmount, 0, 1)*3
	}
	shape := fmt.Sprintf("pow(1-(%s),%.6f)", x, power)
	if incoming {
		shape = fmt.Sprintf("pow(%s,%.6f)", x, power)
	}
	if t.Curve == "equal_power" || t.Curve == "" {
		shape = "cos((" + x + ")*PI/2)"
		if incoming {
			shape = "sin((" + x + ")*PI/2)"
		}
	}
	gain := t.GainA
	if incoming {
		gain = t.GainB
	}
	if gain == 0 && t.Curve == "" {
		gain = 1
	} // legacy projects omit gain
	gain = clampF(gain, 0, 2)
	expr := fmt.Sprintf("(%s)*%.6f", shape, gain)
	if incoming {
		expr = fmt.Sprintf("if(lt(t,%.9f),%s,1)", t.Duration, expr)
	} else {
		expr = fmt.Sprintf("if(lt(t,%.9f),1,%s)", start, expr)
	}
	return ",aeval=exprs='val(0)*(" + expr + ")|val(1)*(" + expr + ")'"
}

func samePath(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	return strings.EqualFold(a, b)
}
