import imageio_ffmpeg
import subprocess
import sys

try:
    ffmpeg_path = imageio_ffmpeg.get_ffmpeg_exe()
    print(f"FFmpeg path found: {ffmpeg_path}")
    
    # Try running it
    result = subprocess.run([ffmpeg_path, "-version"], capture_output=True, text=True)
    if result.returncode == 0:
        print("FFmpeg execution successful!")
        sys.exit(0)
    else:
        print("FFmpeg execution failed!")
        print(result.stderr)
        sys.exit(1)
except Exception as e:
    print(f"Error: {e}")
    sys.exit(1)
