# Memo Design System

> A local-first, privacy-focused LLM chat application with RAG memory, external provider support, and E2E-encrypted cloud sync.

> **Theme:** "Pewter Study" — a warm graphite **mid-tone** identity (neither the glare of a light theme nor the cave of a true dark one) with a single muted bronze accent. A calm, premium workspace for a "second brain".

---

## 1. Design Philosophy

- **One surface, three layers**: the whole app lives in a single mid-tone identity. Depth comes from light/elevation steps, not from color.
- **Spend the accent in one place**: bronze marks only the primary action, the active state, and progress — never decoration.
- **Plain language over system terms**: "Balanced — recommended", not "Q4_K_M". Name things by what the user controls.
- **Hardware-aware**: every model surfaces "does this fit your device?" up front.
- **Empty screens are invitations**: every empty/error state points to the next step.

---

## 2. Color System (mid-tone, no neon)

Surfaces sit deliberately between a light theme (~98% L) and a true dark theme (~11% L) — a warm-neutral graphite ramp around 26–42% L. Carried on `ThemeData` as a `ThemeExtension<ThemeColors>`, read via `MemoTheme.of(context)`.

### 2.1 Pewter (default)

| Token | Hex | Usage |
|-------|-----|-------|
| `bgApp` (surface-0) | `#2B2E33` | App background (mid-graphite, never pure black) |
| `bgPanel` (surface-1) | `#33373D` | Sidebar, cards, panels |
| `bgElement` (surface-2) | `#3C4147` | Inputs, secondary fills |
| `bgHover` (surface-3) | `#474D54` | Hover, selected |
| `textMain` (ink) | `#ECE9E3` | Primary text — warm off-white, not pure white |
| `textSecondary` | `#CFCBC3` | Secondary labels |
| `textMuted` | `#B4B0A8` | Metadata, captions |
| `textDim` | `#85827B` | Placeholders, timestamps |
| `textInverse` | `#241F18` | Text on the bronze accent |
| `borderSoft` | `#FFFFFF @ 8%` | Hairline dividers, card borders |
| `borderHover` | `#FFFFFF @ 16%` | Hover border, emphasized rule |

### 2.2 Night (deeper variant)

Same accent + ink, deeper surfaces: `bgApp #1E2024 · bgPanel #25282D · bgElement #2D3036 · bgHover #383C43`.

> The bright **light theme is retired**. `themeMode` light → Pewter, dark → Night, system → auto. Both Memo themes use `Brightness.dark` internally.

### 2.3 Accent — "Bronze" (muted, NOT neon, NOT the old gold)

| Token | Hex | Usage |
|-------|-----|-------|
| `accent` | `#B08D57` | Primary button, active tab, progress, links |
| `accentLight` / `accentHover` | `#C6A06A` | Hover (lifts slightly) |
| `accentMuted` | `#B08D57 @ 14%` | Selected-row fill, user bubble, soft tints |
| `accentPale` | `#C6A06A` | Pale fills (applied with alpha) |

### 2.4 Functional (softened, no neon)

| Token | Hex | Usage |
|-------|-----|-------|
| `green` / `successGreen` | `#6FA07B` | Running, connected, success, "fits your device" |
| `red` | `#C4736B` | Errors, delete, stop |
| `warningOrange` | `#C99A5B` | Warnings, pending, "hardware may be insufficient" |
| `warmBrown` | `#8A7B63` | Incognito badge, tertiary |

Brand colors stay literal: WhatsApp green `#25D366`.

---

## 3. Typography

`google_fonts` — a premium, legible pairing (replacing plain Inter everywhere).

| Role | Font | Usage |
|------|------|-------|
| **Display / heading** | **Schibsted Grotesk** (500/600/700) | Screen titles, large numerals (download %), stats |
| **Body / UI** | **Inter** (400/500/600) | Message content, labels, settings |
| **Mono / code-data** | **JetBrains Mono** (400/500) | Code blocks, file sizes, technical values |

Scale (px): `display 30/600 · h1 22/600 · h2 18/600 · title 16/600 · body 15/400 · label 13/500 · caption 12/400 · micro 10/600`. Line height: chat 1.7, UI 1.5.

In `theme.dart`, display/headline/title roles map to Schibsted Grotesk; body/label to Inter.

---

## 4. Spacing, Radii, Shadows

| Spacing | Value | | Radius | Value |
|---------|-------|-|--------|-------|
| xs | 4px | | `radiusSm` | 8px |
| sm | 8px | | `radiusMd` | 14px |
| md | 12px | | `radiusLg` | 20px |
| lg | 16px | | `radiusFull` | 999px |
| xl | 20px | | | |
| xxl | 24/32px | | | |

Shadows are deep (black-based) to read on the dark mid-tone surface:

| Token | Color | Blur | Offset |
|-------|-------|------|--------|
| `shadowSm` | `#000 @ 20%` | 8px | 0,2 |
| `shadowMd` | `#000 @ 25%` | 16px | 0,4 |
| `shadowLg` | `#000 @ 30%` | 32px | 0,8 |

---

## 5. Component Library

### 5.1 NavRail
- 64px, `bgPanel`, right `borderSoft`. Logo: 40×40 `bgElement` tile, bronze border, bronze "M".
- Stacked 44×44 icon buttons. Active: `accentMuted` fill + bronze icon. Settings gear pinned bottom.

### 5.2 Engine Strip (signature element)
- Persistent 40px strip at the foot of the content area (right of the NavRail).
- **Running**: green dot + `memory` icon + chat model name, then `hub` icon + "Memory: <model>", each with a one-tap stop. A "Models ›" affordance on the right.
- **Offline**: dim dot + "No model running" + bronze "Start a model" — taps jump to the Models tab.
- Reads `modelStatusProvider` + `embeddingStatusProvider` (5s poll). Calm by design; the eye goes here and to the download %.

