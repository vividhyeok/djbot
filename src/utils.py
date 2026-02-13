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
    """
    Calculates a stable file hash for caching.

    Uses file content (head/tail chunks + size) instead of path/mtime so the
    same audio uploaded again can reuse analysis cache even when copied to a
    new temp path.
    """
    filepath = Path(filepath)
    stat = filepath.stat()
    file_size = stat.st_size
    chunk_size = 1024 * 1024  # 1MB

    md5 = hashlib.md5()
    md5.update(str(file_size).encode("utf-8"))

    with open(filepath, "rb") as f:
        head = f.read(chunk_size)
        md5.update(head)

        if file_size > chunk_size:
            f.seek(max(0, file_size - chunk_size))
            tail = f.read(chunk_size)
            md5.update(tail)

    return md5.hexdigest()

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
