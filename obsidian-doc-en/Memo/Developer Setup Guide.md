# 🚀 Developer Setup Guide

Follow the steps below to contribute to the Memo project or run it in your local environment.

## Requirements
- **Go:** v1.20+
- **Flutter:** v3.10+ (Master or Stable channel)
- **C++ Compiler:** (Optional, if you wish to compile Llama.cpp)
- **Linux:** `build-essential`, `libgtk-3-dev`, `libayatana-appindicator3-dev`

## Setup Steps

### 1. Clone the Repository
```bash
git clone https://github.com/user/memo.git
cd memo
```

### 2. Prepare the Backend
Download dependencies and start the server:
```bash
go mod tidy
go run . --port 8090
```

### 3. Prepare the Frontend
In a separate terminal:
```bash
cd frontend
flutter pub get
flutter run -d linux
```

## Important Files
- `main.go`: Entry point.
- `app.go`: Core app logic.
- `frontend/lib/main.dart`: UI entry point.
- `config/config.yaml`: Configuration settings.

### Linked Notes:
- [[Build and Packaging]]
- [[Backend (Go) Architecture]]
