package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ytdlpPath is the resolved path to the yt-dlp binary. Empty means not yet found.
// Protected by ytdlpMu for concurrent access (auto-update goroutine vs HTTP handlers).
var ytdlpPath string
var ytdlpMu sync.RWMutex

// ytdlpBinDir is the managed directory where we self-download/update yt-dlp.
// Set by main() from the --data-dir flag so we always have a writable location.
var ytdlpBinDir string

// ytdlpDownloading is true while an initial background download is in progress.
var ytdlpDownloading atomic.Bool

func getYtdlpPath() string {
	ytdlpMu.RLock()
	defer ytdlpMu.RUnlock()
	return ytdlpPath
}

func setYtdlpPath(p string) {
	ytdlpMu.Lock()
	ytdlpPath = p
	ytdlpMu.Unlock()
}

// ytdlpExeName returns the OS-appropriate binary filename.
func ytdlpExeName() string {
	if runtime.GOOS == "windows" {
		return "yt-dlp.exe"
	}
	return "yt-dlp"
}

// initYtdlp resolves the yt-dlp binary, downloading it if necessary,
// then kicks off a background auto-update. It returns immediately so that
// the HTTP server can start while the download proceeds in the background.
func initYtdlp() {
	exeName := ytdlpExeName()

	// 1. YTDLP_PATH env override (highest priority)
	if p := os.Getenv("YTDLP_PATH"); p != "" {
		ytdlpPath = p
		log.Printf("[yt-dlp] using YTDLP_PATH: %s", ytdlpPath)
		go tryAutoUpdateYtdlp()
		return
	}

	// 2. Search candidates in priority order:
	//    a) managed bin dir  (self-downloaded — always writable)
	//    b) same dir as this exe  (bundled installer)
	//    c) system PATH
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	var candidates []string
	if ytdlpBinDir != "" {
		candidates = append(candidates, filepath.Join(ytdlpBinDir, exeName))
	}
	candidates = append(candidates, filepath.Join(exeDir, exeName))
	if pathBin, err := exec.LookPath("yt-dlp"); err == nil {
		candidates = append(candidates, pathBin)
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			setYtdlpPath(abs)
			log.Printf("[yt-dlp] found: %s", abs)
			break
		}
	}

	// 3. Not found anywhere → download asynchronously into managed dir
	if getYtdlpPath() == "" {
		if ytdlpBinDir != "" {
			log.Printf("[yt-dlp] not found — downloading latest release in background...")
			ytdlpDownloading.Store(true)
			go func() {
				defer ytdlpDownloading.Store(false)
				if err := downloadYtdlp(ytdlpBinDir); err != nil {
					log.Printf("[yt-dlp] download failed: %v", err)
					return
				}
				setYtdlpPath(filepath.Join(ytdlpBinDir, exeName))
				log.Printf("[yt-dlp] ready: %s", getYtdlpPath())
			}()
			return // server starts immediately; user can retry after download
		}
		log.Printf("[yt-dlp] not found and no managed dir — YouTube downloads unavailable")
		return
	}

	// 4. Auto-update in background (non-blocking)
	go tryAutoUpdateYtdlp()
}

// tryAutoUpdateYtdlp runs yt-dlp -U if the binary is writable. If it is a
// read-only system install, a fresh copy is downloaded into the managed dir.
func tryAutoUpdateYtdlp() {
	cur := getYtdlpPath()
	if cur == "" {
		return
	}

	// Check write permission before attempting -U (avoids confusing permission errors)
	f, err := os.OpenFile(cur, os.O_WRONLY|os.O_APPEND, 0)
	writable := err == nil
	if f != nil {
		f.Close()
	}

	if writable {
		log.Printf("[yt-dlp] auto-update: %s -U", cur)
		cmd := exec.Command(cur, "-U")
		hideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("[yt-dlp] auto-update warning: %v", err)
		}
		if first := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2); len(first) > 0 && first[0] != "" {
			log.Printf("[yt-dlp] auto-update: %s", first[0])
		}
		return
	}

	// Read-only (system install) → download a writable managed copy
	if ytdlpBinDir != "" {
		log.Printf("[yt-dlp] system binary is read-only; downloading managed copy to %s", ytdlpBinDir)
		if err := downloadYtdlp(ytdlpBinDir); err != nil {
			log.Printf("[yt-dlp] managed copy download failed: %v", err)
			return
		}
		managed := filepath.Join(ytdlpBinDir, ytdlpExeName())
		setYtdlpPath(managed)
		log.Printf("[yt-dlp] switched to managed copy: %s", managed)
	}
}

// downloadYtdlp fetches the latest yt-dlp release for the current OS/arch
// and saves it into dir with the correct executable permissions.
func downloadYtdlp(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	// yt-dlp release asset names per platform
	var assetName string
	switch runtime.GOOS {
	case "windows":
		assetName = "yt-dlp.exe"
	case "darwin":
		assetName = "yt-dlp_macos" // universal binary (arm64 + x86_64)
	default: // linux
		assetName = "yt-dlp"
	}

	dlURL := "https://github.com/yt-dlp/yt-dlp/releases/latest/download/" + assetName
	log.Printf("[yt-dlp] downloading from %s", dlURL)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(dlURL)
	if err != nil {
		return fmt.Errorf("GET: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET: HTTP %d", resp.StatusCode)
	}

	destName := ytdlpExeName()
	tmp := filepath.Join(dir, destName+".download")

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("write: %w", err)
	}
	f.Close()

	dest := filepath.Join(dir, destName)
	os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(dest, 0755)
	}
	log.Printf("[yt-dlp] saved to %s", dest)
	return nil
}

