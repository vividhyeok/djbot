package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ExportZipRequest expects absolute paths to an MP3 and an LRC file
type ExportZipRequest struct {
	Mp3Path string `json:"mp3_path"`
	LrcPath string `json:"lrc_path"`
	MixName string `json:"mix_name,omitempty"` // Base name for the zip and contents
}

// handleExportZip packages the provided files into a ZIP and sends it as response
func handleExportZip(w http.ResponseWriter, r *http.Request) {
	var req ExportZipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Mp3Path == "" || req.LrcPath == "" {
		http.Error(w, "mp3_path and lrc_path required", http.StatusBadRequest)
		return
	}

	baseName := req.MixName
	if baseName == "" {
		baseName = "AutoMix"
	}

	// Make sure baseName is safe
	safeName := filepath.Base(baseName)
	if ext := filepath.Ext(safeName); ext != "" {
		safeName = safeName[:len(safeName)-len(ext)]
	}

	// Prepare ZIP response
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeName+`.zip"`)

	zw := zip.NewWriter(w)
	defer zw.Close()

	if err := addFileToZip(zw, req.Mp3Path, safeName+".mp3"); err != nil {
		log.Printf("Failed to zip mp3: %v", err)
		http.Error(w, "Failed to zip mp3", http.StatusInternalServerError)
		return
	}
	if err := addFileToZip(zw, req.LrcPath, safeName+".lrc"); err != nil {
		log.Printf("Failed to zip lrc: %v", err)
		http.Error(w, "Failed to zip lrc", http.StatusInternalServerError)
		return
	}
}

func addFileToZip(zw *zip.Writer, filePath, zipFilePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipFilePath
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, f)
	return err
}

// handleCacheClear deletes contents of uploads and output directories
func handleCacheClear(w http.ResponseWriter, r *http.Request) {
	clearDir(uploadsDir)
	clearDir(outputDir)
	// Also clean up any _preview.mp3 and _analysis.json files in cache root if they got placed there
	clearPatternMatch(cacheDir, "*_preview.mp3")
	clearPatternMatch(cacheDir, "*_analysis.json")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func clearDir(dirPath string) {
	d, err := os.Open(dirPath)
	if err != nil {
		return
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return
	}
	for _, name := range names {
		os.RemoveAll(filepath.Join(dirPath, name))
	}
}

func clearPatternMatch(dirPath, pattern string) {
	files, _ := filepath.Glob(filepath.Join(dirPath, pattern))
	for _, f := range files {
		os.Remove(f)
	}
}

// isChildPath reports whether child is rooted inside parent.
// Uses filepath.Rel so it is correct on case-sensitive (Linux) filesystems.
func isChildPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// handleServeFile serves a local file as a binary stream for downloading.
// This prevents Tauri from navigating when setting asset:// URLs on <a> tags.
func handleServeFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path is required", 400)
		return
	}

	// Security: only allow paths inside cwd or the data directory.
	// filepath.Rel-based check is correct on case-sensitive (Linux) filesystems
	// unlike the previous strings.ToLower prefix comparison.
	absPath, _ := filepath.Abs(path)
	cwd, _ := filepath.Abs(".")
	absData, _ := filepath.Abs(filepath.Dir(cacheDir))
	if !isChildPath(cwd, absPath) && !isChildPath(absData, absPath) {
		http.Error(w, "forbidden path", 403)
		return
	}

	f, err := os.Open(absPath)
	if err != nil {
		http.Error(w, "file not found: "+err.Error(), 404)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		http.Error(w, "stat error", 500)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(absPath)))
	http.ServeContent(w, r, filepath.Base(absPath), info.ModTime(), f)
}
