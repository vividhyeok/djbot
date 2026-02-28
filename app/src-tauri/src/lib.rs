use std::sync::{Arc, Mutex};
use std::process::{Command, Stdio};
use std::io::{BufRead, BufReader};
use tauri::{Manager, State};

struct WorkerState {
    port: Arc<Mutex<Option<u16>>>,
}

#[tauri::command]
fn get_worker_port(state: State<WorkerState>) -> Result<u16, String> {
    let lock = state.port.lock().map_err(|e| e.to_string())?;
    lock.ok_or_else(|| "Worker not ready yet".to_string())
}

#[tauri::command]
fn get_output_dir() -> String {
    // Return absolute path to output directory relative to cwd
    let cwd = std::env::current_dir().unwrap_or_default();
    let out = cwd.join("output");
    std::fs::create_dir_all(&out).ok();
    out.to_string_lossy().to_string()
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let port_state = Arc::new(Mutex::new(None::<u16>));
    let port_state_clone = Arc::clone(&port_state);

    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .manage(WorkerState { port: Arc::clone(&port_state) })
        .invoke_handler(tauri::generate_handler![get_worker_port, get_output_dir])
        .setup(move |app| {
            // Resolve sidecar path
            let resource_path = app
                .path()
                .resource_dir()
                .expect("resource dir");

            // Try sidecar in binaries/ subfolder first, then cwd
            let sidecar_candidates = vec![
                resource_path.join("binaries").join("goworker.exe"),
                resource_path.join("goworker.exe"),
                std::env::current_dir().unwrap_or_default().join("backend").join("goworker.exe"),
            ];

            let mut sidecar_path = None;
            for candidate in &sidecar_candidates {
                if candidate.exists() {
                    sidecar_path = Some(candidate.clone());
                    break;
                }
            }

            let sidecar_path = sidecar_path.unwrap_or_else(|| {
                // Fallback: try PATH
                std::path::PathBuf::from("goworker")
            });

            // Find ffmpeg
            let ffmpeg = find_ffmpeg();

            // Resolve data directory: in dev it should be the project root (sibling of 'app')
            // to avoid triggering tauri dev restarts. In production it's the app data dir.
            let data_dir = if cfg!(debug_assertions) {
                let mut p = std::env::current_dir().unwrap_or_default();
                while p.ends_with("src-tauri") || p.ends_with("app") {
                    p.pop();
                }
                p
            } else {
                app.path().app_data_dir().unwrap_or_else(|_| std::env::current_dir().unwrap_or_default())
            };
            std::fs::create_dir_all(&data_dir).ok();

            let port_arc = Arc::clone(&port_state_clone);
            std::thread::spawn(move || {
                let mut cmd = Command::new(&sidecar_path);
                if let Some(ff) = ffmpeg {
                    cmd.args(["--ffmpeg", &ff]);
                }
                cmd.args(["--data-dir", &data_dir.to_string_lossy()]);
                cmd.stdout(Stdio::piped())
                   .stderr(Stdio::inherit());

                match cmd.spawn() {
                    Ok(mut child) => {
                        if let Some(stdout) = child.stdout.take() {
                            let reader = BufReader::new(stdout);
                            for line in reader.lines().flatten() {
                                if let Some(port_str) = line.strip_prefix("PORT:") {
                                    if let Ok(port) = port_str.trim().parse::<u16>() {
                                        let mut lock = port_arc.lock().unwrap();
                                        *lock = Some(port);
                                        eprintln!("[djbot] Go worker on port {}", port);
                                    }
                                }
                            }
                        }
                    }
                    Err(e) => {
                        eprintln!("[djbot] Failed to start Go worker: {}", e);
                    }
                }
            });

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

fn find_ffmpeg() -> Option<String> {
    // 1. PATH candidates first (most common)
    let path_candidates = vec![
        "ffmpeg".to_string(),
        "ffmpeg.exe".to_string(),
    ];
    for c in &path_candidates {
        if which_in_path(c) {
            eprintln!("[djbot] Found ffmpeg in PATH: {}", c);
            return Some(c.clone());
        }
    }

    // 2. Common Windows install locations
    let home = std::env::var("USERPROFILE").unwrap_or_default();
    let local_app = std::env::var("LOCALAPPDATA").unwrap_or_default();
    let program_files = std::env::var("ProgramFiles").unwrap_or_default();
    let program_files_x86 = std::env::var("ProgramFiles(x86)").unwrap_or_default();

    let fixed_candidates = vec![
        // winget / scoop / manual extracts
        format!("{}/scoop/apps/ffmpeg/current/bin/ffmpeg.exe", home),
        format!("{}/scoop/shims/ffmpeg.exe", home),
        format!("{}/AppData/Local/Microsoft/WindowsApps/ffmpeg.exe", home),
        // Chocolatey
        "C:/ProgramData/chocolatey/bin/ffmpeg.exe".to_string(),
        // Common manual installs
        format!("{}/ffmpeg/bin/ffmpeg.exe", program_files),
        format!("{}/ffmpeg-essentials/bin/ffmpeg.exe", program_files),
        format!("{}/ffmpeg/bin/ffmpeg.exe", program_files_x86),
        // imageio_ffmpeg (Python package) — multiple Python versions
        format!("{}/Python/Python312/site-packages/imageio_ffmpeg/binaries/ffmpeg.exe", local_app),
        format!("{}/Python/Python311/site-packages/imageio_ffmpeg/binaries/ffmpeg.exe", local_app),
        format!("{}/Python/Python310/site-packages/imageio_ffmpeg/binaries/ffmpeg.exe", local_app),
        format!("{}/Python/Python39/site-packages/imageio_ffmpeg/binaries/ffmpeg.exe", local_app),
        // imageio_ffmpeg installed in Roaming (pip user install)
        format!("{}/Roaming/Python/Python312/site-packages/imageio_ffmpeg/binaries/ffmpeg.exe", local_app.replace("Local", "")),
        format!("{}/Python/Python312/site-packages/imageio_ffmpeg/binaries/ffmpeg.exe", home.replace("\\", "/")),
        // Local project bin
        "./ffmpeg.exe".to_string(),
        "./bin/ffmpeg.exe".to_string(),
    ];

    for c in &fixed_candidates {
        let path = std::path::Path::new(c);
        if path.exists() {
            eprintln!("[djbot] Found ffmpeg at: {}", c);
            return Some(c.clone());
        }
    }

    // 3. Scan imageio_ffmpeg binaries dir for any Python version
    //    File may be named ffmpeg-win-x86_64-vX.Y.exe
    let roaming = format!("{}/AppData/Roaming", home);
    for py_ver in &["Python314", "Python313", "Python312", "Python311", "Python310", "Python39"] {
        let bin_dir = format!("{}/Python/{}/site-packages/imageio_ffmpeg/binaries", roaming, py_ver);
        let dir_path = std::path::Path::new(&bin_dir);
        if dir_path.exists() {
            if let Ok(entries) = std::fs::read_dir(dir_path) {
                for entry in entries.flatten() {
                    let name = entry.file_name();
                    let name_str = name.to_string_lossy();
                    if name_str.starts_with("ffmpeg") && name_str.ends_with(".exe") {
                        let full = entry.path().to_string_lossy().to_string();
                        eprintln!("[djbot] Found imageio_ffmpeg: {}", full);
                        return Some(full);
                    }
                }
            }
        }
    }

    eprintln!("[djbot] WARNING: ffmpeg not found! Analysis will fail.");
    None
}

fn which_in_path(name: &str) -> bool {
    std::process::Command::new(name)
        .arg("-version")
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .is_ok()
}
