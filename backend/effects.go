package main

import (
	"fmt"
	"math"
	"strings"
)

func automationValue(points []AutomationPoint, time, fallback float64) float64 {
	if len(points) == 0 {
		return fallback
	}
	if time <= points[0].Time {
		return points[0].Value
	}
	for i := 1; i < len(points); i++ {
		if time <= points[i].Time {
			a, b := points[i-1], points[i]
			if b.Time <= a.Time {
				return b.Value
			}
			return a.Value + (b.Value-a.Value)*(time-a.Time)/(b.Time-a.Time)
		}
	}
	return points[len(points)-1].Value
}

// Effects run on the assembled clip clock, not the source clock. Commands
// update at 50 Hz, with short audio frames to keep control latency bounded.
func transitionEffects(t TransitionSpec, incoming bool, start float64, id string) string {
	low, mid, high, filter := t.Automation.LowA, t.Automation.MidA, t.Automation.HighA, t.Automation.FilterA
	if incoming {
		low, mid, high, filter = t.Automation.LowB, t.Automation.MidB, t.Automation.HighB, t.Automation.FilterB
	}
	if len(low) == 0 && t.Type == "bass_swap" {
		low = []AutomationPoint{{0, 1}, {t.Duration * .48, 1}, {t.Duration * .52, 0}, {t.Duration, 0}}
		if incoming {
			low = []AutomationPoint{{0, 0}, {t.Duration * .48, 0}, {t.Duration * .52, 1}, {t.Duration, 1}}
		}
	}
	if len(filter) == 0 && (t.Type == "filter_fade" || t.Type == "drop") {
		filter = []AutomationPoint{{0, 1}, {t.Duration, 0}}
		if incoming {
			filter = []AutomationPoint{{0, 0}, {t.Duration, 1}}
		}
	}
	var nodes []string
	var commands strings.Builder
	for _, band := range []struct {
		name, definition, parameter string
		points                      []AutomationPoint
		isFilter                    bool
	}{
		{"bass@" + id + "low", "f=220:g=0", "gain", low, false},
		{"equalizer@" + id + "mid", "f=1500:t=q:w=1:g=0", "gain", mid, false},
		{"treble@" + id + "high", "f=6000:g=0", "gain", high, false},
		{"lowpass@" + id + "filter", fmt.Sprintf("f=20000:t=q:w=%.6f", clampF(t.FilterResonance, .5, 2)), "frequency", filter, true},
	} {
		if len(band.points) == 0 {
			continue
		}
		nodes = append(nodes, fmt.Sprintf("%s=%s:enable='between(t,%.9f,%.9f)'", band.name, band.definition, start, start+t.Duration))
		steps := int(math.Ceil(t.Duration * 50))
		for i := 0; i <= steps; i++ {
			x := math.Min(float64(i)/50, t.Duration)
			v := automationValue(band.points, x, 1)
			if !finite(v) {
				v = 1
			}
			if band.isFilter {
				cut := t.FilterCutoff
				if cut <= 0 {
					cut = 400
				}
				cut = clampF(cut, 40, 18000)
				v = cut * math.Pow(20000/cut, clampF(v, 0, 1))
			} else {
				v = clampF((v-1)*18, -36, 12)
			}
			fmt.Fprintf(&commands, "%.9f %s %s %.6f;", start+x, band.name, band.parameter, v)
		}
	}
	if len(nodes) == 0 {
		return ""
	}
	return ",asetnsamples=n=441:p=0,asendcmd=c='" + commands.String() + "'," + strings.Join(nodes, ",")
}
