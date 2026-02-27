# DJ Bot - Automix Application 🎧

DJ Bot is a desktop application powered by **Tauri** (Frontend) and **Go** (Backend) that automatically analyzes YouTube playlists, extracts highlights, and intelligently mixes them into a seamless audio track using FFmpeg.

## ✨ Features
- **YouTube Playlist Support**: Automatically downloads songs from any YouTube playlist.
- **Smart Analysis**: Analyzes BPM, key, energy, and musical structure to find the best segments for each track.
- **Auto-Mixing**: Stitches tracks together using a reliable crossfade transition for a natural, seamless listening experience.
- **Multi-Version Mixes**: Keep previous mix variations, generate new ones, and manage different iterations easily.
- **ZIP Export**: Download the finished `.mp3` mix along with a `.lrc` lyrics file (that acts as navigation markers) compressed into a single ZIP file.
- **Cache Management**: Easily clear downloaded raw audio and temporary conversion files.

## 🛠 Tech Stack
- **Frontend**: Tauri v2, Vanilla JS, CSS
- **Backend**: Go (Worker Process)
- **Audio Processing**: FFmpeg
- **Downloader**: yt-dlp

## 🚀 Getting Started

### Prerequisites
- Node.js
- Go (1.20+)
- Rust & Cargo (for Tauri build)
- FFmpeg (Must be installed and added to PATH)
- yt-dlp (Must be installed and added to PATH)

### Installation & Run
1. Install frontend dependencies:
   ```sh
   cd app
   npm install
   ```
2. Run the development server (This will automatically build and start the Go backend worker as a Tauri sidecar):
   ```sh
   npm run tauri dev
   ```

### 📁 Architecture & Engine Updates 
- The Go backend (`backend/`) now fully manages audio independently using raw **PCM Float32 Array Compositing**, guaranteeing absolute exact-millisecond crossfade accuracy without relying on Pydub, Python, or complex FFmpeg `filter_complex` graphs.
- **Mix Planner (`planner.go`)**: Slices tracks based on energy and highlight analysis, preparing precise `PlayStart` and `PlayEnd` sequences.
- **Renderer (`renderer.go`)**: Synthesizes the master mix timeline by calculating track overlaps sequentially, rendering individual track bytes, and overlaying their PCM Float buffers sequentially in memory before the final single-pass MP3 encode.
- **Headless Testing (`simulate_test.go`)**: Includes an advanced backend unit test suite simulating thousands of track timings instantly, guaranteeing the structural crossfade bounds mathematically drift `0.00ms` before GUI execution.
- The UI communicates with the backend via HTTP, receiving the calculated durations, timelines, and real-time build status payloads.
