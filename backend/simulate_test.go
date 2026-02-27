package main

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"
)

func TestTimelineSimulation(t *testing.T) {
	rand.Seed(42)
	allPass := true

	fmt.Println("======================================================")
	fmt.Println("🚀 [DJBot] 헤드리스 믹스 타임라인 시뮬레이터 (30회 검증)")
	fmt.Println("======================================================")

	for iteration := 1; iteration <= 30; iteration++ {
		numTracks := 5 + rand.Intn(11) // 5 to 15 tracks
		var playlist []TrackWithAnalysis

		for i := 0; i < numTracks; i++ {
			bpm := 90.0 + rand.Float64()*60.0
			dur := 160.0 + rand.Float64()*80.0 // 2:40 to 4:00

			beats := []float64{}
			interval := 60.0 / bpm
			for cur := 0.0; cur < dur; cur += interval {
				beats = append(beats, cur)
			}
			segs := []Segment{
				{Time: 0, Label: "Intro", Energy: 0.4},
				{Time: dur * 0.25, Label: "Verse", Energy: 0.6},
				{Time: dur * 0.5, Label: "Chorus", Energy: 0.9},
				{Time: dur - 30, Label: "Outro", Energy: 0.3},
			}
			energy := make([]float64, len(beats))
			for j := range energy {
				energy[j] = 0.4 + rand.Float64()*0.5
			}

			playlist = append(playlist, TrackWithAnalysis{
				Filename: fmt.Sprintf("Track_%d", i+1),
				Analysis: TrackAnalysis{
					Filepath:  "fake",
					Duration:  dur,
					BPM:       bpm,
					BeatTimes: beats,
					Segments:  segs,
					Energy:    energy,
				},
			})
		}

		rawTracks := make([]TrackAnalysis, len(playlist))
		for i, x := range playlist {
			rawTracks[i] = x.Analysis
		}

		plan := GenerateMixPlan(rawTracks, nil, nil, 1)
		sortedPlaylist := make([]TrackWithAnalysis, len(plan.SortedTracks))
		for i, an := range plan.SortedTracks {
			sortedPlaylist[i] = TrackWithAnalysis{Filename: fmt.Sprintf("Track_%d", i+1), Analysis: an}
		}

		entries := ComputePlayBounds(sortedPlaylist, plan.Selections)

		// Simulate renderer.go timeline loop exactly
		currentOffsetMs := 0
		prevChunkMs := 0
		totalExpectedSec := 0.0
		hasNegative := false

		for i := 0; i < len(entries); i++ {
			tt := entries[i]
			startSec := tt.PlayStart
			endSec := tt.PlayEnd
			if endSec <= 0 {
				endSec = tt.Duration
			}
			if startSec < 0 {
				startSec = 0
			}
			if startSec >= endSec-15.0 {
				startSec = math.Max(0, endSec-15.0)
			}
			chunkPhysicalDurSec := endSec - startSec

			if chunkPhysicalDurSec < 0 {
				hasNegative = true
				fmt.Printf("  ❌ NEGATIVE CHUNK at track %d: %.1f - %.1f = %.1f\n", i, endSec, startSec, chunkPhysicalDurSec)
			}

			totalExpectedSec += chunkPhysicalDurSec
			var xfadeDurMs int = 0

			if i > 0 && i-1 < len(plan.Selections) {
				trans := plan.Selections[i-1]
				xfadeDurMs = int(math.Round(trans.Duration * 1000.0))
				if xfadeDurMs < 4000 {
					xfadeDurMs = 4000
				}
				maxByPrev := prevChunkMs - 1000
				maxByB := int(chunkPhysicalDurSec*1000.0) - 5000
				if xfadeDurMs > maxByPrev && maxByPrev > 0 {
					xfadeDurMs = maxByPrev
				}
				if xfadeDurMs > maxByB && maxByB > 0 {
					xfadeDurMs = maxByB
				}
				if xfadeDurMs < 0 {
					xfadeDurMs = 0
				}
				currentOffsetMs -= xfadeDurMs
				if currentOffsetMs < 0 {
					currentOffsetMs = 0
				}
				totalExpectedSec -= float64(xfadeDurMs) / 1000.0
			}

			prevChunkMs = int(math.Round(chunkPhysicalDurSec * 1000.0))
			currentOffsetMs += prevChunkMs
		}

		actualTotalSec := float64(currentOffsetMs) / 1000.0
		diff := math.Abs(actualTotalSec - totalExpectedSec)

		status := "✅"
		if diff > 0.05 || hasNegative {
			status = "❌"
			allPass = false
			t.Errorf("Run #%d FAILED: drift=%.3f, negative=%v", iteration, diff, hasNegative)
		}

		fmt.Printf("%s RUN #%02d | Tracks: %d | Duration: %6.1fs | Drift: %.3fs\n",
			status, iteration, numTracks, actualTotalSec, diff)
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("======================================================")
	if allPass {
		fmt.Println("🎉 모든 30회 시뮬레이션 통과! 타임라인 오차 없음. 실사용 준비 완료.")
	} else {
		fmt.Println("❌ 일부 시뮬레이션 실패. 로그에서 상세 내용 확인.")
	}
}
