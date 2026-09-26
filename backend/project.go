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
	"time"
)

var safeProjectName = regexp.MustCompile(`[^a-zA-Z0-9._\-가-힣]+`)

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
		http.Error(w, err.Error(), 400)
		return
	}
	if p.SchemaVersion == 0 {
		p.SchemaVersion = 1
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
		http.Error(w, err.Error(), 500)
		return
	}
	path := projectPath(p.Name)
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, data, 0644); err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"path": path, "project": p})
}
func handleProjectLoad(w http.ResponseWriter, r *http.Request) {
	path := projectPath(r.URL.Query().Get("name"))
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "project not found", 404)
		return
	}
	var p MixProject
	if json.Unmarshal(data, &p) != nil || p.SchemaVersion < 1 {
		http.Error(w, "invalid project", 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
func handleProjectList(w http.ResponseWriter, r *http.Request) {
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
