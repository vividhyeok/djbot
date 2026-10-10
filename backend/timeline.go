package main

import "fmt"

// Timeline seconds are output seconds; cue positions are source seconds.
// Transition duration is never silently extended or shortened by the renderer.
type RenderRegion struct {
	SourceStart float64 `json:"source_start"`
	SourceEnd   float64 `json:"source_end"`
	Speed       float64 `json:"speed"`
	Duration    float64 `json:"duration"`
}

type RenderClip struct {
	Start    float64        `json:"start"`
	Duration float64        `json:"duration"`
	Regions  []RenderRegion `json:"regions"`
}

func playbackSpeed(v float64) float64 {
	if v == 0 {
		return 1
	}
	return v
}

func BuildRenderTimeline(tracks []TrackEntry, transitions []TransitionSpec) ([]RenderClip, error) {
	if len(tracks) < 1 || len(transitions) != len(tracks)-1 {
		return nil, fmt.Errorf("each adjacent pair needs one transition")
	}
	for i, t := range transitions {
		for _, fade := range []float64{t.FadeInSeconds, t.FadeOutSeconds} {
			if !finite(fade) || fade < 0 || fade > t.Duration {
				return nil, fmt.Errorf("transition %d: fade length must be within overlap", i+1)
			}
		}
		if !finite(t.Duration) || t.Duration <= 0 || !finite(t.AOutTime) || !finite(t.BInTime) || t.AOutTime < 0 || t.BInTime < 0 {
			return nil, fmt.Errorf("transition %d: invalid cue or duration", i+1)
		}
		for _, speed := range []float64{playbackSpeed(t.SpeedA), playbackSpeed(t.SpeedB)} {
			if !finite(speed) || speed < .5 || speed > 2 {
				return nil, fmt.Errorf("transition %d: unsupported speed", i+1)
			}
		}
	}
	clips := make([]RenderClip, len(tracks))
	for i, track := range tracks {
		if !finite(track.Duration) || track.Duration <= 0 {
			return nil, fmt.Errorf("track %d: invalid duration", i+1)
		}
		start, end := track.PlayStart, track.PlayEnd
		if end == 0 {
			end = track.Duration
		}
		entryEnd, exitStart := start, end
		if i > 0 {
			start = transitions[i-1].BInTime
			entryEnd = start + transitions[i-1].Duration*playbackSpeed(transitions[i-1].SpeedB)
		}
		if i < len(transitions) {
			exitStart = transitions[i].AOutTime
			end = exitStart + transitions[i].Duration*playbackSpeed(transitions[i].SpeedA)
		}
		if !finite(start) || !finite(end) || start < 0 || end > track.Duration+.001 || end <= start || entryEnd > exitStart+.001 {
			return nil, fmt.Errorf("track %d: transition cues overlap or exceed source; adjust entry, exit or duration", i+1)
		}
		clip := &clips[i]
		add := func(a, b, speed float64) {
			if b > a {
				d := (b - a) / speed
				clip.Regions = append(clip.Regions, RenderRegion{a, b, speed, d})
				clip.Duration += d
			}
		}
		if i > 0 {
			add(start, entryEnd, playbackSpeed(transitions[i-1].SpeedB))
		}
		add(entryEnd, exitStart, 1)
		if i < len(transitions) {
			add(exitStart, end, playbackSpeed(transitions[i].SpeedA))
		}
		if i > 0 {
			clip.Start = clips[i-1].Start + clips[i-1].Duration - transitions[i-1].Duration
		}
	}
	return clips, nil
}
