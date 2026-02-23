"""
YouTube Playlist Downloader — yt-dlp Python API for DJ Bot.

Accepts YouTube or YouTube Music playlist URLs,
downloads tracks as MP3 (192kbps) to cache/youtube/.
Uses yt-dlp as a Python module (not CLI) to ensure latest version.
"""

import os
import re
import logging
from pathlib import Path

logger = logging.getLogger("YTDownloader")

DOWNLOAD_DIR = Path("cache/youtube")

def sanitize_filename(name: str) -> str:
    """Remove problematic characters from filename."""
    name = re.sub(r'[<>:"/\\|?*]', '', name)
    name = name.strip('. ')
    if len(name) > 150:
        name = name[:150]
    return name

def _normalize_url(url: str) -> str:
    """Convert music.youtube.com URLs to www.youtube.com for better compatibility."""
    url = url.replace("music.youtube.com", "www.youtube.com")
    if '&si=' in url:
        url = url.split('&si=')[0]
    return url

def _find_ffmpeg():
    """Find ffmpeg via imageio_ffmpeg."""
    try:
        import imageio_ffmpeg
        return imageio_ffmpeg.get_ffmpeg_exe()
    except:
        return None

def get_playlist_info(url: str) -> list:
    """
    Fetch playlist metadata without downloading.
    Returns list of dicts with 'title', 'id', 'duration'.
    """
    import yt_dlp
    
    url = _normalize_url(url)
    
    ydl_opts = {
        'extract_flat': True,
        'quiet': True,
        'no_warnings': True,
        'ignoreerrors': True,
    }
    
    try:
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(url, download=False)
            if not info:
                return []
            
            entries = info.get('entries', [])
            results = []
            for entry in entries:
                if entry is None:
                    continue
                results.append({
                    'title': entry.get('title', 'Unknown'),
                    'id': entry.get('id', ''),
                    'duration': entry.get('duration', 0),
                })
            
            logger.info(f"Playlist info: found {len(results)} tracks")
            return results
    except Exception as e:
        logger.error(f"Error fetching playlist info: {e}")
        return []

def download_playlist_batch(url: str, progress_callback=None) -> list:
    """
    Download all tracks from a YouTube/YouTube Music playlist using yt-dlp Python API.
    
    Downloads the entire playlist in one pass (more reliable than individual downloads).
    
    Args:
        url: YouTube playlist URL
        progress_callback: Optional callable(status_text) for progress updates
    
    Returns:
        List of dicts with 'title', 'filepath', 'filename'
    """
    import yt_dlp
    
    url = _normalize_url(url)
    DOWNLOAD_DIR.mkdir(parents=True, exist_ok=True)
    
    ffmpeg_path = _find_ffmpeg()
    
    # Track download progress
    downloaded_files = []
    current_title = [""]
    
    def progress_hook(d):
        if d['status'] == 'downloading':
            filename = d.get('filename', '')
            if progress_callback and filename:
                pct = d.get('_percent_str', '?%')
                progress_callback(f"⬇️ {current_title[0][:50]}... {pct}")
        elif d['status'] == 'finished':
            filepath = d.get('filename', '')
            if filepath:
                downloaded_files.append({
                    'raw_path': filepath,
                    'title': current_title[0]
                })
    
    def postprocessor_hook(d):
        if d['status'] == 'finished':
            filepath = d.get('info_dict', {}).get('filepath', '')
            if filepath and downloaded_files:
                # Update the last entry with the final post-processed path
                downloaded_files[-1]['final_path'] = filepath
    
    ydl_opts = {
        'format': 'bestaudio/best',
        'outtmpl': str(DOWNLOAD_DIR / '%(title)s.%(ext)s'),
        'ignoreerrors': True,
        'no_warnings': True,
        'quiet': True,
        'progress_hooks': [progress_hook],
        'postprocessor_hooks': [postprocessor_hook],
        'retries': 3,
        'socket_timeout': 30,
    }
    
    if ffmpeg_path:
        ydl_opts['ffmpeg_location'] = ffmpeg_path
    
    try:
        # First get playlist info for titles
        entries = get_playlist_info(url)
        total = len(entries)
        
        if progress_callback:
            progress_callback(f"🎵 {total}곡 발견! 다운로드 시작...")
        
        # Download each track individually via Python API (better error handling)
        results = []
        for i, entry in enumerate(entries):
            video_id = entry['id']
            title = entry['title']
            current_title[0] = title
            safe_title = sanitize_filename(title)
            
            # Check cache first (any audio extension)
            cached = None
            for ext in ['.mp3', '.webm', '.m4a', '.opus', '.ogg']:
                candidate = DOWNLOAD_DIR / f"{safe_title}{ext}"
                if candidate.exists() and candidate.stat().st_size > 50000:
                    cached = candidate
                    break
            
            if cached:
                logger.info(f"[cached] {safe_title}")
                results.append({
                    'title': title,
                    'filepath': str(cached),
                    'filename': cached.name
                })
                if progress_callback:
                    progress_callback(f"✅ [{i+1}/{total}] {title[:40]} (캐시)")
                continue
            
            if progress_callback:
                progress_callback(f"⬇️ [{i+1}/{total}] {title[:40]}...")
            
            video_url = f"https://www.youtube.com/watch?v={video_id}"
            
            try:
                downloaded_files.clear()
                with yt_dlp.YoutubeDL(ydl_opts) as ydl:
                    ydl.download([video_url])
                
                # Find the downloaded file
                found_path = None
                # Check by final_path from postprocessor
                if downloaded_files and downloaded_files[-1].get('final_path'):
                    fp = downloaded_files[-1]['final_path']
                    if os.path.exists(fp):
                        found_path = fp
                
                # Fallback: search directory
                if not found_path:
                    for f in DOWNLOAD_DIR.iterdir():
                        if safe_title.lower() in f.stem.lower() and f.suffix == '.mp3':
                            found_path = str(f)
                            break
                
                # Broader fallback: check any new mp3
                if not found_path:
                    for f in sorted(DOWNLOAD_DIR.glob("*.mp3"), key=os.path.getmtime, reverse=True):
                        found_path = str(f)
                        break
                
                if found_path and os.path.exists(found_path):
                    results.append({
                        'title': title,
                        'filepath': found_path,
                        'filename': os.path.basename(found_path)
                    })
                    logger.info(f"[downloaded] {title}")
                    if progress_callback:
                        progress_callback(f"✅ [{i+1}/{total}] {title[:40]}")
                else:
                    logger.warning(f"[failed] {title} - file not found after download")
                    if progress_callback:
                        progress_callback(f"⚠️ [{i+1}/{total}] {title[:40]} 실패")
                        
            except Exception as e:
                logger.warning(f"[failed] {title}: {e}")
                if progress_callback:
                    progress_callback(f"⚠️ [{i+1}/{total}] {title[:40]} 오류")
        
        logger.info(f"Downloaded {len(results)}/{total} tracks successfully")
        return results
        
    except Exception as e:
        logger.error(f"Playlist download error: {e}")
        return []
