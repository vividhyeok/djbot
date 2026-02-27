package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	clearDir("output")
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
