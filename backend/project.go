package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var safeProjectName = regexp.MustCompile(`[^\p{L}\p{N}._ -]+`)
var projectWriteMu sync.Mutex

func projectPath(name string) string {
	name = strings.TrimSuffix(filepath.Base(name), ".djbot")
	name = safeProjectName.ReplaceAllString(name, "_")
	if name == "" {
		name = "Untitled"
	}
	return filepath.Join(projectsDir, name+".djbot")
}

func handleProjectSave(w http.ResponseWriter, r *http.Request) {
	var p MixProject
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if p.SchemaVersion == 0 {
		p.SchemaVersion = 2
	}
	if p.SchemaVersion < 1 || p.SchemaVersion > 2 {
		http.Error(w, "unsupported project version", http.StatusBadRequest)
		return
	}
	if p.ID == "" {
		p.ID = randHex(8)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if p.CreatedAt == "" {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	path := projectPath(p.Name)
	err = writeProjectFile(path, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"path": path, "project": p})
}

func handleProjectLoad(w http.ResponseWriter, r *http.Request) {
	path := projectPath(r.URL.Query().Get("name"))
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	var p MixProject
	if json.Unmarshal(data, &p) != nil || p.SchemaVersion < 1 || p.SchemaVersion > 2 {
		http.Error(w, "invalid project", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func writeProjectFile(path string, data []byte) error {
	projectWriteMu.Lock()
	defer projectWriteMu.Unlock()
	f, err := os.CreateTemp(filepath.Dir(path), ".project-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func handleProjectList(w http.ResponseWriter, _ *http.Request) {
	entries, _ := os.ReadDir(projectsDir)
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".djbot") {
			names = append(names, strings.TrimSuffix(e.Name(), ".djbot"))
		}
	}
	sort.Strings(names)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"projects": names}); err != nil {
		fmt.Fprint(w, "{}")
	}
}
