package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ytdlpPath = "yt-dlp"

func initYtdlp() {
	if p := os.Getenv("YTDLP_PATH"); p != "" {
		ytdlpPath = p
		return
	}
	if path, err := exec.LookPath("yt-dlp"); err == nil {
		ytdlpPath = path
		return
	}
	for _, c := range []string{"yt-dlp.exe", filepath.Join(".", "yt-dlp.exe")} {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			ytdlpPath = abs
			return
		}
	}
	log.Println("[yt-dlp] not found in PATH")
}

type DownloadRequest struct {
	URL       string `json:"url"`
	OutputDir string `json:"output_dir,omitempty"`
	MaxTracks int    `json:"max_tracks,omitempty"`
}

type DownloadResponse struct {
	Files []DownloadedFile `json:"files"`
	Error string           `json:"error,omitempty"`
}

type DownloadedFile struct {
	Path     string `json:"path"`
	Filename string `json:"filename"`
	Title    string `json:"title"`
}

// DownloadYouTubePlaylist downloads audio via yt-dlp.
//
// Root cause of the Korean-path bug: on Korean Windows yt-dlp outputs paths in CP949,
// but Go reads subprocess stdout as bytes and treats them as UTF-8, corrupting the path.
// Fix: set PYTHONUTF8=1 + PYTHONIOENCODING=utf-8 on the subprocess so yt-dlp outputs
// real UTF-8. Then use filepath.Base() to extract only the sanitised ASCII filename,
// and re-join with the known outputDir (which Go holds as a correct UTF-8 string).
func DownloadYouTubePlaylist(url, outputDir string, maxTracks int) ([]DownloadedFile, error) {
	if outputDir == "" {
		outputDir = uploadsDir
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	outputTemplate := filepath.Join(outputDir, "%(title)s.%(ext)s")

	args := []string{
		url,
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--output", outputTemplate,
		"--no-playlist-reverse",
		"--no-warnings",
		"--no-part",
		"--no-mtime",
		"--no-cache-dir",
		"--ignore-errors", // Don't abort on error (continue if one video is unavailable)
		"--geo-bypass",    // Attempt to bypass geographic restrictions
		"--add-metadata",
		// Removed --restrict-filenames to preserve non-English titles natively
		// Removed --extractor-args "youtube:player-client=ios,android,web" due to recent "Video unavailable" issues
		"--print", "after_move:filepath",
	}
	if maxTracks > 0 {
		args = append(args, "--playlist-end", fmt.Sprintf("%d", maxTracks))
	}

	log.Printf("[yt-dlp] Downloading: %s (max=%d)", url, maxTracks)

	cmd := exec.Command(ytdlpPath, args...)
	// Force Python to use UTF-8 for stdout so Korean parent-directory chars are not corrupted.
	cmd.Env = append(os.Environ(), "PYTHONUTF8=1", "PYTHONIOENCODING=utf-8")

	out, err := cmd.Output()

	// In --ignore-errors mode, yt-dlp exits with non-zero if ANY video in the playlist fails.
	// But it still prints the successful paths to stdout.
	// So we process stdout first, and only return a hard error if NO files were downloaded at all.
	var files []DownloadedFile
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		name := filepath.Base(line) // Native Unicode basename
		if seen[name] {
			continue
		}
		seen[name] = true

		absPath := filepath.Join(outputDir, name)
		if _, statErr := os.Stat(absPath); statErr != nil {
			log.Printf("[yt-dlp] file not found: %s (raw line: %q)", name, line)
			continue
		}
		title := strings.TrimSuffix(name, filepath.Ext(name))
		title = strings.ReplaceAll(title, "_", " ")
		files = append(files, DownloadedFile{
			Path:     absPath,
			Filename: name,
			Title:    title,
		})
		log.Printf("[yt-dlp] Ready: %s", name)
	}

	// If NO files were successfully downloaded, THEN return the original command error
	if len(files) == 0 && err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("yt-dlp failed: %w\n%s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("yt-dlp failed: %w", err)
	}

	return files, nil
}

// handleDownloadYouTube handles POST /download/youtube
func handleDownloadYouTube(w http.ResponseWriter, r *http.Request) {
	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.URL == "" {
		http.Error(w, "url is required", 400)
		return
	}
	if req.MaxTracks <= 0 {
		req.MaxTracks = 30
	}

	absUploads, _ := filepath.Abs(uploadsDir)
	files, err := DownloadYouTubePlaylist(req.URL, absUploads, req.MaxTracks)

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(DownloadResponse{Error: err.Error()})
		return
	}
	json.NewEncoder(w).Encode(DownloadResponse{Files: files})
}
