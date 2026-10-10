package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

var listenMu sync.Mutex

// Listening does not require choosing an export name or writing into output.
// Repeated playback of an unchanged plan reuses its encoded listening cache.
func handleListen(w http.ResponseWriter, r *http.Request) {
	var req RenderMixRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if _, err := BuildRenderTimeline(req.Playlist, req.Transitions); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	identities := []string{}
	for _, track := range req.Playlist {
		h, err := fileHash(track.Filepath)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		identities = append(identities, h)
	}
	keyData, _ := json.Marshal(struct {
		Version     int
		Tracks      []TrackEntry
		Transitions []TransitionSpec
		Hashes      []string
	}{1, req.Playlist, req.Transitions, identities})
	sum := sha256.Sum256(keyData)
	dir, _ := filepath.Abs(cacheDir)
	path := filepath.Join(dir, fmt.Sprintf("listen_%x.mp3", sum[:16]))
	listenMu.Lock()
	defer listenMu.Unlock()
	if r.Context().Err() != nil {
		return
	}
	if st, err := os.Stat(path); err != nil || st.Size() < 1000 {
		_, index, err := RenderFinalMix(req.Playlist, req.Transitions, path, dir)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = os.Remove(index)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"output_path": path})
}
