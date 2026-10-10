package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in local-only QA; no user source or generated audio is uploaded or retained.
func TestLocalLibraryPacingAndAudition(t *testing.T) {
	dir := os.Getenv("DJBOT_QA_ANALYSIS_DIR")
	if dir == "" {
		t.Skip("set DJBOT_QA_ANALYSIS_DIR for local audio QA")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*_analysis_v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tracks []TrackAnalysis
	for _, f := range files {
		data, e := os.ReadFile(f)
		if e != nil {
			continue
		}
		var a TrackAnalysis
		if json.Unmarshal(data, &a) != nil || a.Duration < 60 {
			continue
		}
		if _, e = os.Stat(a.Filepath); e != nil {
			continue
		}
		tracks = append(tracks, a)
		if len(tracks) == 6 {
			break
		}
	}
	if len(tracks) < 2 {
		t.Fatal("need at least two available analysed sources")
	}
	for _, mode := range []string{"quick", "balanced", "long"} {
		p := GenerateMixPlanV2(tracks, nil, nil, MixSettings{ListeningMode: mode})
		entries := make([]TrackEntry, len(p.SortedTracks))
		for i, a := range p.SortedTracks {
			entries[i] = TrackEntry{Filepath: a.Filepath, Duration: a.Duration, LoudnessDB: a.LoudnessDB}
		}
		timeline, e := BuildRenderTimeline(entries, p.Selections)
		if e != nil {
			t.Fatal(e)
		}
		t.Logf("%s: %d tracks, first exit %.2fs, timeline %.2fs (last source retained in this API fixture)", mode, len(tracks), p.Selections[0].AOutTime, timeline[len(timeline)-1].Start+timeline[len(timeline)-1].Duration)
		tr := p.Selections[0]
		entries = entries[:2]
		entries[0].PlayStart = math.Max(0, tr.AOutTime-8)
		entries[1].PlayEnd = math.Min(entries[1].Duration, tr.BInTime+tr.Duration*playbackSpeed(tr.SpeedB)+8)
		work := t.TempDir()
		start := time.Now()
		out, _, e := RenderFinalMix(entries, []TransitionSpec{tr}, filepath.Join(work, "audition.wav"), work)
		if e != nil {
			t.Fatal(e)
		}
		cold := time.Since(start)
		start = time.Now()
		if _, _, e = RenderFinalMix(entries, []TransitionSpec{tr}, filepath.Join(work, "again.wav"), work); e != nil {
			t.Fatal(e)
		}
		t.Logf("audition cold=%v cached-stage=%v", cold, time.Since(start))
		pcm, _, e := decodeToPCM(out)
		if e != nil {
			t.Fatal(e)
		}
		peak, energy := 0.0, 0.0
		for _, s := range pcm {
			v := float64(s)
			if !finite(v) {
				t.Fatal("non-finite sample")
			}
			peak = math.Max(peak, math.Abs(v))
			energy += v * v
		}
		rms := math.Sqrt(energy / float64(len(pcm)))
		if peak > .9 || rms < .001 {
			t.Fatalf("invalid output peak %.4f rms %.4f", peak, rms)
		}
		t.Logf("peak=%.4f RMS=%.4f; not a listening evaluation", peak, rms)
	}
}
