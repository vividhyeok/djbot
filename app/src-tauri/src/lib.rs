use std::sync::{Arc, Mutex};
use std::process::{Command, Stdio};
use std::io::{BufRead, BufReader};
use tauri::{Manager, State};

struct WorkerState {
    port: Arc<Mutex<Option<u16>>>,
    /// Absolute path to the app data directory used by the Go worker.
    /// Stored here so `get_output_dir` stays consistent with what we passed
    /// to the worker via `--data-dir`.
    data_dir: Arc<Mutex<Option<std::path::PathBuf>>>,
}

#[tauri::command]
fn get_worker_port(state: State<WorkerState>) -> Result<u16, String> {
    let lock = state.port.lock().map_err(|e| e.to_string())?;
    lock.ok_or_else(|| "Worker not ready yet".to_string())
}

#[tauri::command]
fn get_output_dir(state: State<WorkerState>) -> String {
    let lock = state.data_dir.lock().unwrap();
    let base = lock
        .as_ref()
        .cloned()
        .unwrap_or_else(|| std::env::current_dir().unwrap_or_default());
    let out = base.join("output");
    std::fs::create_dir_all(&out).ok();
    out.to_string_lossy().to_string()
}

/// Return the compile-time platform+arch specific filename for the Go worker.
///
/// This must match exactly what the CI build step produces; see release.yml.
fn goworker_name() -> &'static str {
    #[cfg(all(target_os = "windows", target_arch = "x86_64"))]
    return "goworker-x86_64-pc-windows-msvc.exe";

    #[cfg(all(target_os = "macos", target_arch = "aarch64"))]
    return "goworker-aarch64-apple-darwin";

    #[cfg(all(target_os = "macos", target_arch = "x86_64"))]
    return "goworker-x86_64-apple-darwin";

    #[cfg(all(target_os = "linux", target_arch = "x86_64"))]
    return "goworker-x86_64-unknown-linux-gnu";

    // Fallback: bare name, rely on PATH
    #[cfg(not(any(
        all(target_os = "windows", target_arch = "x86_64"),
        all(target_os = "macos",   target_arch = "aarch64"),
        all(target_os = "macos",   target_arch = "x86_64"),
        all(target_os = "linux",   target_arch = "x86_64"),
    )))]
    return "goworker";
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let port_state     = Arc::new(Mutex::new(None::<u16>));
    let data_dir_state = Arc::new(Mutex::new(None::<std::path::PathBuf>));

    let port_clone     = Arc::clone(&port_state);
    let data_dir_clone = Arc::clone(&data_dir_state);

    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .manage(WorkerState {
            port:     Arc::clone(&port_state),
            data_dir: Arc::clone(&data_dir_state),
        })
        .invoke_handler(tauri::generate_handler![get_worker_port, get_output_dir])
        .on_window_event(|_window, event| {
            if let tauri::WindowEvent::Destroyed = event {
                // On Windows, kill the worker by name so it doesn't linger.
                #[cfg(target_os = "windows")]
                {
                    let _ = Command::new("taskkill")
                        .args(["/F", "/IM", goworker_name(), "/T"])
                        .output();
                }
                // On macOS / Linux the child process inherits the session and
                // will receive SIGHUP / be reaped when the parent exits.
            }
        })
        .setup(move |app| {
            let resource_path = app
                .path()
                .resource_dir()
                .expect("resource dir not found");

            let worker_name = goworker_name();

            // Look for the worker binary in several locations (most → least specific):
            //   1. <resource>/binaries/<name>   – Tauri-bundled sidecar
            //   2. <resource>/<name>             – alternative bundle layout
            //   3. <cwd>/backend/<name>          – dev mode (cargo run)
            let candidates = [
                resource_path.join("binaries").join(worker_name),
                resource_path.join(worker_name),
                std::env::current_dir()
                    .unwrap_or_default()
                    .join("backend")
                    .join(worker_name),
            ];

            let sidecar_path = candidates
                .iter()
                .find(|p| p.exists())
                .cloned()
                .unwrap_or_else(|| {
                    // Last resort: bare name and hope it is in PATH
                    std::path::PathBuf::from(if cfg!(target_os = "windows") {
                        "goworker.exe"
                    } else {
                        "goworker"
                    })
                });

            eprintln!("[djbot] using worker: {}", sidecar_path.display());

            let ffmpeg = find_ffmpeg();

            // Data directory:
            //   debug  → project root (avoids triggering tauri dev hot-reload)
            //   release → OS app-data dir (writable, persists across sessions)
            let data_dir = if cfg!(debug_assertions) {
                let mut p = std::env::current_dir().unwrap_or_default();
                while p.ends_with("src-tauri") || p.ends_with("app") {
                    p.pop();
                }
                p
            } else {
                app.path()
                    .app_data_dir()
                    .unwrap_or_else(|_| std::env::current_dir().unwrap_or_default())
            };
            std::fs::create_dir_all(&data_dir).ok();

            // Persist data_dir in state for get_output_dir
            {
                let mut lock = data_dir_clone.lock().unwrap();
                *lock = Some(data_dir.clone());
            }

            let port_arc = Arc::clone(&port_clone);
            std::thread::spawn(move || {
                let mut cmd = Command::new(&sidecar_path);
                if let Some(ff) = ffmpeg {
                    cmd.args(["--ffmpeg", &ff]);
                }
                cmd.args(["--data-dir", &data_dir.to_string_lossy()]);
                cmd.stdout(Stdio::piped()).stderr(Stdio::inherit());

                match cmd.spawn() {
                    Ok(mut child) => {
                        if let Some(stdout) = child.stdout.take() {
                            let reader = BufReader::new(stdout);
                            for line in reader.lines().flatten() {
                                if let Some(port_str) = line.strip_prefix("PORT:") {
                                    if let Ok(port) = port_str.trim().parse::<u16>() {
                                        let mut lock = port_arc.lock().unwrap();
                                        *lock = Some(port);
                                        eprintln!("[djbot] Go worker listening on port {}", port);
                                    }
                                }
                            }
                        }
                        // Worker exited — log for diagnostics
                        if let Ok(status) = child.wait() {
                            eprintln!("[djbot] Go worker exited: {}", status);
                        }
                    }
                    Err(e) => {
                        eprintln!("[djbot] Failed to start Go worker ({}): {}", sidecar_path.display(), e);
                    }
                }
            });

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

/// Find a usable ffmpeg binary. Checks PATH first, then well-known install
/// locations for each platform. Returns Some(path) or None.
fn find_ffmpeg() -> Option<String> {
    // 1. Check PATH (works on all platforms after a normal install / brew install)
    if which_in_path("ffmpeg") {
        eprintln!("[djbot] ffmpeg found in PATH");
        return Some("ffmpeg".to_string());
    }

    // 2. Platform-specific locations ----------------------------------------

    // ── Windows ────────────────────────────────────────────────────────────
    #[cfg(target_os = "windows")]
    {
        let home        = std::env::var("USERPROFILE").unwrap_or_default();
        let local_app   = std::env::var("LOCALAPPDATA").unwrap_or_default();
        let program_files     = std::env::var("ProgramFiles").unwrap_or_default();
        let program_files_x86 = std::env::var("ProgramFiles(x86)").unwrap_or_default();

        let fixed: &[&str] = &[
            // Package managers
            &format!("{}/scoop/apps/ffmpeg/current/bin/ffmpeg.exe", home),
            &format!("{}/scoop/shims/ffmpeg.exe", home),
            "C:/ProgramData/chocolatey/bin/ffmpeg.exe",
            &format!("{}/AppData/Local/Microsoft/WindowsApps/ffmpeg.exe", home),
            // Manual installs
            &format!("{}/ffmpeg/bin/ffmpeg.exe", program_files),
            &format!("{}/ffmpeg-essentials/bin/ffmpeg.exe", program_files),
            &format!("{}/ffmpeg/bin/ffmpeg.exe", program_files_x86),
            // imageio_ffmpeg (installed by pip)
            &format!("{}/Python/Python312/site-packages/imageio_ffmpeg/binaries/ffmpeg.exe", local_app),
            &format!("{}/Python/Python311/site-packages/imageio_ffmpeg/binaries/ffmpeg.exe", local_app),
            &format!("{}/Python/Python310/site-packages/imageio_ffmpeg/binaries/ffmpeg.exe", local_app),
            &format!("{}/Python/Python39/site-packages/imageio_ffmpeg/binaries/ffmpeg.exe", local_app),
        ];
        for c in fixed {
            if std::path::Path::new(c).exists() {
                eprintln!("[djbot] ffmpeg found: {}", c);
                return Some(c.to_string());
            }
        }

        // Scan Roaming imageio_ffmpeg for any Python version / renamed binary
        let roaming = format!("{}/AppData/Roaming", home);
        for py_ver in &["Python314","Python313","Python312","Python311","Python310","Python39"] {
            let bin_dir = format!("{}/Python/{}/site-packages/imageio_ffmpeg/binaries", roaming, py_ver);
            if let Ok(entries) = std::fs::read_dir(&bin_dir) {
                for entry in entries.flatten() {
                    let n = entry.file_name();
                    let ns = n.to_string_lossy();
                    if ns.starts_with("ffmpeg") && ns.ends_with(".exe") {
                        let full = entry.path().to_string_lossy().to_string();
                        eprintln!("[djbot] ffmpeg found (imageio): {}", full);
                        return Some(full);
                    }
                }
            }
        }
    }

    // ── macOS ──────────────────────────────────────────────────────────────
    #[cfg(target_os = "macos")]
    {
        let candidates: &[&str] = &[
            "/opt/homebrew/bin/ffmpeg",  // Apple Silicon Homebrew
            "/usr/local/bin/ffmpeg",     // Intel Homebrew
            "/opt/local/bin/ffmpeg",     // MacPorts
            "/usr/bin/ffmpeg",           // (rare) system install
        ];
        for c in candidates {
            if std::path::Path::new(c).exists() {
                eprintln!("[djbot] ffmpeg found: {}", c);
                return Some(c.to_string());
            }
        }
    }

    // ── Linux ──────────────────────────────────────────────────────────────
    #[cfg(target_os = "linux")]
    {
        let candidates: &[&str] = &[
            "/usr/bin/ffmpeg",
            "/usr/local/bin/ffmpeg",
            "/snap/bin/ffmpeg",
            "/var/lib/flatpak/exports/bin/ffmpeg",
            "/usr/games/ffmpeg",
        ];
        for c in candidates {
            if std::path::Path::new(c).exists() {
                eprintln!("[djbot] ffmpeg found: {}", c);
                return Some(c.to_string());
            }
        }
    }

    eprintln!("[djbot] WARNING: ffmpeg not found. Audio analysis will fail.");
    eprintln!("[djbot] Install ffmpeg: https://ffmpeg.org/download.html");
    None
}

/// Returns true if `name` can be invoked from PATH.
fn which_in_path(name: &str) -> bool {
    Command::new(name)
        .arg("-version")
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .is_ok()
}
