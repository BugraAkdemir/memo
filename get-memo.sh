#!/usr/bin/env bash
# Memo — one-line installer for Linux and macOS.
#
#   curl -fsSL https://download.bugradev.com/install.sh | bash
#
# Downloads the packaged build for this OS, installs the persistent bits
# (engine binaries, config, data dirs) into ~/.memo exactly like run_memo.sh
# does on first launch, and puts a `memo` wrapper on PATH so a plain `memo`
# in any terminal opens the CLI. Safe to re-run — existing config/data are
# never overwritten, only the binaries are refreshed.
set -euo pipefail

DOMAIN="https://download.bugradev.com"
APP_NAME="Memo"
MEMO_HOME="$HOME/.memo"

os="$(uname -s)"
case "$os" in
    Linux)  url="$DOMAIN/memo.tar.gz" ;;
    Darwin) url="$DOMAIN/memo-mac.zip" ;;
    *)
        echo "Desteklenmeyen işletim sistemi: $os" >&2
        exit 1
        ;;
esac

for bin in curl; do
    command -v "$bin" >/dev/null 2>&1 || { echo "Hata: '$bin' bulunamadı, kurup tekrar dene." >&2; exit 1; }
done

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

archive="$work_dir/$(basename "$url")"
echo "İndiriliyor: $url"
curl -fsSL -o "$archive" "$url"

echo "Açılıyor..."
case "$os" in
    Linux)  tar xzf "$archive" -C "$work_dir" ;;
    Darwin)
        command -v unzip >/dev/null 2>&1 || { echo "Hata: 'unzip' bulunamadı." >&2; exit 1; }
        unzip -q "$archive" -d "$work_dir"
        ;;
esac

src="$work_dir/$APP_NAME"
[ -d "$src" ] || src="$work_dir"

mkdir -p "$MEMO_HOME/bin" "$MEMO_HOME/config" \
    "$MEMO_HOME/data/models" "$MEMO_HOME/data/memory" "$MEMO_HOME/data/sessions" \
    "$MEMO_HOME/data/agent-backups" "$MEMO_HOME/data/skills" "$MEMO_HOME/data/whatsapp"

# Engine binaries (llama-server + vec0) — first-install only, matches
# run_memo.sh's own first-run copy so a later desktop-app install doesn't
# clobber a working engine with a stale one.
if [ ! -d "$MEMO_HOME/binaries" ] && [ -d "$src/binaries" ]; then
    echo "Motor binary'leri kopyalanıyor (llama.cpp + vec0)..."
    cp -r "$src/binaries" "$MEMO_HOME/binaries"
fi

# The memo CLI binary itself. Older archives only ship memo-backend (before
# the plain `memo` copy was added) — fall back to that.
if [ -f "$src/memo" ]; then
    cp -f "$src/memo" "$MEMO_HOME/bin/memo"
elif [ -f "$src/memo-backend" ]; then
    cp -f "$src/memo-backend" "$MEMO_HOME/bin/memo"
else
    echo "Hata: arşivde memo/memo-backend binary'si bulunamadı." >&2
    exit 1
fi
chmod +x "$MEMO_HOME/bin/memo"

# Config / env / provider defaults — only seeded if missing, so an existing
# install's settings are never overwritten by a re-run or upgrade.
if [ ! -f "$MEMO_HOME/config/config.yaml" ]; then
    if [ -f "$src/config/config.yaml" ]; then
        cp "$src/config/config.yaml" "$MEMO_HOME/config/config.yaml"
    elif [ -f "$src/config/config.yaml.example" ]; then
        cp "$src/config/config.yaml.example" "$MEMO_HOME/config/config.yaml"
    fi
fi
if [ ! -f "$MEMO_HOME/.env" ] && [ -f "$src/.env" ]; then
    cp "$src/.env" "$MEMO_HOME/.env"
fi
if [ ! -f "$MEMO_HOME/data/providers.json" ] && [ -f "$src/data/providers.example.json" ]; then
    cp "$src/data/providers.example.json" "$MEMO_HOME/data/providers.json"
fi
if [ ! -f "$MEMO_HOME/data/permissions.json" ]; then
    echo '[]' > "$MEMO_HOME/data/permissions.json"
fi

# ~/.local/bin/memo: a wrapper, not a plain symlink. Without MEMO_DATA_DIR
# pinned, the backend falls back to a relative "data" dir — meaning `memo`
# run from some other directory would create a stray ./data there instead
# of using ~/.memo/data. The wrapper pins the data dir but does NOT cd, so
# the caller's cwd (used for agent file access) is preserved.
mkdir -p "$HOME/.local/bin"
cat > "$HOME/.local/bin/memo" <<WRAPPER
#!/bin/bash
export MEMO_DATA_DIR="$MEMO_HOME/data"
exec "$MEMO_HOME/bin/memo" "\$@"
WRAPPER
chmod +x "$HOME/.local/bin/memo"

# Make sure ~/.local/bin is actually on PATH for new shells.
case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;; # already present
    *)
        for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
            if [ -f "$rc" ] && ! grep -q '\.local/bin' "$rc" 2>/dev/null; then
                echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$rc"
            fi
        done
        if [ -d "$HOME/.config/fish" ] && ! grep -rq '\.local/bin' "$HOME/.config/fish/config.fish" 2>/dev/null; then
            mkdir -p "$HOME/.config/fish"
            echo 'fish_add_path $HOME/.local/bin' >> "$HOME/.config/fish/config.fish"
        fi
        ;;
esac

echo ""
echo "✓ Memo kuruldu: $HOME/.local/bin/memo"
if command -v memo >/dev/null 2>&1; then
    echo "  'memo' yazarak başlayabilirsin."
else
    echo "  Yeni bir terminal aç (ya da kabuğunu yeniden başlat) ve 'memo' yaz."
fi
