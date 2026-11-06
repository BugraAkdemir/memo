# 🔄 In-App Updates

> **Packages:** `internal/updater/` (manifest, signature, download, extract, swap, rollback, components), `internal/components/` (which engine version is installed), `cmd/memo-updater/` (the swap program shipped inside each Linux/macOS release), `cmd/memo-sign/` (the maintainer's signing tool)
> **API:** `GET /api/update/check`, `GET|POST /api/update/status`, `POST /api/update/apply` (admin only)
> Full page: `docs/AUTO_UPDATE.md`

Since v4.7.0 Memo can update itself.

- **One click:** *Update now* downloads the release, verifies it, closes Memo, swaps the new files in and opens Memo again. On Windows it downloads the installer, checks it and starts it.
- **Your things are never touched:** settings, models, memory, chats and provider keys stay where they are; only the program files are replaced.
- **Rollback:** Memo waits for the new version to answer; if it doesn't, the previous files are restored and the previous version opened. The next time you open the update window it says the attempt failed and why.
- **Control:** *Later* asks again next launch; *Skip this version* is remembered; a release marked **required** hides *Skip*.
- **Signed:** every download is checked against a manifest signed by the maintainer (ed25519); a mismatch is deleted and nothing is installed. A taken-over website could withhold an update but could not make Memo install an unsigned file. The public key cannot be changed remotely.
- **Engine updates:** llama.cpp, whisper.cpp and the Subscriptions connector can be refreshed on their own, into Memo's data folder (never over the installed program); a broken one is removed and the bundled engine is used again. Offered only for the engine you actually use.
- **Installs that can't update themselves** (AppImage, packages, Docker, running from source) get an *Open download page* button.

Versions up to 4.6.0 have no updater, so moving to 4.7.0 is one last manual update. Related: [[Llama.cpp Integration]], [[Build and Packaging]].
