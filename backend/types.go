package main

// TrackAnalysis matches the Python analyzer_engine.py output format exactly.
type TrackAnalysis struct {
	Filepath   string      `json:"filepath"`
	Hash       string      `json:"hash"`
	Duration   float64     `json:"duration"`
	BPM        float64     `json:"bpm"`
	LoudnessDB float64     `json:"loudness_db"`
	Key        string      `json:"key"`
	BeatTimes  []float64   `json:"beat_times"`
	Phrases    []float64   `json:"phrases"`
	Segments   []Segment   `json:"segments"`
	Energy     []float64   `json:"energy"`
	Highlights []Highlight `json:"highlights"`
}

type Segment struct {
	Time        float64 `json:"time"`
	Label       string  `json:"label"`
	Energy      float64 `json:"energy"`
	VocalEnergy float64 `json:"vocal_energy"`
}

type Highlight struct {
	StartBeatIdx int     `json:"start_beat_idx"`
	EndBeatIdx   int     `json:"end_beat_idx"`
	StartTime    float64 `json:"start_time"`
	EndTime      float64 `json:"end_time"`
	Score        float64 `json:"score"`
}

// --- API Request/Response types ---

type AnalyzeRequest struct {
	Filepaths []string `json:"filepaths"`
}

type AnalyzeResponse struct {
	Results []TrackAnalysis `json:"results"`
	Errors  []string        `json:"errors,omitempty"`
}

type RenderPreviewRequest struct {
	TrackAPath string         `json:"track_a_path"`
	TrackBPath string         `json:"track_b_path"`
	Spec       TransitionSpec `json:"spec"`
}

type TransitionSpec struct {
	Type       string  `json:"type"`
	Name       string  `json:"name"`
	Duration   float64 `json:"duration"`
	AOutTime   float64 `json:"a_out_time"`
	BInTime    float64 `json:"b_in_time"`
	SpeedA     float64 `json:"speed_a"`
	SpeedB     float64 `json:"speed_b"`
	PitchStepB float64 `json:"pitch_step_b"`
	FilterType string  `json:"filter_type"`
}

type RenderPreviewResponse struct {
	OutputPath string `json:"output_path"`
	Error      string `json:"error,omitempty"`
}

type RenderMixRequest struct {
	Playlist    []TrackEntry     `json:"playlist"`
	Transitions []TransitionSpec `json:"transitions"`
	OutputPath  string           `json:"output_path"`
}

type TrackEntry struct {
	Filepath   string  `json:"filepath"`
	Filename   string  `json:"filename"`
	Duration   float64 `json:"duration"`
	BPM        float64 `json:"bpm"`
	LoudnessDB float64 `json:"loudness_db"`
	PlayStart  float64 `json:"play_start"`
	PlayEnd    float64 `json:"play_end"`
}

type RenderMixResponse struct {
	MP3Path string `json:"mp3_path"`
	LRCPath string `json:"lrc_path"`
	Error   string `json:"error,omitempty"`
}

// --- Planner types ---

type TrackWithAnalysis struct {
	Filename string
	Analysis TrackAnalysis
}

type MixPlan struct {
	SortedTracks []TrackAnalysis    `json:"sorted_tracks"`
	Candidates   [][]TransitionSpec `json:"candidates"`
	Selections   []TransitionSpec   `json:"selections"`
}

type PlanRequest struct {
	Tracks      []TrackAnalysis    `json:"tracks"`
	TypeWeights map[string]float64 `json:"type_weights,omitempty"`
	BarWeights  map[int]float64    `json:"bar_weights,omitempty"`
	Scenarios   int                `json:"scenarios,omitempty"`
}

type PlanResponse struct {
	Plan  MixPlan `json:"plan"`
	Error string  `json:"error,omitempty"`
}
