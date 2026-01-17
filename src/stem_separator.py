import subprocess
import os
from pathlib import Path
from src.utils import logger, get_file_hash, CACHE_DIR, STEMS_DIR

class StemSeparator:
    def __init__(self):
        self.model = "htdemucs" # Fast and good quality

    def separate_track(self, filepath: str) -> dict:
        """
        Separates track into 4 stems.
        Returns dictionary of paths: {'vocals': path, 'drums': path, ...}
        """
        file_hash = get_file_hash(filepath)
        # Demucs output structure: <out_dir>/<model>/<track_name>/<stem>.wav
        # We process single file, so we expect:
        # cache/stems/htdemucs/<filename_no_ext>/{vocals,no_vocals...}.wav
        
        filename_no_ext = Path(filepath).stem
        output_dir = STEMS_DIR
        target_dir = output_dir / self.model / filename_no_ext
        
        # Check if already separated
        expected_stems = ["vocals", "drums", "bass", "other"]
        result_paths = {}
        missing = False
        
        if target_dir.exists():
            for stem in expected_stems:
                stem_path = target_dir / f"{stem}.wav"
                if stem_path.exists():
                    result_paths[stem] = str(stem_path)
                else:
                    missing = True
                    break
        else:
            missing = True

        if not missing:
            logger.info(f"Using cached stems for {filepath}")
            return result_paths

        logger.info(f"Separating stems for {filepath} using Demucs...")
        
        # Construct command
        # python -m demucs.separate -n htdemucs --out cache/stems <filepath>
        cmd = [
            "python", "-m", "demucs.separate",
            "-n", self.model,
            "--out", str(output_dir),
            filepath
        ]
        
        try:
            # Run without capture_output first to see progress if needed, 
            # but for GUI app usually capture is better. 
            # We'll expect it takes time.
            process = subprocess.run(cmd, check=True, capture_output=True, text=True)
            logger.info("Demucs separation complete.")
        except subprocess.CalledProcessError as e:
            logger.error(f"Demucs failed: {e.stderr}")
            raise RuntimeError(f"Stem separation failed: {e.stderr}")

        # specific directory might be strictly the filename or slightly normalized by demucs
        # Demucs usually uses the filename without extension
        
        for stem in expected_stems:
            stem_path = target_dir / f"{stem}.wav"
            if stem_path.exists():
                result_paths[stem] = str(stem_path)
            else:
                 # Fallback check (sometimes demucs normalizes spaces to underscores)
                 pass
        
        return result_paths

if __name__ == "__main__":
    pass
