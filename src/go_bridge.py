"""
Go Worker Bridge — connects Python to the Go analysis/render sidecar.

Usage:
    from src.go_bridge import GoWorker
    worker = GoWorker()       # builds + starts Go binary automatically
    result = worker.analyze_track("path/to/file.mp3")
    worker.stop()
"""

import subprocess
import requests
import json
import os
import sys
import time
import threading
import logging
from pathlib import Path

logger = logging.getLogger("GoWorker")

class GoWorker:
    def __init__(self, auto_start=True):
        self.process = None
        self.port = None
        self.base_url = None
        self._available = False
        self._lock = threading.Lock()
        
        if auto_start:
            self._ensure_binary()
            self._start()
    
    @property
    def available(self):
        return self._available
    
    def _get_project_dir(self):
        """Get the djbot project root directory."""
        return Path(__file__).parent.parent.resolve()
    
    def _get_binary_path(self):
        project = self._get_project_dir()
        if sys.platform == 'win32':
            return project / "goworker" / "goworker.exe"
        return project / "goworker" / "goworker"
    
    def _ensure_binary(self):
        """Build the Go binary if it doesn't exist."""
        binary = self._get_binary_path()
        if binary.exists():
            logger.info(f"Go binary found: {binary}")
            return True
        
        logger.info("Building Go worker binary...")
        goworker_dir = self._get_project_dir() / "goworker"
        
        try:
            result = subprocess.run(
                ["go", "build", "-o", str(binary), "."],
                cwd=str(goworker_dir),
                capture_output=True, text=True, timeout=120
            )
            if result.returncode != 0:
                logger.error(f"Go build failed: {result.stderr}")
                return False
            logger.info(f"Go binary built: {binary}")
            return True
        except FileNotFoundError:
            logger.warning("Go not installed — falling back to Python-only mode")
            return False
        except Exception as e:
            logger.warning(f"Go build error: {e}")
            return False

    def _start(self):
        """Start the Go worker process."""
        binary = self._get_binary_path()
        if not binary.exists():
            logger.warning("Go binary not found, worker unavailable")
            return
        
        # Find ffmpeg path
        ffmpeg_path = self._find_ffmpeg()
        
        cmd = [str(binary)]
        if ffmpeg_path:
            cmd.extend(["--ffmpeg", ffmpeg_path])
        
        try:
            self.process = subprocess.Popen(
                cmd,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                cwd=str(self._get_project_dir()),
            )
            
            # Read the port from stdout
            line = self.process.stdout.readline().decode().strip()
            if line.startswith("PORT:"):
                self.port = int(line.split(":")[1])
                self.base_url = f"http://127.0.0.1:{self.port}"
                self._available = True
                logger.info(f"Go worker started on port {self.port}")
                
                # Start stderr reader thread
                t = threading.Thread(target=self._read_stderr, daemon=True)
                t.start()
            else:
                logger.error(f"Unexpected output from Go worker: {line}")
                self.stop()
        except Exception as e:
            logger.warning(f"Failed to start Go worker: {e}")
            self._available = False

    def _read_stderr(self):
        """Read Go worker stderr in background for logging."""
        try:
            for line in self.process.stderr:
                logger.debug(f"[go] {line.decode().strip()}")
        except:
            pass

    def _find_ffmpeg(self):
        """Try to find ffmpeg path."""
        try:
            import imageio_ffmpeg
            return imageio_ffmpeg.get_ffmpeg_exe()
        except ImportError:
            pass
        # Check PATH
        import shutil
        path = shutil.which("ffmpeg")
        return path

    def stop(self):
        """Stop the Go worker."""
        if self.process:
            self.process.terminate()
            try:
                self.process.wait(timeout=5)
            except:
                self.process.kill()
            self.process = None
            self._available = False
            logger.info("Go worker stopped")

    def analyze_track(self, filepath):
        """Analyze a single track via Go worker. Returns dict or None on failure."""
        return self.analyze_batch([filepath])[0] if self._available else None

    def analyze_batch(self, filepaths):
        """Analyze multiple tracks in parallel. Returns list of dicts."""
        if not self._available:
            return [None] * len(filepaths)
        
        try:
            resp = requests.post(
                f"{self.base_url}/analyze",
                json={"filepaths": filepaths},
                timeout=600  # 10 min for large batches
            )
            data = resp.json()
            results = data.get("results", [])
            errors = data.get("errors", [])
            if errors:
                for e in errors:
                    logger.warning(f"Go analysis error: {e}")
            
            # Filter out empty results (zero-value structs)
            cleaned = []
            for r in results:
                if r.get("filepath") and r.get("duration", 0) > 0:
                    cleaned.append(r)
                else:
                    cleaned.append(None)
            return cleaned
        except Exception as e:
            logger.error(f"Go worker analyze failed: {e}")
            return [None] * len(filepaths)

    def render_preview(self, track_a_path, track_b_path, spec):
        """Render a transition preview. Returns output path or None."""
        if not self._available:
            return None
        try:
            resp = requests.post(
                f"{self.base_url}/render/preview",
                json={
                    "track_a_path": track_a_path,
                    "track_b_path": track_b_path,
                    "spec": spec
                },
                timeout=120
            )
            data = resp.json()
            if data.get("error"):
                logger.error(f"Preview render error: {data['error']}")
                return None
            return data.get("output_path")
        except Exception as e:
            logger.error(f"Go worker preview failed: {e}")
            return None

    def render_mix(self, playlist, transitions, output_path):
        """Render final mix. Returns (mp3_path, lrc_path) or None."""
        if not self._available:
            return None
        try:
            resp = requests.post(
                f"{self.base_url}/render/mix",
                json={
                    "playlist": playlist,
                    "transitions": transitions,
                    "output_path": output_path
                },
                timeout=1800  # 30 min for long mixes
            )
            data = resp.json()
            if data.get("error"):
                logger.error(f"Mix render error: {data['error']}")
                return None
            return data.get("mp3_path"), data.get("lrc_path")
        except Exception as e:
            logger.error(f"Go worker mix failed: {e}")
            return None

    def __del__(self):
        self.stop()


# Singleton instance
_worker = None

def get_worker():
    """Get or create the singleton Go worker."""
    global _worker
    if _worker is None:
        _worker = GoWorker(auto_start=True)
    return _worker
