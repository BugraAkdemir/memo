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
The `package_windows.sh` script (still in the full support phase) prepares `.exe` outputs with a similar logic.

## Distribution Formats
- **Portable Folder:** A folder containing all dependencies.
- **AppImage (Planned):** A Linux package that runs in a single file.

### Linked Notes:
- [[Developer Setup Guide]]
- [[System Overview]]
