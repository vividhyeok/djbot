import os
import hashlib
import json
import logging
import sys
from pathlib import Path

# Setup Logging with UTF-8 encoding for Windows
# Reconfigure stdout to use UTF-8 to avoid UnicodeEncodeError on Windows
if sys.platform == 'win32':
    sys.stdout.reconfigure(encoding='utf-8')
    sys.stderr.reconfigure(encoding='utf-8')

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler("djbot.log", encoding='utf-8'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger("DJBot")

CACHE_DIR = Path("cache")
CACHE_DIR.mkdir(exist_ok=True)
STEMS_DIR = CACHE_DIR / "stems"
STEMS_DIR.mkdir(exist_ok=True)

# FFMPEG is configured via external script setup_ffmpeg.py if needed.


def get_file_hash(filepath: str) -> str:
    """Calculates partial MD5 of a file for efficient caching."""
    filepath = Path(filepath)
    stat = filepath.stat()
    # Hash based on path, size, mtime (fast check)
    identifier = f"{filepath.absolute()}_{stat.st_size}_{stat.st_mtime}"
    return hashlib.md5(identifier.encode()).hexdigest()

def save_json(data, filepath):
    with open(filepath, 'w', encoding='utf-8') as f:
        json.dump(data, f, indent=2, ensure_ascii=False)

def load_json(filepath):
    if not os.path.exists(filepath):
        return None
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return json.load(f)
    except (json.JSONDecodeError, OSError) as e:
        print(f"[WARN] Failed to load JSON from {filepath}: {e}")
        return None

def ensure_dirs():
    Path("output").mkdir(exist_ok=True)
    Path("cache").mkdir(exist_ok=True)
    (Path("cache") / "stems").mkdir(exist_ok=True)
