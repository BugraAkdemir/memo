# mmproj download and Windows frontend startup — 11 November 2026

The user requested two investigations, then authorized fixing only mmproj downloads. Windows startup behavior is unchanged. The model was described as “ofical gemma 4ab”; its exact HF repository remains unconfirmed. No real model download, vendor account or Windows machine was used for validation.

## mmproj: cause and implementation

The old `ModelDetailPanel._download` → `App.DownloadModel` → `Store.DownloadModel` path fetched only the selected GGUF. `findMmproj` discovered files already on disk but never downloaded them. A temporary HTTP fixture reproduced the defect: the repository advertised model + projector, weights downloaded, projector requests = 0.

The corrected backend flow in `internal/modelstore/projector.go`:

- Reads recursive repository files and follows same-endpoint pagination. Nested download URLs preserve slash separators. File paths cannot contain traversal or Windows drive/backslash paths.
- Selects a projector from filenames and folders. F16 is preferred among precision variants of the same projector. Ambiguous distinct projectors produce an error rather than an arbitrary association.
- Downloads and verifies the projector before completing the model operation, with one progress/cancellation/error lifecycle. Partial files are cleaned up; failures remain visible in the existing download banner.
- Isolates new vision weights by repository and keeps the selected projector at `<model path>.mmproj/mmproj.gguf`. Both local discovery and llama startup prefer that private directory; manually installed legacy sibling projectors remain supported.
- Preserves known-repository older flat weights. The repair action reuses them only when their size and available LFS checksum match; corrupt or unmatched weights are downloaded again. Repository-relative filenames are cached in metadata so nested files match the model picker.
- Model deletion also removes its owned projector directory and metadata.

The model detail panel offers “Download missing vision component” / “Eksik görsel bileşenini indir” when the repository has a projector and the downloaded model lacks one. The existing download API performs repair, so it is shared with CLI callers too. No model-family names are hardcoded. Projectors in a different repository or with an unrecognized filename are not automatically inferred.

Source: `internal/modelstore/modelstore.go`, `internal/modelstore/projector.go`, `internal/llama/llama.go`, `frontend/lib/screens/model_store/model_detail_panel.dart`, `frontend/lib/core/l10n.dart`.

## Verification

Single runs with bounded local Go parallelism; no stress/busy-loop tests:

```text
GOMAXPROCS=2 CGO_ENABLED=1 go test -p 1 -tags sqlite_fts5 ./internal/modelstore ./internal/llama -race -count=1
ok  memo/internal/modelstore  1.047s
ok  memo/internal/llama       5.542s
```

Regression coverage includes recursive pagination, nested files, identically named models/projectors in different repositories, ambiguous selection, corrupt projectors, cancellation, failed-projector retry, repair without weight re-download, owned-file deletion, and text-only models. Llama's private-directory preference is tested alongside legacy discovery.

```text
flutter analyze lib/: 3 pre-existing use_build_context_synchronously infos; no errors/warnings
flutter test --no-pub --concurrency=1 test/models_test.dart test/providers/models_provider_test.dart: 86 passed
flutter test --no-pub --concurrency=1 test/widgets/model_projector_download_test.dart: 2 passed (TR + EN repair action)
Go vet ./internal/modelstore ./internal/llama: exit 0
Changed Flutter UI literal check: no matches
```

Full Go build/vet/race tests and Flutter analysis/tests run in remote CI; the latest run/result is recorded in `handoff.md`. Native Windows runtime and real HF download acceptance remain unverified. Implementation checkpoint: `af2e1814`, branch `codex/mmproj-download`; no PR or release.

## Windows frontend investigation (unchanged)

The user clarified that backend/other processes are fast and the frontend is slow, especially after installation. These are code findings, not measured root-cause attribution:

- `AppShell` constructs invisible tabs in an `IndexedStack` at startup, starting their widgets/providers too.
- The theme uses Inter and Schibsted Grotesk via google_fonts 6.3.3, with no bundled font assets. The inspected local dependency source tries assets, disk cache, then HTTP (runtime fetching defaults to true). Font loading is asynchronous and immediately returns a fallback style: this is not proof that font HTTP requests block the first frame.
- The native Windows window is shown from Flutter's next-frame callback. Dart awaits preferences before `runApp`; notifications are inert on Windows and tray setup starts asynchronously. No duration was measured for preferences, plugin registration, engine startup or rasterization.
- The CI recipe builds Windows in release mode.
- Separately, the generated Windows launcher copies the engine tree on first launch and waits 1 + 2 seconds when starting a backend. These operations precede the frontend process and cannot explain latency measured strictly inside that process.

No Windows runtime, cold/warm trace or user startup log was available. Defender scanning, shader caching, driver and OS file-cache explanations remain hypotheses. Next discussion: capture native process entry, plugin completion, Dart entry, preferences completion, first build and first raster timestamps; compare cold/warm launches before attributing or changing Windows startup.
