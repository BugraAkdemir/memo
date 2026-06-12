# Backup & Restore

Memo provides a complete zip-based backup and restore system (`.memo` format) for full data portability, as well as Google Drive cloud sync.

---

## 📦 Local Backup (`.memo` Format)

### Export
```
GET /api/export
```

Creates a zip archive containing:
- All chat sessions (`data/sessions/`)
- Configuration (`config/config.yaml`)
- Provider config with encrypted keys (`data/providers.json`)
- Orchestra config (`data/orchestra.json`)
- Memory store (`data/memory/`)
- WhatsApp data (`data/whatsapp/`)
- Model index (`data/models/`)

### Import
```
POST /api/import
```

Restores from a `.memo` zip file. Overwrites existing data.

### Wipe
```
POST /api/wipe
```

Completely wipes all data (sessions, memory, WhatsApp, providers). Requires double confirmation. Configuration file is preserved.

## ☁️ Cloud Sync (Google Drive)

Memo supports E2E encrypted backup to Google Drive:

1. **OAuth Authentication**: Connect your Google account via OAuth 2.0
2. **Encryption**: Data is encrypted with AES-256-GCM using a user-provided passphrase (or auto-generated machine-specific key)
3. **Key Derivation**: PBKDF2 with 600,000 iterations and random 16-byte salt
4. **Storage**: Hidden app-data folder on Google Drive
5. **Sync Triggers**: Automatic on memory save, or manual via API

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/sync/status` | Auth status, last sync time |
| `POST` | `/api/sync/start` | Force sync now |
| `POST` | `/api/sync/connect` | Start OAuth flow |

### Encryption Details

- **Algorithm**: AES-256-GCM
- **Key Derivation**: PBKDF2 (600,000 iterations)
- **Salt**: Random 16 bytes, prepended to ciphertext
- **Fallback Key**: If no passphrase is set, a persistent random UUID from `data/.machine-id` is used
- **Legacy Support**: Old SHA-256 derived keys still work for decryption

## 🔒 Security

- Backup zip files use standard ZIP compression (no encryption on the zip itself — E2E encryption is applied at the cloud sync level)
- No telemetry or cloud dependency — everything is optional
- Wipe operation is atomic and complete
