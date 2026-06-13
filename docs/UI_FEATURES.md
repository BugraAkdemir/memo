# Memo UI — Feature Inventory

---

## 1. NAVIGATION (app_shell.dart)

- AppShell → NavRail (64px) + IndexedStack
- **NavRail**: Logo, Chat, Agent, Models, WhatsApp (beta), Settings (dialog)
- Active tab: filled icon, accent color
- SetupWizardOverlay (first launch)
- LlamaInstallerOverlay (download progress)
- VersionBanner (update notification)
- Theme: light/dark toggle (System, Light, Dark)

---

## 2. CHAT SCREEN (chat_screen.dart)

### 2.1 Chat Sidebar (chat_sidebar.dart)
- Width: 260px
- Search bar (filter by title)
- "New Chat" button
- Chat list: title, preview, timestamp, delete on hover
- Active chat highlight
- Empty state: "No chats yet"

### 2.2 Top Bar (_ChatTopBar)
- Title + badges (incognito, agent, agent project)
- Badge styles: icon + label, colored bg 12% alpha
- Action buttons: Undo (agent), WhatsApp toggle, Export (markdown)

### 2.3 Messages Area
- WelcomeView (empty state): Logo, tagline, feature cards, quick actions
- ChatMessageList: scrollable, auto-scroll, RepaintBoundary per bubble

### 2.4 Message Bubbles (chat_message_list.dart)

**User bubble:**
- Right-aligned, accentMuted bg (6% gold)
- Image preview (if hasImage): Image.file, 480px max, rounded
- Markdown content (MarkdownBody)
- Timestamp (always visible)

**Assistant bubble:**
- Left-aligned, avatar (M logo)
- bgPanel bg, borderSoft border
- Thinking toggle (collapsible section)
- Agent event cards (streaming tool calls)
- Markdown content
- Timestamp + copy button (on hover)

**Streaming bubble:**
- Real-time content from streamingContentProvider
- Thinking from streamingThinkingProvider

**Typing indicator:**
- 3 animated pulsing dots

### 2.5 Chat Input (chat_input.dart)

**Image picker:**
- Button → FilePicker (image type) → sets _pickedImagePath
- Preview row: 60x60 thumbnail, filename, close button
- Does NOT send immediately — user types prompt + sends manually

**File picker:**
- Button → FilePicker (any type) → sets _pickedFilePath
- Same preview + send pattern

**Text field:**
- Multiline, max 160px height
- "/" triggers prompt template popup
- Filtered list, arrow navigation, Enter selects

**Send/Stop button:**
- Gold accent (idle) → red (sending) via AnimatedContainer
- Stop icon when sending, Send icon when idle

**Send logic:**
- If image/file picked → sendFile(text, path) (streaming or non-streaming)
- If text only → sendMessage(text)
- WhatsApp mode → _sendWhatsApp(text)

---

## 3. MODEL STORE SCREEN (model_store_screen.dart)

### Current tabs:
- Discover, Local Models, Downloading (horizontal tab bar)

### 3.1 Discover Tab
- Search bar (HuggingFace search)
- Filter chips: All, Vision, Text, Embedding, Tool
- Model cards grid (2 columns)
- Card: name, author, tags, downloads, likes, download button
- Loading: skeleton cards
- Empty: "No models found"
- Error: retry button

### 3.2 Local Models Tab
- Search + sort bar
- Local model cards:
  - Filename + repo ID
  - Tags: Vision (backend mmproj_path), Embedding, Think, Tool, Text
  - File size (GB/MB format)
  - Delete button
  - Actions: Start (play icon), Config (gear), Show in Dir (folder)
  - Status: running/stopped/loading/error
- Models filtered: mmproj/multimodal files hidden
- Model config dialog: ctx_size, port, gpu_layers, start/stop

### 3.3 Downloading Tab
- Active download: progress bar, speed, ETA, cancel
- Empty: "No active downloads"

---

## 4. AGENT SCREEN (agent_screen.dart)

- Agent mode toggle
- New Agent Chat: picks project folder
- Agent chat list
- Agent messages: tool execution cards with status
- Permission requests: dialog (allow/deny/always)
- Undo button for edits

---

## 5. WHATSAPP SCREEN (whatsapp_screen.dart) — BETA

- Connection status: connected/disconnected/connecting
- QR code display for pairing
- Contact list with avatars
- Message history + search
- WhatsApp-style input

---

## 6. SETTINGS DIALOG (settings_dialog.dart)

- Left panel: tab list (140px)
- Right panel: content area
- Tabs:

### General
- Theme selector (System/Light/Dark)
- Language selector (Türkçe/English)
- Streaming toggle
- Beta features toggle
- Incognito mode toggle

### Providers
- Provider cards: name, active status
- Configure, Test, Delete buttons
- Add Provider button
- Active provider indicator

### Llama
- Binary path + Browse
- Models directory + Browse
- GPU Layers (slider)
- Engine mode (CPU/NVIDIA/AMD dropdown)
- Context size (text field)
- Temperature (slider)

### Memory
- Memory enabled toggle
- Top K results (slider/field)
- Min similarity (slider)
- Memory files list + delete

### Cloud Sync
- Google Drive auth status
- Sync enabled toggle
- Auto-sync interval
- Sync Now / Pull Now / Disconnect buttons

