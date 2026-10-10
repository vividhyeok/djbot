package main

import (
	"math"
	"testing"
)

func TestTimelineKeepsSourceCuesAndOutputOverlap(t *testing.T) {
	tracks := []TrackEntry{{Duration: 100}, {Duration: 100}, {Duration: 100}}
	tr := []TransitionSpec{{AOutTime: 80, BInTime: 5, Duration: 4, SpeedA: 1.04, SpeedB: .96}, {AOutTime: 75, BInTime: 8, Duration: 3, SpeedA: .98, SpeedB: 1.02}}
	clips, err := BuildRenderTimeline(tracks, tr)
	if err != nil {
		t.Fatal(err)
	}
	for i, x := range tr {
		if math.Abs(clips[i].Start+clips[i].Duration-clips[i+1].Start-x.Duration) > 1e-9 {
			t.Fatal("overlap changed")
		}
	}
	if math.Abs(clips[1].Regions[0].SourceEnd-8.84) > 1e-9 || clips[1].Regions[1].SourceEnd != 75 {
		t.Fatal("source cues moved")
	}
}

func TestTimelineRejectsConflictingMiddleTrackAndMissingTransitions(t *testing.T) {
	tracks := []TrackEntry{{Duration: 20}, {Duration: 20}, {Duration: 20}}
	tr := []TransitionSpec{{AOutTime: 15, BInTime: 10, Duration: 4}, {AOutTime: 12, BInTime: 0, Duration: 4}}
	if _, err := BuildRenderTimeline(tracks, tr); err == nil {
		t.Fatal("overlapping middle transitions accepted")
	}
	if _, err := BuildRenderTimeline(tracks, nil); err == nil {
		t.Fatal("missing transitions accepted")
	}
}