### 5.3 Chat Sidebar
- 260px, `bgPanel`. Search + "New Chat" on top. Rows hover → `bgHover`, active → `accentMuted`. Delete on hover.

### 5.4 Chat Message Bubble
- User: right, `accentMuted` (bronze tint). Assistant: left, `bgPanel`. Max width ~62%, `radiusMd`.
- Thinking block (collapsible), agent tool-call cards, timestamp (`textDim`), images max 480px, code blocks `bgApp` + JetBrains Mono. Right-click → edit/delete.

### 5.5 Chat Input
- `bgElement` field, focus → bronze border. Image/file/STT icons muted. Send = bronze; while sending = red stop square.
- Type "/" → templates. Welcome-screen suggestions push starter text via `composerDraftProvider`.

### 5.6 Welcome View (empty chat)
- `bgElement` logo tile + bronze "M". Title + subtitle.
- Four **functional** suggestion cards (Material icons, no emoji) — tap fills the composer. Subtle fade-in entrance. "/" tip chip.

### 5.7 Model Screen (Discover / My Models)
Recommendation-first; quantization is hidden behind a smart default.

- **Header**: "Models" title + hardware chip ("Your device: 15 GB RAM · GPU/CPU"). Segmented Discover/My Models tabs (active = bronze).
- **Discover**: "Recommended for you" — curated cards (`curated_models.dart`): name, one-line description, kind pill (Chat/Memory/Vision), `~size`, **hardware-fit badge**, single **Download** button. One click resolves the best-fit GGUF (prefer Q4_K_M) via `getModelFiles` and downloads it.
  - **Advanced search** (collapsible): HF search → "Other versions" dialog listing files with **plain-language quant** ("Balanced — recommended", "Highest quality", …) + per-file fit badge.
- **My Models**: local cards with kind/size pills, Start (via `ModelConfigDialog`) / Stop, running indicator, delete, import. **Empty state**: a numbered 3-step guide (Download → Start → Chat).
- **Download banner**: inline filename + % + speed + thin bronze progress bar. Polling is adaptive + `autoDispose` (1s active / 4s idle).

### 5.8 Fit Badge
Driven by `hardwareFit(sizeBytes, gpu)` using VRAM, then system RAM (from `/api/gpu` → `ram_total_mb`), then absolute-size tiers:
- **good** → green check ("Fits your device — fast on GPU" / "Fits your device")
- **ok** → bronze check ("Runs on GPU + CPU" / "Runs (CPU)" / "Runs on most computers")
- **warn** → orange triangle ("A strong computer is recommended" / "Your hardware may be insufficient")

### 5.9 Buttons
- **Elevated/Primary**: bronze bg, `textInverse`, `radiusSm`.
- **Outlined**: transparent, `borderSoft`, `textMain`.
- **Text**: bronze. **Icon**: 20px, `textMuted`. **Danger**: `red`.

### 5.10 Inputs
- Filled `bgElement`, `borderSoft`, `radiusMd`, focus → bronze 1.5px. Hint `textDim`.

### 5.11 Settings Dialog
- `bgPanel` modal, `radiusLg`. Left tab list + right content. Active tab bronze-marked.

### 5.12 Progress & Loading
- Linear progress = bronze on `bgElement`. Spinners bronze. Skeletons for async. Error = red icon + message + retry.

---

## 6. Screen Layouts

### 6.1 Chat
```
┌───────────────────────────────────────────────┐
│ NavRail │ Sidebar │ Top bar                    │
│  64px   │  260px  ├────────────────────────────┤
│         │         │ Messages (scroll)          │
│  icons  │  chats  │                            │
│         │         │ Chat input (/ templates)   │
│         │         ├────────────────────────────┤
│         │         │ ● Engine Strip · Models ›  │
└───────────────────────────────────────────────┘
```

### 6.2 Model Screen
```
┌───────────────────────────────────────────────┐
│ Models                  [ Your device: … ]     │
│ ( Discover ) ( My Models )                     │
├───────────────────────────────────────────────┤
│ Recommended for you                            │
│ ┌───────────────┐ ┌───────────────┐            │
│ │ Llama 3.1 8B  │ │ Qwen 2.5 7B   │            │
│ │ ~4.9 GB       │ │ ~4.7 GB       │            │
│ │ ✓ Fits device │ │ ✓ Fits device │            │
│ │  [ Download ] │ │  [ Download ] │            │
│ └───────────────┘ └───────────────┘            │
│ ▸ Open advanced search                         │
└───────────────────────────────────────────────┘
```

---

## 7. States

Every view handles: **Loading** (skeleton/spinner), **Empty** (invitation + single action), **Error** (what went wrong + how to fix + retry), **Data**, **Streaming** (live tokens), **Disabled** (reduced opacity).

---

## 8. Interaction Patterns

- Right-click / long-press → context menu. Hover → `bgHover` / bronze border (no layout-shifting scale).
- Thin custom scrollbar (`bgHover`, 4px). Floating snackbars (`bgHover`). Tooltips (`bgHover` + border).
- Transitions 150–300ms; `prefers-reduced-motion` respected; visible keyboard focus; touch targets ≥ 40px.

---

## 9. Iconography

- **Material Symbols (outlined), never emoji.** 20px default, 14–16px inline, 40px for empty-state glyphs.

---

## 10. Themes

- **Pewter** (default mid-tone) + **Night** (deeper). No bright light theme.
- No hardcoded colors — always `MemoTheme.of(context)` for surfaces/text, `MemoTheme.<const>` for accent/functional. Switchable via `themeMode` (light→Pewter, dark→Night, system→auto).
