package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestProjectHTTPOverwriteAndReload(t *testing.T) {
	old := projectsDir
	projectsDir = t.TempDir()
	defer func() { projectsDir = old }()
	p := MixProject{SchemaVersion: 2, Name: "한글 믹스", Transitions: []TransitionSpec{{Duration: 3, Locked: true, Curve: "custom", CurveAmount: .72, GainA: .8}}}
	for _, gain := range []float64{.8, .4} {
		p.Transitions[0].GainA = gain
		data, _ := json.Marshal(p)
		w := httptest.NewRecorder()
		handleProjectSave(w, httptest.NewRequest("POST", "/project/save", bytes.NewReader(data)))
		if w.Code != 200 {
			t.Fatalf("save failed: %s", w.Body.String())
		}
		var response struct {
			Project MixProject `json:"project"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		p = response.Project
	}
	w := httptest.NewRecorder()
	handleProjectLoad(w, httptest.NewRequest("GET", "/project/load?name=%ED%95%9C%EA%B8%80%20%EB%AF%B9%EC%8A%A4", nil))
	var restored MixProject
	if err := json.Unmarshal(w.Body.Bytes(), &restored); err != nil {
		t.Fatal(err)
	}
	if restored.ID != p.ID || restored.Transitions[0].GainA != .4 || !restored.Transitions[0].Locked {
		t.Fatal("project state lost")
	}
}

func TestConcurrentProjectWritesPublishCompleteDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mix.djbot")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			data, _ := json.Marshal(map[string]int{"writer": i})
			if err := writeProjectFile(path, data); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]int
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal("partial document published", err)
	}
}
