package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStagedClipCacheReusesUnchangedAudio(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg required")
	}
	dir, work := t.TempDir(), t.TempDir()
	source := filepath.Join(dir, "source.wav")
	if err := writeToneWAV(source, 3); err != nil {
		t.Fatal(err)
	}
	track := TrackEntry{Filepath: source, Duration: 3}
	clip := RenderClip{Duration: 3, Regions: []RenderRegion{{0, 3, 1, 3}}}
	first, err := cachedRenderClip(track, clip, nil, nil, dir, work)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(first)
	clip.Start = 100
	second, err := cachedRenderClip(track, clip, nil, nil, dir, work)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(second)
	if first != second || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged clip re-rendered")
	}
	track.LoudnessDB = -30
	third, err := cachedRenderClip(track, clip, nil, nil, dir, work)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatal("gain changes must invalidate staged audio")
	}
}
