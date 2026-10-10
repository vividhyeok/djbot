package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var clipCacheMu sync.Mutex

// Source content and all audible parameters key the immutable staged PCM.
// Arrangement start is excluded: moving an unchanged clip needs no DSP rerun.
func cachedRenderClip(track TrackEntry, clip RenderClip, in, out *TransitionSpec, cache, work string) (string, error) {
	hash, err := fileHash(track.Filepath)
	if err != nil {
		return "", err
	}
	clip.Start = 0
	key, _ := json.Marshal(struct {
		Version  int
		Hash     string
		Loudness float64
		Clip     RenderClip
		In, Out  *TransitionSpec
	}{2, hash, track.LoudnessDB, clip, in, out})
	sum := sha256.Sum256(key)
	path := filepath.Join(cache, fmt.Sprintf("stage_%x.wav", sum[:16]))
	clipCacheMu.Lock()
	defer clipCacheMu.Unlock()
	if st, err := os.Stat(path); err == nil && st.Size() > 44 {
		return path, nil
	}
	tmp := filepath.Join(work, fmt.Sprintf("stage_%x.wav", sum[:16]))
	if err := renderClip(track.Filepath, track.LoudnessDB, clip, in, out, tmp, filepath.Join(work, "clip.ffgraph")); err != nil {
		return "", err
	}
	// Keep persistent PCM cache bounded. Overflow remains temporary for this job.
	var size int64
	files, _ := filepath.Glob(filepath.Join(cache, "stage_*.wav"))
	for _, f := range files {
		if st, e := os.Stat(f); e == nil {
			size += st.Size()
		}
	}
	if st, e := os.Stat(tmp); e == nil && size+st.Size() > 2*1024*1024*1024 {
		return tmp, nil
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, nil
}
