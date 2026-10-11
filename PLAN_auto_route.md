# Auto Route

- [x] Backend: request-scoped selection from saved API models, authenticated subscription models and local chat GGUFs; published capability cache; context before truncation; preserve manual/task routing; record actual model.
- [ ] Frontend: Auto Route picker choice and live/persisted model attribution; TR/EN guidance; real HTTP routing/history regression tests, mandatory Go/Flutter verification, handoff and commit.

Routing metadata is evidence, not a measured quality score. Missing metadata stays unknown. Local GGUF context is limited by the configured runtime window; unloaded local models use a private process cleaned up after the turn. Image generation continues through the existing explicit-image routing.
