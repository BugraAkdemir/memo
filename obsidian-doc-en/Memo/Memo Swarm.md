# 🐝 Memo Swarm

> **Status:** Beta (Settings → Beta Features)  
> **Packages:** `internal/swarm/`, `internal/app/swarm.go`, `internal/llama` (RPC), Flutter `swarm_screen`  
> **API:** `/api/swarm/*`  
> **UI:** Sidebar → **Swarm** (hidden on macOS)

---

## What is this for? (plain language)

Sometimes an AI **model file** is so large it **will not fit in one computer’s memory**.

Memo Swarm turns a few PCs at home or at the office into a **team**:

- The model file stays on **one machine** (the Host).
- Other machines **do not download the model**; they only lend **compute power** (Join).
- The goal is **capacity, not speed** — running a model you could not run alone.

Under the hood Memo only **orchestrates** llama.cpp’s existing **RPC** feature (`rpc-server` + host `llama-server --rpc`). It does not invent a new distributed runtime.

---

## Who does what?

| Role | Responsibility | Needs the GGUF file? |
|------|----------------|----------------------|
| **Host** | Opens a room, picks the model, shares the code, sets share %, starts the swarm | Yes |
| **Join** | Pastes the code, runs a local helper process | No |

---

## How to use it (three steps)

1. On **every** machine: Memo running, **Settings → Beta Features** on.
2. **Host:** Swarm screen → Host → pick model → create room → copy the **room code**.  
   Joiners register with an HTTP call; they must reach the Host’s Memo web API (same LAN / remote access, or a suitable tunnel).
3. **Other PCs:** Join → paste the code. They appear on the Host list; set each **share %** (0 means they do almost no work) → **Start Swarm**.

```
[Host PC]  model.gguf  +  room code  +  llama-server --rpc
                │
     ┌──────────┼──────────┐
     ▼          ▼          ▼
 [PC 2]      [PC 3]      [PC 4]
 rpc-server  rpc-server  rpc-server
 (compute)   (compute)   ...
```

---

## Requirements

- Linux or Windows (bundled `rpc-server`). **No Swarm UI on macOS** yet (RPC binary not packaged).
- Machines must **reach each other** (same Wi‑Fi/LAN; for remote sites, **OS-level** Tailscale or similar L3).
- Memo’s embedded Tailscale tunnel can help with HTTP; **RPC is separate OS TCP** — the embedded tunnel alone may not be enough.
- Host needs the GGUF; joiners do not.

---

## Honest limits

- **Beta.** Can break; use carefully in production-like setups.
- Starting the swarm loads the model across machines; **always** routing every chat turn to that combined server may still be finishing polish — see `handoff.md` / `PLAN_memo_swarm.md` (local).
- Usually **slower** tokens; the win is **capacity**.
- If all helper shares stay 0, there is effectively no pooling.
- Real multi-machine verification (Stage 10) depends on your hardware.

---

## Developer map

| Layer | Where |
|-------|--------|
| Room + secret | `internal/swarm/room.go` |
| Worker process | `internal/swarm/worker.go` |
| App glue | `internal/app/swarm.go` |
| llama RPC | `internal/llama/rpc_probe.go`, `StartWithRPC` |
| HTTP | `internal/webserver/handlers_swarm.go` |
| UI | `frontend/lib/screens/swarm_screen.dart` |
| Config | `config.Swarm` (`rpc_port`, …) |

---

### Related notes

- [[Features Catalog]]
- [[Llama.cpp Integration]]
- [[Advanced Settings]]
- [[00 Home]]
- Release notes: `versinNote/v3.3.3.md`
