# 📖 User Guide

Follow these steps to start your local AI experience with Memo. For a
longer, more detailed walkthrough (including the new Self-Driving task
loop and Agent Mode), see [`guide/en/`](../../guide/en/) at the repo root.

## Initial Setup
1. Start the application.
2. **Setup Wizard:** Determine your name, the assistant's name, and the "System Prompt" (personality).
3. **Llama Check:** The application will check for the necessary engines and ask for your permission to download them if they are missing.

## Acquiring Models
1. Click the **Model Store** (Factory) icon from the side menu.
2. Discover popular models on Hugging Face.
3. Download a model suitable for your computer's RAM/VRAM capacity.
4. When the download is finished, bring the AI to life with the "Start Model" button.

## Chat and Memory
- **Starting a Chat:** Open a new topic with the `+` button in the upper left.
- **Remembering:** Memo begins to get to know you as you talk. A few days later, you will see it remembers when you ask about an old topic.
- **Adding Files:** Add files using the `+` button when having code written or having a document summarized, or type `@` to mention a specific file by name.
- **Ask it to reflect:** type `/insight` to have Memo look back over your recent mood and memory for a real pattern (it says so if there isn't enough to go on).
- **Switching models fast:** click the model/provider pill in the chat's top bar instead of opening Settings.

## Letting Memo Act On Its Own
- **Routines** (sidebar → Routines): describe something in plain language ("every morning at 8, summarize my calendar") and Memo schedules it — as a simple message or, if you enable it, a full tool-using agent run. Works on desktop and mobile.
- **Proactive nudges:** Memo may notice a pattern and bring it up on its own — a banner appears with Yes / Not now / Stop asking. Turn this off entirely, or trim it down with Minimal Mode, in Settings → General.
- **Live Mode:** the voice icon next to the chat input opens a full-screen, real-time voice call (native audio-to-audio, not the old speech-to-text relay) — pick Google Live or OpenAI Realtime as the engine in Settings first.
- **Self-Driving tasks:** hand Memo a `Task.md` checklist and it works through it unattended — see the Agent Mode section of [`guide/en/`](../../guide/en/).

## Settings and Customization
- **System Prompt:** You can change how the assistant should behave (e.g., "Give short and concise answers" or "Behave like a coding expert") from here.
- **Cloud Sync:** Connect to Google Drive to back up your data.
- Settings is now a searchable rail — type a few letters of what you're looking for instead of hunting through tabs.

---
> **Tip:** For faster responses, you can benefit from the power of your graphics card by increasing the "GPU Layers" count in the settings.