// ──────────────────────────────────────────────────────────────────────────
// HTTP request / response types
// ──────────────────────────────────────────────────────────────────────────

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
// Root cause of the Korean-path bug on Windows: yt-dlp outputs paths in CP949
// but Go treats subprocess stdout as bytes and re-interprets them as UTF-8,
// corrupting the path. Fix: force PYTHONUTF8=1 + PYTHONIOENCODING=utf-8 so
// yt-dlp outputs real UTF-8, then use filepath.Base() for the filename and
// re-join with the known outputDir (a correct UTF-8 string held by Go).
func DownloadYouTubePlaylist(url, outputDir string, maxTracks int) ([]DownloadedFile, error) {
	if getYtdlpPath() == "" {
		if ytdlpDownloading.Load() {
			return nil, fmt.Errorf("yt-dlp is still downloading (first run). Please wait a moment and try again.")
		}
		return nil, fmt.Errorf("yt-dlp is not available. " +
			"Please install it from https://github.com/yt-dlp/yt-dlp/releases/latest " +
			"or restart the app to trigger the automatic download.")
	}

	if outputDir == "" {
		outputDir = uploadsDir
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	outputTemplate := filepath.Join(outputDir, "%(title)s.%(ext)s")

	// Common args shared across all retry stages.
	//
	// Key: "--format bestaudio[ext=m4a]/bestaudio" forces yt-dlp to pick
	// audio-only CDN URLs instead of video+audio combined streams. Audio CDN
	// URLs are served from a different infrastructure and are far less likely
	// to be rate-limited or 403'd by YouTube's bot-detection.
	baseArgs := []string{
		url,
		"--format", "bestaudio[ext=m4a]/bestaudio[ext=webm]/bestaudio",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--output", outputTemplate,
		"--no-playlist-reverse",
		"--no-part",
		"--no-cache-dir",
		"--ignore-errors",
		"--add-metadata",
		"--retries", "5",
		"--fragment-retries", "5",
		"--print", "after_move:filepath",
	}
	if maxTracks > 0 {
		baseArgs = append(baseArgs, "--playlist-end", fmt.Sprintf("%d", maxTracks))
	}

	// ── Multi-stage retry strategy ────────────────────────────────────────
	//
	// YouTube 403 Forbidden errors most commonly come from:
	//   a) Bot detection on bulk playlist downloads → add request delays
	//   b) Authentication requirement (members-only, age-restricted)
	//      → browser cookie fallback
	//
	// We stop as soon as stdout contains at least one downloaded file path.
	type retryStage struct {
		label     string
		extraArgs []string
	}
	stages := []retryStage{
		// Stage 1: default (fastest, works for most public playlists)
		{label: "default", extraArgs: nil},
		// Stage 2: add sleep between requests to avoid rate-limiting
		{label: "slow", extraArgs: []string{"--sleep-requests", "2", "--sleep-interval", "1"}},
		// Stage 3-5: try browser cookies (handles login-required content)
		{label: "chrome", extraArgs: []string{"--cookies-from-browser", "chrome"}},
		{label: "edge", extraArgs: []string{"--cookies-from-browser", "edge"}},
		{label: "firefox", extraArgs: []string{"--cookies-from-browser", "firefox"}},
	}

	var lastOut []byte
	var lastErr error

	for _, stage := range stages {
		args := make([]string, len(baseArgs))
		copy(args, baseArgs)
		args = append(args, stage.extraArgs...)

		log.Printf("[yt-dlp] [%s] attempting download...", stage.label)
		cmd := exec.Command(getYtdlpPath(), args...)
		hideWindow(cmd)
		// Force UTF-8 output from Python/yt-dlp on every platform.
		cmd.Env = append(os.Environ(), "PYTHONUTF8=1", "PYTHONIOENCODING=utf-8")

		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		lastOut = out
		lastErr = err

		if len(out) > 0 {
			log.Printf("[yt-dlp] [%s] success — %d file(s) downloaded", stage.label, strings.Count(strings.TrimSpace(string(out)), "\n")+1)
			break
		}

		errStr := stderr.String()
		log.Printf("[yt-dlp] [%s] failed: %v\nstderr: %s", stage.label, err, errStr)
	}

	out := lastOut
	err := lastErr

	// Parse stdout: each line is an absolute path printed by --print after_move:filepath
	var files []DownloadedFile
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		name := filepath.Base(line)
		if seen[name] {
			continue
		}
		seen[name] = true

		absPath := filepath.Join(outputDir, name)
		if _, statErr := os.Stat(absPath); statErr != nil {
			log.Printf("[yt-dlp] file not found on disk: %s", name)
			continue
		}
		title := strings.TrimSuffix(name, filepath.Ext(name))
		title = strings.ReplaceAll(title, "_", " ")
		files = append(files, DownloadedFile{Path: absPath, Filename: name, Title: title})
		log.Printf("[yt-dlp] ready: %s", name)
	}

	if len(files) == 0 && err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("yt-dlp failed after all attempts: %w\n%s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("yt-dlp failed after all attempts: %w", err)
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
