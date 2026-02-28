package main

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var ffmpegPath = "ffmpeg"

func initFFmpeg() {
	if p := os.Getenv("FFMPEG_PATH"); p != "" {
		ffmpegPath = p
	}
}

// fileHash produces the same hash as Python utils.py get_file_hash
func fileHash(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	size := info.Size()
	chunkSize := int64(1024 * 1024)

	h := md5.New()
	h.Write([]byte(fmt.Sprintf("%d", size)))

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	head := make([]byte, chunkSize)
	n, _ := f.Read(head)
	h.Write(head[:n])

	if size > chunkSize {
		f.Seek(-chunkSize, io.SeekEnd)
		tail := make([]byte, chunkSize)
		n, _ = f.Read(tail)
		h.Write(tail[:n])
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// decodeToPCM decodes audio to mono float32 PCM at 22050Hz via ffmpeg
func decodeToPCM(path string) ([]float32, int, error) {
	sr := 22050
	cmd := exec.Command(ffmpegPath,
		"-v", "error",
		"-i", path,
		"-f", "f32le",
		"-acodec", "pcm_f32le",
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", sr),
		"-",
	)
	hideWindow(cmd)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, 0, fmt.Errorf("pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, 0, fmt.Errorf("start ffmpeg: %w (%s)", err, stderr.String())
	}

	data, err := io.ReadAll(stdout)
	if err != nil {
		return nil, 0, fmt.Errorf("read: %w", err)
	}
	if waitErr := cmd.Wait(); waitErr != nil {
		log.Printf("[ffmpeg stderr] %s", stderr.String())
	}

	numSamples := len(data) / 4
	if numSamples == 0 {
		return nil, 0, fmt.Errorf("no audio data decoded from %s (stderr: %s)", path, stderr.String())
	}

	samples := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		bits := binary.LittleEndian.Uint32(data[i*4 : i*4+4])
		samples[i] = math.Float32frombits(bits)
	}

	return samples, sr, nil
}

func loadCachedAnalysis(cachePath string) (*TrackAnalysis, error) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	var ta TrackAnalysis
	if err := json.Unmarshal(data, &ta); err != nil {
		return nil, err
	}
	return &ta, nil
}

func saveCachedAnalysis(cachePath string, ta *TrackAnalysis) error {
	data, err := json.MarshalIndent(ta, "", "  ")
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(cachePath), 0755)
	return os.WriteFile(cachePath, data, 0644)
}

// AnalyzeTrack runs the full analysis pipeline on a single track
func AnalyzeTrack(path, cacheDir string) (*TrackAnalysis, error) {
	hash, err := fileHash(path)
	if err != nil {
		return nil, fmt.Errorf("hash: %w", err)
	}

	cachePath := filepath.Join(cacheDir, hash+"_analysis.json")
	if cached, err := loadCachedAnalysis(cachePath); err == nil {
		log.Printf("[cache hit] %s", path)
		return cached, nil
	}

	log.Printf("[analyzing] %s", path)

	samples, sr, err := decodeToPCM(path)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	duration := float64(len(samples)) / float64(sr)
	loudness := computeLoudnessDB(samples)

	hopSize := 512
	frameSize := 1024
	onset := computeOnsetEnvelope(samples, sr, frameSize, hopSize)

	bpm := estimateBPM(onset, sr, hopSize)
	beatTimes := estimateBeatTimes(onset, sr, duration, bpm, hopSize)
	energy := computeBeatEnergy(samples, sr, beatTimes)
	key := detectKey(samples, sr)

	// Phrases: every 32 beats
	gridSize := 32
	var phrases []float64
	for i := 0; i < len(beatTimes); i += gridSize {
		phrases = append(phrases, beatTimes[i])
	}

	segments := classifySegments(phrases, energy, beatTimes, duration, gridSize)
	highlights := detectHighlights(beatTimes, energy)

	ta := &TrackAnalysis{
		Filepath:   path,
		Hash:       hash,
		Duration:   math.Round(duration*100) / 100,
		BPM:        bpm,
		LoudnessDB: math.Round(loudness*10) / 10,
		Key:        key,
		BeatTimes:  beatTimes,
		Phrases:    phrases,
		Segments:   segments,
		Energy:     energy,
		Highlights: highlights,
	}

	saveCachedAnalysis(cachePath, ta)
	log.Printf("[done] %s (%.1fs, %.0f BPM, %s)", path, duration, bpm, key)
	return ta, nil
}

// AnalyzeBatch analyzes multiple tracks in parallel
func AnalyzeBatch(paths []string, cacheDir string) ([]TrackAnalysis, []string) {
	results := make([]TrackAnalysis, len(paths))
	errors := make([]string, len(paths))
	var wg sync.WaitGroup

	// Limit concurrency to NumCPU
	sem := make(chan struct{}, 4)

	for i, p := range paths {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ta, err := AnalyzeTrack(path, cacheDir)
			if err != nil {
				errors[idx] = fmt.Sprintf("%s: %v", path, err)
				return
			}
			results[idx] = *ta
		}(i, p)
	}
	wg.Wait()

	// Compact errors
	var errs []string
	for _, e := range errors {
		if e != "" {
			errs = append(errs, e)
		}
	}
	return results, errs
}
