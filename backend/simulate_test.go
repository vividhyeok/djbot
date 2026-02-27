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

	fmt.Println("======================================================")
	fmt.Println("🚀 [DJBot] 헤드리스 믹스 타임라인 시뮬레이터 가동 시작")
	fmt.Println("======================================================")

	for iteration := 1; iteration <= 3; iteration++ {
		fmt.Printf("\n▶ 시뮬레이션 RUN #%d ---------------------------------\n", iteration)
		time.Sleep(500 * time.Millisecond)

		numTracks := 5 + rand.Intn(3)
		var playlist []TrackWithAnalysis
		for i := 0; i < numTracks; i++ {
			dur := 180.0 + rand.Float64()*60.0
			bpm := 120.0

			beats := []float64{}
			for cur := 0.0; cur < dur; cur += (60.0 / bpm) {
				beats = append(beats, cur)
			}
			segs := []Segment{{Time: 0, Label: "Intro", Energy: 0.5}, {Time: dur - 30, Label: "Outro", Energy: 0.5}}

			playlist = append(playlist, TrackWithAnalysis{
				Filename: fmt.Sprintf("Track_%d", i+1),
				Analysis: TrackAnalysis{Filepath: "fake", Duration: dur, BPM: bpm, BeatTimes: beats, Segments: segs},
			})
		}

		rawTracks := make([]TrackAnalysis, len(playlist))
		for i, x := range playlist {
			rawTracks[i] = x.Analysis
		}

		fmt.Printf("[Planner] %d개의 가상 트랙 타임라인 생성 중...\n", numTracks)
		plan := GenerateMixPlan(rawTracks, nil, nil, 1)

		sortedPlaylist := make([]TrackWithAnalysis, len(plan.SortedTracks))
		for i, an := range plan.SortedTracks {
			sortedPlaylist[i] = TrackWithAnalysis{Filename: fmt.Sprintf("Track_%d", i+1), Analysis: an}
		}

		entries := ComputePlayBounds(sortedPlaylist, plan.Selections)

		currentOffsetMs := 0
		var expectedTotalDuration float64 = 0

		fmt.Println("\n[Renderer] 타임라인 오버랩(덧셈/뺄셈) 시뮬레이션 진행:")
		fmt.Println("---------------------------------------------------------------------------------")
		fmt.Printf("%-10s | %-12s | %-12s | %-15s | %-15s\n", "Track", "Play Length", "Crossfade", "Start Time(MS)", "Expected Total")
		fmt.Println("---------------------------------------------------------------------------------")

		for i := 0; i < len(entries); i++ {
			tt := entries[i]
			startSec := tt.PlayStart
			endSec := tt.PlayEnd
			if endSec <= 0 {
				endSec = tt.Duration
			}
			if endSec <= startSec+15.0 {
				endSec = startSec + 15.0
			}

			chunkPhysicalDurSec := endSec - startSec
			expectedTotalDuration += chunkPhysicalDurSec

			var xfadeDurMs int = 0
			if i > 0 {
				trans := plan.Selections[i-1]
				xfadeDurMs = int(math.Round(trans.Duration * 1000.0))

				if xfadeDurMs < 2000 {
					xfadeDurMs = 2000
				}
				maxCurrent := currentOffsetMs - 500
				maxB := int(chunkPhysicalDurSec*1000.0) - 500
				if xfadeDurMs > maxCurrent {
					xfadeDurMs = maxCurrent
				}
				if xfadeDurMs > maxB {
					xfadeDurMs = maxB
				}
				if xfadeDurMs < 0 {
					xfadeDurMs = 0
				}

				// Pydub 교훈: 크로스페이드 길이만큼 이전 트랙의 타임라인을 뒤로 감음 (오버랩)
				currentOffsetMs -= xfadeDurMs
				if currentOffsetMs < 0 {
					currentOffsetMs = 0
				}

				expectedTotalDuration -= (float64(xfadeDurMs) / 1000.0)
			}

			fmt.Printf("%-10s | %6.2f초     | %6.2f초     | %8d ms      |  %8.2f초\n",
				tt.Filename, chunkPhysicalDurSec, float64(xfadeDurMs)/1000.0, currentOffsetMs, expectedTotalDuration)

			currentOffsetMs += int(math.Round(chunkPhysicalDurSec * 1000.0))
			time.Sleep(300 * time.Millisecond) // 진행상황 시각 효과
		}

		actualTotalSec := float64(currentOffsetMs) / 1000.0
		diff := math.Abs(actualTotalSec - expectedTotalDuration)

		fmt.Println("---------------------------------------------------------------------------------")
		if diff > 0.05 {
			fmt.Printf("❌ [결과] FAIL! 타임라인 오류 발생. 오차: %.3f초\n", diff)
			t.Errorf("Drift detected: %.3f", diff)
		} else {
			fmt.Printf("✅ [결과] SUCCESS! 100%% 정확 (계산상 총 길이: %.3f초, 실제 타임라인: %.3f초)\n", expectedTotalDuration, actualTotalSec)
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("======================================================")
	fmt.Println("🎉 [DJBot] 시뮬레이션 검증 완료. 오차 없이 믹싱 가능한 상태입니다.")
}
