package main

const AnalysisVersion = 2

type TrackAnalysis struct {
	Version            int               `json:"version"`
	Filepath           string            `json:"filepath"`
	Hash               string            `json:"hash"`
	Duration           float64           `json:"duration"`
	BPM                float64           `json:"bpm"`
	BPMConfidence      float64           `json:"bpm_confidence"`
	LoudnessDB         float64           `json:"loudness_db"`
	Key                string            `json:"key"`
	KeyConfidence      float64           `json:"key_confidence"`
	BeatTimes          []float64         `json:"beat_times"`
	Downbeats          []float64         `json:"downbeats"`
	Phrases            []float64         `json:"phrases"`
	Segments           []Segment         `json:"segments"`
	Energy             []float64         `json:"energy"`
	Onset              []float64         `json:"onset,omitempty"`
	Spectral           []SpectralFeature `json:"spectral,omitempty"`
	Waveform           []float64         `json:"waveform,omitempty"`
	Highlights         []Highlight       `json:"highlights"`
	DownbeatConfidence float64           `json:"downbeat_confidence,omitempty"`
	PhraseConfidence   float64           `json:"phrase_confidence,omitempty"`
	SectionConfidence  float64           `json:"section_confidence,omitempty"`
	VocalConfidence    float64           `json:"vocal_confidence,omitempty"`
}
type SpectralFeature struct {
	Time      float64 `json:"time"`
	Centroid  float64 `json:"centroid"`
	BassRatio float64 `json:"bass_ratio"`
	MidRatio  float64 `json:"mid_ratio"`
	HighRatio float64 `json:"high_ratio"`
	Flux      float64 `json:"flux"`
}
type Segment struct {
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Time        float64 `json:"time"`
	Label       string  `json:"label"`
	Energy      float64 `json:"energy"`
	VocalEnergy float64 `json:"vocal_energy"`
	Confidence  float64 `json:"confidence"`
}
type Highlight struct {
	StartBeatIdx int     `json:"start_beat_idx"`
	EndBeatIdx   int     `json:"end_beat_idx"`
	StartTime    float64 `json:"start_time"`
	EndTime      float64 `json:"end_time"`
	Score        float64 `json:"score"`
}
type AutomationPoint struct {
	Time  float64 `json:"time"`
	Value float64 `json:"value"`
}
type Automation struct {
	VolumeA []AutomationPoint `json:"volume_a,omitempty"`
	VolumeB []AutomationPoint `json:"volume_b,omitempty"`
	LowA    []AutomationPoint `json:"low_a,omitempty"`
	LowB    []AutomationPoint `json:"low_b,omitempty"`
	MidA    []AutomationPoint `json:"mid_a,omitempty"`
	MidB    []AutomationPoint `json:"mid_b,omitempty"`
	HighA   []AutomationPoint `json:"high_a,omitempty"`
	HighB   []AutomationPoint `json:"high_b,omitempty"`
	FilterA []AutomationPoint `json:"filter_a,omitempty"`
	FilterB []AutomationPoint `json:"filter_b,omitempty"`
	Tempo   []AutomationPoint `json:"tempo,omitempty"`
}
type ScoreBreakdown struct {
	Total      float64 `json:"total"`
	Beat       float64 `json:"beat"`
	Downbeat   float64 `json:"downbeat"`
	Phrase     float64 `json:"phrase"`
	Harmony    float64 `json:"harmony"`
	BPM        float64 `json:"bpm"`
	Tempo      float64 `json:"tempo"`
	Energy     float64 `json:"energy"`
	Loudness   float64 `json:"loudness"`
	Spectral   float64 `json:"spectral"`
	Vocal      float64 `json:"vocal"`
	Smoothness float64 `json:"smoothness"`
	Diversity  float64 `json:"diversity,omitempty"`
	SetArc     float64 `json:"set_arc,omitempty"`
}
type TransitionSpec struct {
	ID              string         `json:"id,omitempty"`
	Type            string         `json:"type"`
	Name            string         `json:"name"`
	Duration        float64        `json:"duration"`
	Bars            int            `json:"bars"`
	AOutTime        float64        `json:"a_out_time"`
	BInTime         float64        `json:"b_in_time"`
	TargetBPM       float64        `json:"target_bpm"`
	SpeedA          float64        `json:"speed_a"`
	SpeedB          float64        `json:"speed_b"`
	PitchStepB      float64        `json:"pitch_step_b"`
	FilterType      string         `json:"filter_type,omitempty"`
	Curve           string         `json:"curve,omitempty"`
	CurveAmount     float64        `json:"curve_amount,omitempty"`
	GainA           float64        `json:"gain_a,omitempty"`
	GainB           float64        `json:"gain_b,omitempty"`
	FilterCutoff    float64        `json:"filter_cutoff,omitempty"`
	FilterResonance float64        `json:"filter_resonance,omitempty"`
	EchoFeedback    float64        `json:"echo_feedback,omitempty"`
	EchoMix         float64        `json:"echo_mix,omitempty"`
	Locked          bool           `json:"locked,omitempty"`
	Manual          bool           `json:"manual,omitempty"`
	PhraseAligned   bool           `json:"phrase_aligned"`
	DownbeatAligned bool           `json:"downbeat_aligned"`
	Automation      Automation     `json:"automation"`
	Score           ScoreBreakdown `json:"score"`
}
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
type RenderPreviewResponse struct {
	OutputPath string `json:"output_path"`
	Error      string `json:"error,omitempty"`
}
type RenderMixRequest struct {
	Playlist    []TrackEntry     `json:"playlist"`
	Transitions []TransitionSpec `json:"transitions"`
	OutputPath  string           `json:"output_path"`
	Format      string           `json:"format,omitempty"`
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
	MP3Path string `json:"mp3_path,omitempty"`
	WAVPath string `json:"wav_path,omitempty"`
	LRCPath string `json:"lrc_path,omitempty"`
	Error   string `json:"error,omitempty"`
}
type TrackWithAnalysis struct {
	Filename string
	Analysis TrackAnalysis
}
type MixSettings struct {
	ListeningMode          string    `json:"listening_mode,omitempty"`
	Variation              int       `json:"variation,omitempty"`
	Style                  string    `json:"style"`
	SearchDepth            string    `json:"search_depth"`
	PreserveOrder          bool      `json:"preserve_order"`
	AvoidConsecutiveArtist bool      `json:"avoid_consecutive_artist"`
	FullLength             bool      `json:"full_length"`
	EnergyCurve            []float64 `json:"energy_curve,omitempty"`
	TransitionLength       float64   `json:"transition_length,omitempty"`
	TransitionCharacter    float64   `json:"transition_character,omitempty"`
	HarmonicPriority       float64   `json:"harmonic_priority,omitempty"`
	PhrasePriority         float64   `json:"phrase_priority,omitempty"`
	VocalOverlap           float64   `json:"vocal_overlap,omitempty"`
	TempoFlexibility       float64   `json:"tempo_flexibility,omitempty"`
	EnergyMovement         float64   `json:"energy_movement,omitempty"`
}
type MixPlan struct {
	SortedTracks []TrackAnalysis    `json:"sorted_tracks"`
	Candidates   [][]TransitionSpec `json:"candidates"`
	Selections   []TransitionSpec   `json:"selections"`
	Alternatives [][]TransitionSpec `json:"alternatives"`
	Score        ScoreBreakdown     `json:"score"`
	Evaluated    int                `json:"evaluated"`
	Settings     MixSettings        `json:"settings"`
}
type PlanRequest struct {
	Tracks      []TrackAnalysis    `json:"tracks"`
	TypeWeights map[string]float64 `json:"type_weights,omitempty"`
	BarWeights  map[int]float64    `json:"bar_weights,omitempty"`
	Scenarios   int                `json:"scenarios,omitempty"`
	Settings    MixSettings        `json:"settings"`
	Locked      []TransitionSpec   `json:"locked,omitempty"`
}

