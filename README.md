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

### 📁 Architecture Notes
- The Go backend (`backend/`) handles all heavy lifting: downloading, song analysis, mix planning, and FFmpeg rendering.
- The UI communicates with the backend via HTTP.
- To prevent file lock issues on Windows, be cautious when interrupting the `tauri dev` server. Always use `Ctrl + C` gracefully or clear the `src-tauri/target` cache if "Access Denied" occurs.
