package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

var cacheDir = "cache"
var uploadsDir = "cache/uploads"
var outputDir = "output"
var binDir = "bin" // managed directory for self-downloaded binaries (e.g. yt-dlp)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	ffmpegFlag := flag.String("ffmpeg", "", "Path to ffmpeg executable")
	dataDirFlag := flag.String("data-dir", ".", "Root directory for cache and output")
	flag.Parse()

	if *ffmpegFlag != "" {
		os.Setenv("FFMPEG_PATH", *ffmpegFlag)
	}

	// Adjust paths based on data-dir
	if *dataDirFlag != "." {
		absData, _ := filepath.Abs(*dataDirFlag)
		cacheDir = filepath.Join(absData, "cache")
		uploadsDir = filepath.Join(cacheDir, "uploads")
		outputDir = filepath.Join(absData, "output")
		binDir = filepath.Join(absData, "bin")
		weightsFilePath = filepath.Join(absData, "preference_weights.json")
	}

	// Wire the managed bin directory to the downloader before init.
	ytdlpBinDir = binDir

	initFFmpeg()
	initYtdlp()

	// Ensure directories
	os.MkdirAll(cacheDir, 0755)
	os.MkdirAll(uploadsDir, 0755)
	os.MkdirAll(outputDir, 0755)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /analyze", handleAnalyze)
	mux.HandleFunc("POST /upload", handleUpload)
	mux.HandleFunc("POST /plan", handlePlan)
	mux.HandleFunc("POST /render/preview", handleRenderPreview)
	mux.HandleFunc("POST /render/mix", handleRenderMix)
	mux.HandleFunc("POST /download/youtube", handleDownloadYouTube)
	mux.HandleFunc("GET /weights", handleGetWeights)
	mux.HandleFunc("POST /weights", handleSaveWeights)
	mux.HandleFunc("POST /export/zip", handleExportZip)
	mux.HandleFunc("POST /cache/clear", handleCacheClear)
	mux.HandleFunc("GET /files/serve", handleServeFile)

	// Listen on random port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	// Print port for Python bridge / Tauri to read
	fmt.Printf("PORT:%d\n", port)
	log.Printf("Go worker listening on :%d (ffmpeg: %s)", port, ffmpegPath)

	// Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("Shutting down...")
		os.Exit(0)
	}()

	if err := http.Serve(listener, corsMiddleware(mux)); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

// handleUpload accepts multipart file uploads and saves them to uploadsDir.
// Returns JSON: {"files": [{"path": "...", "filename": "..."}]}
func handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(512 << 20); err != nil { // 512 MB max
		http.Error(w, "parse form: "+err.Error(), 400)
		return
	}

	type fileResult struct {
		Path     string `json:"path"`
		Filename string `json:"filename"`
	}
	var results []fileResult

	for _, fhs := range r.MultipartForm.File {
		for _, fh := range fhs {
			src, err := fh.Open()
			if err != nil {
				continue
			}
			defer src.Close()

			// Sanitize filename
			name := filepath.Base(fh.Filename)
			name = strings.ReplaceAll(name, "..", "_")
			dst := filepath.Join(uploadsDir, name)

			out, err := os.Create(dst)
			if err != nil {
				continue
			}
			io.Copy(out, src)
			out.Close()

			absPath, _ := filepath.Abs(dst)
			results = append(results, fileResult{Path: absPath, Filename: name})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"files": results})
}

// handlePlan accepts analyzed tracks and returns mix plan (sorted order + transitions).
func handlePlan(w http.ResponseWriter, r *http.Request) {
	var req PlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if req.Scenarios <= 0 {
		req.Scenarios = 5
	}
	plan := GenerateMixPlan(req.Tracks, req.TypeWeights, req.BarWeights, req.Scenarios)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PlanResponse{Plan: plan})
}

func handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	absCache, _ := filepath.Abs(cacheDir)
	results, errs := AnalyzeBatch(req.Filepaths, absCache)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AnalyzeResponse{
		Results: results,
		Errors:  errs,
	})
}

func handleRenderPreview(w http.ResponseWriter, r *http.Request) {
	var req RenderPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	absCache, _ := filepath.Abs(cacheDir)
	outPath, err := RenderPreview(req.TrackAPath, req.TrackBPath, req.Spec, absCache)

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(RenderPreviewResponse{Error: err.Error()})
		return
	}
	json.NewEncoder(w).Encode(RenderPreviewResponse{OutputPath: outPath})
}

func handleRenderMix(w http.ResponseWriter, r *http.Request) {
	var req RenderMixRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	absCache, _ := filepath.Abs(cacheDir)
	mp3, lrc, err := RenderFinalMix(req.Playlist, req.Transitions, req.OutputPath, absCache)

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(RenderMixResponse{Error: err.Error()})
		return
	}
	json.NewEncoder(w).Encode(RenderMixResponse{MP3Path: mp3, LRCPath: lrc})
}
