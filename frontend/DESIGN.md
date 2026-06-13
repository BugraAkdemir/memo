# Memo Design System

> A local-first, privacy-focused LLM chat application with RAG memory, external provider support, and E2E-encrypted cloud sync.

---

## 1. Design Philosophy

- **Warm & Inviting**: Cream/gold palette instead of cold blue/gray AI chat apps
- **Desktop-native**: NavRail navigation, sidebar patterns, right-click menus
- **Content-first**: Chat messages are the hero; UI chrome is minimal and muted
- **Privacy-signaling**: Visual cues for incognito mode, local-only indicators, encryption badges

---

## 2. Color Palette

### Brand Colors
| Token | Light | Dark | Usage |
|-------|-------|------|-------|
| `accent` | `#C9A84C` (Gold) | same | Primary buttons, links, active indicators, brand elements |
| `accentLight` | `#E8C97A` | same | Hover states, secondary accents |
| `accentPale` | `#F5E8C0` | same | Badge backgrounds, selected item bg |
| `accentMuted` | `#1FC9A84C` (6% alpha) | same | Soft selection highlights, chat bubble bg for user |

### Surface Colors
| Token | Light | Dark | Usage |
|-------|-------|------|-------|
| `bgApp` | `#FDFCF0` (warm cream) | `#1C1C1E` | Main app background |
| `bgPanel` | `#F5F3E0` | `#252528` | Sidebar, cards, panels |
| `bgElement` | `#EAE8D5` | `#2C2C30` | Input fields, secondary elements |
| `bgHover` | `#DDD9C4` | `#36363A` | Hover states |

### Text Colors
| Token | Light | Dark | Usage |
|-------|-------|------|-------|
| `textMain` | `#1A1A1A` | `#F0EDE0` | Body text, primary content |
| `textSecondary` | `#2C2C2C` | `#D0CDC0` | Secondary labels |
| `textMuted` | `#4A4A4A` | `#A09D90` | Timestamps, hints, metadata |
| `textDim` | `#8A8A7A` | `#707060` | Placeholders, disabled text |
| `textInverse` | `#FDFCF0` | `#1C1C1E` | Text on accent buttons |

### Functional Colors
| Token | Value | Usage |
|-------|-------|-------|
| `green` | `#51B576` | Success, WhatsApp connected, active indicators |
| `red` | `#D35F5F` | Errors, delete actions, stop button |
| `warningOrange` | `#D4944F` | Warnings, pending states |
| `warmBrown` | `#8B6535` | Incognito mode badge, tertiary accents |

### Border Colors
| Token | Light | Dark |
|-------|-------|------|
| `borderSoft` | `#141A1A1A` (8% alpha) | `#30FFFFFF` (19% alpha) |
| `borderHover` | `#2E1A1A1A` (18% alpha) | `#50FFFFFF` (31% alpha) |

---

## 3. Typography

- **Font**: Inter (Google Fonts)
- **Weights**: Regular 400, Medium 500, Semibold 600

| Style | Size | Weight | Usage |
|-------|------|--------|-------|
| `displayLarge` | 32 | 600 | Welcome screen heading |
| `headlineMedium` | 22 | 600 | Dialog titles, section headers |
| `titleLarge` | 18 | 600 | Screen titles, sidebar chat names |
| `titleMedium` | 16 | 600 | Chat top bar title |
| `bodyLarge` | 15 | 400 | Chat message content |
| `bodyMedium` | 14 | 400 | Settings, labels, descriptions |
| `bodySmall` | 12 | 400 | Metadata, timestamps |
| `labelLarge` | 14 | 600 | Button text |
| `labelSmall` | 10 | 600 | Badge text, tags |

- **Code**: JetBrains Mono 13px (for code blocks in chat)
- **Line height**: 1.7 for chat messages, 1.5 for UI text

---

## 4. Spacing & Layout

| Token | Value |
|-------|-------|
| `paddingXs` | 4px |
| `paddingSm` | 8px |
| `paddingMd` | 12px |
| `paddingLg` | 16px |
| `paddingXl` | 20px |
| `paddingXxl` | 24px |

### Corner Radii
| Token | Value |
|-------|-------|
| `radiusSm` | 8px |
| `radiusMd` | 14px |
| `radiusLg` | 20px |
| `radiusFull` | 999px |

### Shadows
| Token | Blur | Offset | Opacity |
|-------|------|--------|---------|
| `shadowSm` | 8px | 0,2 | 4% |
| `shadowMd` | 16px | 0,4 | 7% |
| `shadowLg` | 32px | 0,8 | 10% |

---

## 5. Component Library

### 5.1 NavRail
- Width: 64px
- Background: `bgPanel`
- Right border: `borderSoft`
- Vertically stacked icon buttons (40x40px)
- Active state: icon filled + accent color
- Logo at top: 40x40 rounded square with accent border, "M" letter
- Settings gear icon at bottom

### 5.2 Chat Sidebar
- Width: 260px
- Background: `bgPanel`
- Search bar on top with icon
- "New Chat" button
- Scrollable list of chat sessions
- Each chat: title, truncated preview, timestamp
- Active chat: highlighted background
- Delete button on hover