### Identity
- Name field
- Style selector
- System prompt textarea
- Reset to Default button

### Orchestra
- Enable toggle
- Role configuration list
- Add Role button

### Agent
- Agent enabled toggle
- Sandbox config
- Permission history list
- Clear permissions button

### About
- Version string
- Check for Updates button
- Build info

---

## 7. MODEL CONFIG DIALOG (model_config_dialog.dart)

- Title: model filename
- Context size field
- Port field
- GPU layers field
- Status indicator + Start/Stop button
- Loading state while starting

---

## 8. OTHER COMPONENTS

### 8.1 Prompt Templates
- Trigger: "/" in text field
- Filtered popup list
- Templates: /model, /orchestra, custom prompts

### 8.2 Agent Permission Dialog
- Icon + title
- Command preview (code block)
- File path display
- Allow, Deny, Allow Once, Always Allow buttons

### 8.3 Setup Wizard
- Full-screen overlay
- Steps: Welcome → Model → Provider → Done
- Next/Back/Skip navigation
- Progress indicator

### 8.4 Llama Installer
- Full-screen overlay
- Download progress bar
- Status text
- Cancel button

### 8.5 Version Banner
- Top of screen (dismissible)
- "New version available" message
- Download link

---

## 9. GLOBAL UI BEHAVIORS

### States (all components):
- Loading: spinner / skeleton
- Empty: illustration + message + action
- Error: red text + retry
- Streaming: real-time token update
- Disabled: 30% opacity

### Interactions:
- Right-click/long-press → context menu (edit, delete, copy)
- Hover → bgHover bg, reveal actions
- Scroll → thin custom scrollbar
- Errors → floating snackbar (dark bg, white text)

### Performance:
- RepaintBoundary per message bubble
- const constructors everywhere
- No AnimationController per bubble

---

## 10. DATA MODELS

### ChatMessage
- role, content, thinking, imagePath, filePath, timestamp, agentEvents
- hasImage, hasFile, hasThinking getters

### LocalModel
- repoId, filename, size, path, isEmbedding, isVision
- sizeFormatted getter (GB/MB/KB)
- Vision: determined by mmproj_path != null from backend

### StreamChunk (SSE)
- content, thinking, done, error, finishReason, stats

### DownloadProgress
- active, repoId, filename, totalBytes, downloaded, percent, speed

---

## 11. API ENDPOINTS (Flutter → Go backend)

### Chat
- POST /api/send/stream (SSE streaming text)
- POST /api/send_file/stream (SSE streaming file/image upload)
- POST /api/send (non-streaming text)
- POST /api/send_file (non-streaming file)
- GET /api/messages
- POST /api/messages/update
- POST /api/messages/delete

### Sessions
- GET /api/chats
- POST /api/chats/new
- POST /api/chats/switch
- POST /api/chats/delete
- POST /api/chats/rename
- GET /api/chats/active
- GET /api/chat/export
- GET /api/chat/title

### Models
- GET /api/models/local
- DELETE /api/models/local
- POST /api/models/import
- POST /api/models/start
- POST /api/models/stop
- GET /api/models/status
- POST /api/models/embedding/start
- POST /api/models/embedding/stop
- GET /api/models/embedding/status
- GET /api/models/search
- GET /api/models/files
- POST /api/models/download
- GET /api/models/download/progress
- POST /api/models/download/cancel

### Providers
- GET /api/providers
- POST /api/providers/update
- DELETE /api/providers/delete
- POST /api/providers/test
- POST /api/providers/activate

### Identity
- GET /api/system-prompt
- POST /api/system-prompt
- POST /api/system-prompt/reset
- GET /api/incognito-prompt
- POST /api/incognito-prompt

### Memory
- POST /api/memory/clear
- GET /api/memory/files
- DELETE /api/memory/files
- GET /api/memory/settings
- POST /api/memory/settings
- GET /api/memory/enabled
- POST /api/memory/enabled

### Agent
- GET /api/agent/enabled
- POST /api/agent/enabled
- GET /api/agent/permissions
- POST /api/agent/permission
- DELETE /api/agent/permission
- POST /api/agent/permissions/clear
- POST /api/agent/undo
- POST /api/agent/chat

### Sync
- GET /api/sync/auth
- POST /api/sync/auth
- GET /api/sync/account
- GET /api/sync/settings
- POST /api/sync/settings
- POST /api/sync/trigger
- POST /api/sync/pull
- POST /api/sync/disconnect

### WhatsApp
- POST /api/whatsapp/start
- POST /api/whatsapp/stop
- GET /api/whatsapp/status
- POST /api/whatsapp/send
- GET /api/whatsapp/search
- GET /api/whatsapp/chats
- GET /api/whatsapp/messages
- GET /api/whatsapp/stats
- POST /api/whatsapp/chat/stream (SSE)

### System
- GET /api/version
- GET /api/version/check
- GET /api/status
- GET /api/incognito
- POST /api/incognito
- GET /api/gpu
- GET /api/image
- POST /api/transcribe
- GET /api/models/llama/check
- POST /api/models/llama/install
- POST /api/models/llama/skip
- GET /api/models/config
- PUT /api/models/config
- POST /api/export
- POST /api/import
- POST /api/wipe
- GET /api/events
- GET /api/orchestra
- POST /api/orchestra
- GET /api/remote-access
- POST /api/remote-access