type AnalysisOverride struct {
	MaterialID string    `json:"material_id"`
	Downbeats  []float64 `json:"downbeats,omitempty"`
	Phrases    []float64 `json:"phrases,omitempty"`
	Sections   []Segment `json:"sections,omitempty"`
}
type PlanResponse struct {
	Plan  MixPlan `json:"plan"`
	Error string  `json:"error,omitempty"`
}
type AudioMaterial struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	SourcePath string         `json:"source_path"`
	ParentID   string         `json:"parent_id,omitempty"`
	StemRole   string         `json:"stem_role,omitempty"`
	Analysis   *TrackAnalysis `json:"analysis,omitempty"`
}
type TimelineClip struct {
	ID            string  `json:"id"`
	MaterialID    string  `json:"material_id"`
	Lane          int     `json:"lane"`
	TimelineStart float64 `json:"timeline_start"`
	SourceStart   float64 `json:"source_start"`
	SourceEnd     float64 `json:"source_end"`
	Gain          float64 `json:"gain"`
	Muted         bool    `json:"muted"`
}
type MixProject struct {
	SchemaVersion     int                `json:"schema_version"`
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	CreatedAt         string             `json:"created_at"`
	UpdatedAt         string             `json:"updated_at"`
	Materials         []AudioMaterial    `json:"materials"`
	Clips             []TimelineClip     `json:"clips"`
	Transitions       []TransitionSpec   `json:"transitions"`
	Settings          MixSettings        `json:"settings"`
	AnalysisOverrides []AnalysisOverride `json:"analysis_overrides,omitempty"`
	EvaluationMode    bool               `json:"evaluation_mode,omitempty"`
}
