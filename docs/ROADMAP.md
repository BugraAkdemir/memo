# Memo Roadmap

This is a living snapshot of what's actively planned past the current
release (v3.3.4), not a commitment to dates or a final feature list. Items
move, get reshaped, or get dropped as real usage informs them. See
[`versinNote/`](../versinNote/) for what's actually shipped in each past
release.

## Mobile

- **iOS build verification in CI** — `mobile/ios/` already has a full
  Xcode project scaffold, but nothing currently builds it. A
  `flutter build ios --no-codesign` job (parallel to the Android debug APK
  CI added alongside this roadmap) closes that gap first.
- **Remote backend connection** — the mobile app needs the same
  "Backend URL + Token" flow the desktop client got for connecting to a
  Memo instance running elsewhere (LAN, Tailscale, ngrok, a CasaOS
  container). Without it, the mobile app can only be useful next to a
  backend on the same machine, which isn't really the point of a mobile
  companion app.
- **Feature parity audit against desktop** — agent mode, the memory view,
  and other desktop-only surfaces need an explicit pass to decide what's
  actually missing on mobile vs. intentionally left out.
- **Live Mode on mobile** — the hands-free voice conversation mode that
  shipped as beta on desktop in v3.3.4 fits a phone use case arguably
  better than a desktop one.

## Platform Reach

- **arm64 Docker image** — the current image is amd64-only. The ARM Linux
  binaries already produced for the desktop build (and distributed via
  R2) plus the existing Docker/CasaOS backend-only setup are the two
  pieces needed to add a native arm64 variant.
- **Official CasaOS App Store listing** — today's docs tell a user to
  build and push their own image; getting Memo listed in CasaOS's own
  store would remove that step entirely.
- **Real-hardware verification** — the ARM build and the Docker image
  have both only ever been verified by simulating the target environment
  in CI/sandboxes, never on an actual Raspberry Pi or NAS. Needed before
  either can be called properly supported.
- **Package manager distribution** *(nice-to-have, not blocking)* — a
  Homebrew tap for macOS and a winget/Chocolatey package for Windows,
  alongside the existing curl/irm one-line installers. Mainly a trust and
  discoverability improvement for non-technical users who are more
  comfortable installing through a package manager they already know.

## Memo Swarm

`internal/swarm/` is real but still small (~950 lines) — distributed
inference across multiple machines, currently Beta. Maturing it out of
Beta needs to start from actual usage friction (the host/join flow, room
codes) rather than a guessed feature list — scoping this properly is next,
informed by real sessions using it.

## Stability & Accessibility

Memo's audience is split between non-technical/privacy-focused users and
technical self-hosters — this release balances both rather than favoring
either:

- **Beta channel visibility in-app** — Settings should show when you're
  running a beta build and link to the beta downloads, making the
  R2-based beta channel (set up alongside this roadmap) visible to users,
  not just something `curl`-savvy people know to look for.
- **Plain-language error messages, extended further** — v3.3.4 already
  did this pass for the setup wizard and model store; the WhatsApp bridge
  and remote-access (Tailscale/ngrok) screens are the next most
  technical-feeling surfaces that would benefit from the same treatment.
