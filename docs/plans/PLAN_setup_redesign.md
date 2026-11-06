# Setup wizard and decision guidance

- [x] Explain general LLM decision/verification compatibility and recommend fast structured decision models, with TR/EN guidance.
- [x] Rebuild responsive setup navigation and add subscription sign-in using the existing account flow; verify persistence, subscription activation and layouts.

Verification: full Flutter suite 588 passed; final focused suite 28 passed, including the additional embedding-only regression and guidance assertions. Analyze reports 5 pre-existing infos and no new findings. CGO sqlite_fts5 build/vet/full race passed. Localization literal scan: 0 matches; git diff --check empty. Eight offscreen previews at desktop/compact widths passed and were inspected with local Noto Sans and Material Icons; live vendor login and native desktop not exercised.
