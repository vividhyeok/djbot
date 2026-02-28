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

	// 1. Check local directory first (important for bundled installers)
	exePath, _ := os.Executable()
	localPath := filepath.Join(filepath.Dir(exePath), "yt-dlp.exe")
	if _, err := os.Stat(localPath); err == nil {
		ytdlpPath = localPath
	} else if _, err := os.Stat("yt-dlp.exe"); err == nil {
		abs, _ := filepath.Abs("yt-dlp.exe")
		ytdlpPath = abs
	} else if path, err := exec.LookPath("yt-dlp"); err == nil {
		// 2. Fallback to system PATH
		ytdlpPath = path
	}

	if ytdlpPath == "yt-dlp" {
		log.Println("[yt-dlp] not found in local or PATH")
	} else {
		log.Printf("[yt-dlp] using: %s", ytdlpPath)
	}

	// ── Auto-update yt-dlp in the background to fix 403 Forbidden errors ──
	go func() {
		log.Printf("[yt-dlp] attempting auto-update on startup: %s -U", ytdlpPath)
		// We use -U to ensure even if it's an old version on another PC, it gets patched.
		cmd := exec.Command(ytdlpPath, "-U")
		hideWindow(cmd)
		if err := cmd.Run(); err != nil {
			log.Printf("[yt-dlp] auto-update failed: %v", err)
		} else {
			log.Printf("[yt-dlp] auto-update completed successfully.")
		}
	}()
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
		"--no-cache-dir",
		"--ignore-errors",
		"--geo-bypass",
		"--add-metadata",
		// User-agent helps with bot detection
		"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		"--print", "after_move:filepath",
	}
	if maxTracks > 0 {
		args = append(args, "--playlist-end", fmt.Sprintf("%d", maxTracks))
	}

	log.Printf("[yt-dlp] Downloading: %s (max=%d)", url, maxTracks)

	cmd := exec.Command(ytdlpPath, args...)
	hideWindow(cmd)
	// Force Python to use UTF-8 for stdout so Korean chars are not corrupted.
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
			log.Printf("[yt-dlp] file not found: %s", name)
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
