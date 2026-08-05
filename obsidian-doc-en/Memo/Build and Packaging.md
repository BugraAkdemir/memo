# 📦 Build and Packaging

Automatic packaging scripts are used to make Memo ready for the end-user.

## Linux Packaging
The `package_linux.sh` script in the root directory performs these operations:
1. **Backend Build:** Compiles the Go code as the `memo` binary.
2. **Frontend Build:** Compiles the Flutter project in `release` mode.
3. **File Preparation:** Collects all necessary files (config, data, assets) under the `build_output/memo-linux-x64/` folder.
4. **Starter Script:** Creates the `run_memo.sh` file. This script opens the backend in the background and starts the frontend.

### Execution:
```bash
./package_linux.sh
```

## Windows Packaging
The `package_windows.sh` script (still in the full support phase) prepares `.exe` outputs with a similar logic. As of v3.3.4, the installer bundles the **Visual C++ Redistributable** and installs it silently — previously a clean Windows machine without it already installed (common on fresh VMs/new PCs) couldn't launch Memo at all (`msvcp140.dll` missing).

## macOS Packaging

macOS builds are a Flutter App Sandbox target — `frontend/macos/Runner/Release.entitlements` and `DebugProfile.entitlements` must declare every capability the app actually uses, or the OS silently blocks it instead of erroring loudly. A real user-reported "connection error" on macOS (commit `420e6a5`) turned out to be exactly this:

- `com.apple.security.network.client` — missing meant Dio's calls to the local backend (`localhost:8090`) were blocked
- `com.apple.security.device.audio-input` — missing meant no mic access for the `record` package (voice input, Live Mode)
- `com.apple.security.files.user-selected.read-write` — missing meant `file_picker` couldn't access user-selected files
- `Info.plist` was also missing `NSMicrophoneUsageDescription`, required for any mic-access prompt to even show

If you're debugging a macOS-only "can't connect to backend" or "mic doesn't work" report, check these entitlements first — see [[Troubleshooting]].

## Beta Installer Scripts (v3.3.3)

`get-memo-beta.sh` / `get-memo-beta.ps1` — dedicated installer scripts for anyone who wants to track beta builds specifically, kept separate from the stable installer.

## Distribution Formats
- **Portable Folder:** A folder containing all dependencies.
- **AppImage (Planned):** A Linux package that runs in a single file.

### Linked Notes:
- [[Developer Setup Guide]]
- [[System Overview]]
- [[Troubleshooting]]
