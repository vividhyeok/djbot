package main

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func syntheticAnalysis(name string, bpm, energy float64) TrackAnalysis {
	dur := 180.0
	var beats, downs, phrases []float64
	step := 60 / bpm
	for i, t := 0, 0.0; t < dur; i, t = i+1, t+step {
		beats = append(beats, t)
		if i%4 == 0 {
			downs = append(downs, t)
		}
		if i%32 == 0 {
			phrases = append(phrases, t)
		}
	}
	e := make([]float64, len(beats))
	for i := range e {
		e[i] = energy
	}
	return TrackAnalysis{Version: 2, Filepath: name, Duration: dur, BPM: bpm, BPMConfidence: .9, Key: "C Major", KeyConfidence: .8, LoudnessDB: -14, BeatTimes: beats, Downbeats: downs, Phrases: phrases, Energy: e, Segments: []Segment{{Start: 0, End: dur, Time: 0, Label: "verse", Energy: energy, VocalEnergy: .3}}, Spectral: []SpectralFeature{{Time: 0, BassRatio: .3, MidRatio: .5, HighRatio: .2}}}
}

func TestBPMConfidenceAndBeatAlignment(t *testing.T) {
	sr, hop := 22050, 512
	frames := 900
	onset := make([]float64, frames)
	lag := int(math.Round(60 * float64(sr) / (120 * float64(hop))))
	for i := 10; i < frames; i += lag {
		onset[i] = 10
	}
	bpm := estimateBPM(onset, sr, hop)
	if math.Abs(bpm-120) > 3 {
		t.Fatalf("BPM %.1f, want 120±3", bpm)
	}
	if c := tempoConfidence(onset, sr, hop, bpm); c < .7 {
		t.Fatalf("confidence %.2f too low", c)
	}
	beats := trackBeats(onset, sr, float64(frames*hop)/float64(sr), bpm, hop)
	for _, b := range beats {
		if s := alignmentScore(b, beats); s < 99 {
			t.Fatalf("self alignment %.1f", s)
		}
	}
}

func TestPhraseDownbeatAndSearch(t *testing.T) {
	a := syntheticAnalysis("a.wav", 120, .45)
	b := syntheticAnalysis("b.wav", 124, .65)
	downs := detectDownbeats(a.BeatTimes, a.Energy)
	if len(downs) < 10 {
		t.Fatal("downbeats missing")
	}
	plan := GenerateMixPlanV2([]TrackAnalysis{a, b}, nil, nil, MixSettings{Style: "club", SearchDepth: "deep", PreserveOrder: true})
	if len(plan.Selections) != 1 || len(plan.Alternatives[0]) < 3 {
		t.Fatalf("invalid plan: %#v", plan)
	}
	tr := plan.Selections[0]
	if !tr.DownbeatAligned || tr.Score.Total <= 0 || tr.Bars == 0 {
		t.Fatalf("unscored/unquantized transition: %#v", tr)
	}
	if plan.Evaluated < 50 {
		t.Fatalf("deep search evaluated only %d", plan.Evaluated)
	}
}

func TestProjectSerialization(t *testing.T) {
	a := syntheticAnalysis("a.wav", 120, .5)
	p := MixProject{SchemaVersion: 1, Name: "test", Materials: []AudioMaterial{{ID: "m1", Kind: "track", SourcePath: "a.wav", Analysis: &a}}, Clips: []TimelineClip{{ID: "c1", MaterialID: "m1", SourceEnd: 180, Gain: 1}}, Settings: MixSettings{Style: "smooth"}}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got MixProject
	if err = json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Materials[0].Analysis.BPM != 120 || got.Clips[0].MaterialID != "m1" {
		t.Fatalf("roundtrip mismatch: %#v", got)
	}
}

func writeToneWAV(path string, seconds int) error {
	const sr = 44100
	data := make([]int16, sr*seconds*2)
	for i := 0; i < sr*seconds; i++ {
		v := int16(math.Sin(2*math.Pi*220*float64(i)/sr) * 6000)
		data[i*2] = v
		data[i*2+1] = v
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	f.Write([]byte("RIFF"))
	binary.Write(f, binary.LittleEndian, uint32(36+len(data)*2))
	f.Write([]byte("WAVEfmt "))
	binary.Write(f, binary.LittleEndian, uint32(16))
	binary.Write(f, binary.LittleEndian, uint16(1))
	binary.Write(f, binary.LittleEndian, uint16(2))
	binary.Write(f, binary.LittleEndian, uint32(sr))
	binary.Write(f, binary.LittleEndian, uint32(sr*4))
	binary.Write(f, binary.LittleEndian, uint16(4))
	binary.Write(f, binary.LittleEndian, uint16(16))
	f.Write([]byte("data"))
	binary.Write(f, binary.LittleEndian, uint32(len(data)*2))
	return binary.Write(f, binary.LittleEndian, data)
}

func TestRendererPreviewWithFFmpeg(t *testing.T) {
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffmpegPath = p
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.wav"), filepath.Join(dir, "b.wav")
	if writeToneWAV(a, 12) != nil || writeToneWAV(b, 12) != nil {
		t.Fatal("wav fixture")
	}
	out, err := RenderPreview(a, b, TransitionSpec{Type: "bass_swap", Duration: 2, AOutTime: 8, BInTime: 2, SpeedA: 1, SpeedB: 1}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(out); err != nil || st.Size() < 1000 {
		t.Fatalf("preview missing: %v", err)
	}
}
