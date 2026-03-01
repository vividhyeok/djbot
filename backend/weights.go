package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// weightsFilePath is set to an absolute path by main() after dirs are configured.
var weightsFilePath = "preference_weights.json"

// DefaultWeights returns factory-default transition type and bar weights.
func DefaultWeights() WeightsConfig {
	return WeightsConfig{
		TypeWeights: map[string]float64{
			"crossfade":   0.5,
			"bass_swap":   1.6,
			"cut":         1.2,
			"filter_fade": 1.0,
			"mashup":      1.0,
		},
		BarWeights: map[int]float64{
			4: 1.0,
			8: 1.3,
		},
	}
}

// WeightsConfig holds user-preferred transition weights.
type WeightsConfig struct {
	TypeWeights map[string]float64 `json:"type_weights"`
	BarWeights  map[int]float64    `json:"bar_weights"`
}

// loadWeights reads weights from disk, falling back to defaults.
func loadWeights() WeightsConfig {
	data, err := os.ReadFile(weightsFilePath)
	if err != nil {
		return DefaultWeights()
	}
	var cfg WeightsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("[weights] parse error, using defaults: %v", err)
		return DefaultWeights()
	}
	return cfg
}

// saveWeights persists weights to disk.
func saveWeights(cfg WeightsConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(weightsFilePath)
	if dir != "." {
		os.MkdirAll(dir, 0755)
	}
	return os.WriteFile(weightsFilePath, data, 0644)
}

// handleGetWeights returns current weights (file or defaults).
func handleGetWeights(w http.ResponseWriter, r *http.Request) {
	cfg := loadWeights()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

// handleSaveWeights saves user weights to disk.
func handleSaveWeights(w http.ResponseWriter, r *http.Request) {
	var cfg WeightsConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := saveWeights(cfg); err != nil {
		http.Error(w, "save failed: "+err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}