### 5.3 Chat Message Bubble
- **User messages**: Right-aligned, `accentMuted` background, gold-ish tint
- **Assistant messages**: Left-aligned, `bgPanel` background
- Max width: 55% of screen
- Rounded corners: `radiusMd`
- Border: matching color with low opacity
- **Thinking block** (assistant only): Collapsible section with toggle
- **Agent events** (assistant only): Collapsed cards showing tool calls
- **Timestamp**: Below content, 10px, `textDim`
- **Images**: Max 480px wide, rounded, within message bubble
- **Code blocks**: JetBrains Mono, dark bg, padding 14px
- **Edit**: Right-click or long-press → edit/delete popup menu
- **Copy**: Hover shows copy icon on assistant messages

### 5.4 Chat Input
- Bottom of chat area
- **Image icon**: Picks image, shows preview above input with remove button
- **File icon**: Picks any file, shows filename above input
- **Text field**: Expanded, max height 160px, multiline
- **Send button**: Gold accent when idle, red square with stop icon when sending
- **Prompt templates**: Type "/" to trigger template popup
- Image preview box: 60x60 thumbnail with filename and close button

### 5.5 Model Store Card
- Rounded card with `bgApp` background, `borderSoft` border
- **Top row**: Model filename (bold, truncated) + delete button
- **Second row**: Repo ID (small, `textDim`)
- **Tag row**: Embedding/Vision/Think/Tool/Text badges
- **Bottom**: Download button or Start button
- Badges: Small pills with `accentPale` bg, accent text

### 5.6 Settings Dialog
- Modal dialog with `radiusLg`
- Left panel: Tab list (140px wide)
- Right panel: Content area
- Tabs: General, Providers, Llama, Memory, Cloud Sync, Identity, Orchestra, Agent, About
- Save/Cancel buttons at bottom when applicable

### 5.7 Buttons
- **Primary/Elevated**: `accent` bg, white text, `radiusSm`, 14px 24px padding
- **Outlined**: Transparent bg, `borderSoft` border, `textMain` text
- **Text**: `accent` color text only
- **Icon**: 20px icons, `textMuted` color
- **Danger**: Red bg when stop, red text when delete

### 5.8 Input Fields
- Filled: `bgElement` bg
- Border: `borderSoft`, `radiusMd`
- Focus: Gold accent border 1.5px
- Hint: `textDim` color
- Padding: 16px horizontal, 14px vertical

### 5.9 Tabs
- Horizontal tab bar (model store, settings)
- Active: bottom border or filled tab with accent
- Inactive: muted text, no border

### 5.10 Progress & Loading
- **Download progress**: Linear progress bar with percentage, speed text
- **Model loading**: Spinner with status text
- **Streaming typing**: Pulsing dots indicator
- **Error state**: Red icon + message + retry button

---

## 6. Screen Layouts

### 6.1 Chat Screen (Main)
```
┌──────────────────────────────────────────────┐
│  NavRail  │  Chat Sidebar  │  Top Bar        │
│  (64px)   │  (260px)       │  (56px)         │
│           │                │                 │
│           │  Chat list     │  Messages       │
│  icons    │  scrollable    │  scrollable     │
│           │                │                 │
│           │                │  Chat Input     │
│           │                │  (with preview) │
└──────────────────────────────────────────────┘
```

### 6.2 Model Store Screen (Redesigned)
```
┌──────────────────────────────────────────────┐
│  NavRail  │  Tab Bar: Discover | Local       │
│  (64px)   │                                  │
│           │  ┌──────────────────────────────┐ │
│           │  │  Search bar                  │ │
│           │  ├──────────────────────────────┤ │
│           │  │  Model cards (grid/list)     │ │
│           │  │                              │ │
│           │  │  ┌──────┐ ┌──────┐ ┌──────┐ │ │
│           │  │  │Card1 │ │Card2 │ │Card3 │ │ │
│           │  │  └──────┘ └──────┘ └──────┘ │ │
│           │  └──────────────────────────────┘ │
└──────────────────────────────────────────────┘
```

### 6.3 Settings Dialog
```
┌──────────────────────────────────────┐
│  Settings                    [X]     │
├──────────┬───────────────────────────┤
│ General  │  Content area            │
│ Providers│  (scrollable)            │
│ Llama    │                          │
│ Memory   │                          │
│ Sync     │                          │
│ Identity │                          │
│ Orchestra│                          │
│ Agent    │                          │
│ About    │                          │
├──────────┴───────────────────────────┤
│          [Save]  [Cancel]           │
└──────────────────────────────────────┘
```

---

## 7. States

Every view must handle:
- **Loading**: Skeleton or spinner with accent color
- **Empty**: Illustration + message + action button
- **Error**: Error icon + description + retry button
- **Data**: Normal content display
- **Streaming**: Real-time token updates, typing indicator
- **Disabled**: 30% opacity, no interaction

---

## 8. Interaction Patterns

- **Right-click / long-press**: Context menu (edit, delete, copy)
- **Hover**: Subtle bg change (`bgHover`), reveal action icons
- **Scroll**: Thin custom scrollbar (`bgHover` thumb, 4px, rounded)
- **Snackbar**: Floating, dark bg, white text, auto-dismiss
- **Tooltips**: Dark bg, white text, small size

---

## 9. Iconography

- Material Icons (outlined style preferred)
- 20px default size for action icons
- 14px for inline indicators
- 40px for empty state illustrations

---

## 10. Dark Mode

- All colors invert per the palette above
- No hardcoded colors — always use `MemoTheme.of(context)`
- System default, optional manual toggle in settings
