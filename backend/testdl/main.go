package main

import (
	"fmt"
	"path/filepath"
)

var uploadsDir = "uploads"

func main() {
	initYtdlp()
	url := "https://music.youtube.com/playlist?list=PLTSrgkoVSiy8Rkr4W11ejmHC6B6chmDzX&si=Sy2GqdpzUeJLvMwC"
	outDir, _ := filepath.Abs("test_uploads")
	fmt.Printf("Downloading to %s...\n", outDir)
	files, err := DownloadYouTubePlaylist(url, outDir, 5)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Success! Downloaded %d files.\n", len(files))
		for _, f := range files {
			fmt.Printf("- %s\n", f.Filename)
		}
	}
}
