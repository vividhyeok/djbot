package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var cacheDir = "cache"

func main() {
	ffmpegFlag := flag.String("ffmpeg", "", "Path to ffmpeg executable")
	flag.Parse()
	if *ffmpegFlag != "" {
		os.Setenv("FFMPEG_PATH", *ffmpegFlag)
	}
	initFFmpeg()

	// Ensure cache directory
	os.MkdirAll(cacheDir, 0755)
	os.MkdirAll("output", 0755)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /analyze", handleAnalyze)
	mux.HandleFunc("POST /render/preview", handleRenderPreview)
	mux.HandleFunc("POST /render/mix", handleRenderMix)

	// Listen on random port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	// Print port for Python bridge to read
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

	if err := http.Serve(listener, mux); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// Resolve cache dir relative to working directory
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
