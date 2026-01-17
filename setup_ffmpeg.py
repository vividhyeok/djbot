import os
import sys
import subprocess
import winreg

def get_user_path():
    """Retrieves the current User PATH from the Registry."""
    try:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Environment", 0, winreg.KEY_READ) as key:
            try:
                path_val, _ = winreg.QueryValueEx(key, "Path")
                return path_val
            except FileNotFoundError:
                return ""
    except Exception as e:
        print(f"Error reading registry: {e}")
        return None

def set_user_path_registry(new_path_str):
    """Sets the User PATH in the Registry directly to bypass setx limit."""
    try:
        key = winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Environment", 0, winreg.KEY_ALL_ACCESS)
        winreg.SetValueEx(key, "Path", 0, winreg.REG_EXPAND_SZ, new_path_str)
        winreg.CloseKey(key)
        
        # Broadcast config change (so explorer/new shells rely on it)
        try:
             import ctypes
             HWND_BROADCAST = 0xFFFF
             WM_SETTINGCHANGE = 0x001A
             SMTO_ABORTIFHUNG = 0x0002
             result = ctypes.c_long()
             ctypes.windll.user32.SendMessageTimeoutW(HWND_BROADCAST, WM_SETTINGCHANGE, 0, u"Environment", SMTO_ABORTIFHUNG, 5000, ctypes.byref(result))
        except:
             pass
             
        print("Successfully updated Registry PATH.")
        return True
    except Exception as e:
        print(f"Error writing registry: {e}")
        return False

def main():
    print("--- FFMPEG Auto-Configurer (Registry Mode) ---")
    try:
        import imageio_ffmpeg
    except ImportError:
        print("imageio_ffmpeg not installed. Installing...")
        subprocess.check_call([sys.executable, "-m", "pip", "install", "imageio-ffmpeg"])
        import imageio_ffmpeg

    ffmpeg_exe = imageio_ffmpeg.get_ffmpeg_exe()
    ffmpeg_dir = os.path.dirname(ffmpeg_exe)
    print(f"Detected FFMPEG at: {ffmpeg_dir}")

    # Check User Path
    user_path = get_user_path()
    
    if user_path is None:
        print("Could not read User PATH. Aborting.")
        return

    # Normalization for check
    clean_user_path = [p.strip() for p in user_path.split(';') if p.strip()]
    
    if any(os.path.normpath(ffmpeg_dir).lower() == os.path.normpath(p).lower() for p in clean_user_path):
        print("FFMPEG directory is already in your USER PATH.")
    else:
        print("Adding FFMPEG to USER PATH (Registry)...")
        # Construct new path string
        # Ensure we don't double semicolons
        new_path = user_path
        if not new_path.endswith(';'):
            new_path += ';'
        new_path += ffmpeg_dir
        
        if set_user_path_registry(new_path):
            print("Done! Please RESTART your terminal/IDE to see changes.")
        else:
            print("Failed to write to registry.")

if __name__ == "__main__":
    main()
