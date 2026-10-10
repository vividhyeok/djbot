package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
)

// Preview uses the same decoder, timeline, normalization, effects and limiter
// as export. Serializing cache publication prevents simultaneous requests from
// consuming partially encoded files. Analysis normally hits the import cache.
var previewRenderMu sync.Mutex

func RenderPreview(trackAPath, trackBPath string, spec TransitionSpec, cacheDir string) (string, error) {
	previewRenderMu.Lock()
	defer previewRenderMu.Unlock()
	a, err := AnalyzeTrack(trackAPath, cacheDir)
	if err != nil {
		return "", err
	}
	b, err := AnalyzeTrack(trackBPath, cacheDir)
	if err != nil {
		return "", err
	}
	key := transitionPreviewKey(a.Hash, b.Hash, spec)
	out := filepath.Join(cacheDir, "preview_v4_"+key+".wav")
	if st, err := os.Stat(out); err == nil && st.Size() > 44 {
		return out, nil
	}
	playlist := []TrackEntry{
		{Filepath: trackAPath, Filename: filepath.Base(trackAPath), Duration: a.Duration, LoudnessDB: a.LoudnessDB, PlayStart: math.Max(0, spec.AOutTime-8)},
		{Filepath: trackBPath, Filename: filepath.Base(trackBPath), Duration: b.Duration, LoudnessDB: b.LoudnessDB, PlayEnd: math.Min(b.Duration, spec.BInTime+spec.Duration*playbackSpeed(spec.SpeedB)+8)},
	}
	path, index, err := RenderFinalMix(playlist, []TransitionSpec{spec}, out, cacheDir)
	if err != nil {
		return "", fmt.Errorf("preview: %w", err)
	}
	_ = os.Remove(index)
	return path, nil
}
