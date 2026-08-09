import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/server_browse_entry.dart';
import '../providers/chat_provider.dart';

/// What the caller wants back: an existing directory, or an existing file.
enum ServerBrowseMode { directory, file }

/// Browses the *connected backend's* filesystem — not the device this app
/// happens to be running on. Every "pick a folder/file" control in this app
/// (agent project folder, CLI workdir, model import) used to open the OS's
/// native file_picker dialog, which can only ever see the local device's
/// own disk. That's indistinguishable from the right answer when the
/// backend is this same machine (Memo's original desktop-only design), but
/// silently wrong the moment it's a remote self-hosted server: the picked
/// path got sent to the backend as-is, which then looked for it on ITS OWN
/// disk and failed — concretely, this is why model import, agent chats,
/// and CLI-backed chats could all fail against a real self-hosted setup
/// while working fine locally. This dialog fixes that at the source: it
/// always shows and returns a path that genuinely exists on the backend.
///
/// Returns the picked absolute path, or null if cancelled.
Future<String?> showServerFileBrowserDialog(
  BuildContext context, {
  required ServerBrowseMode mode,
  String? startPath,
}) {
  return showDialog<String>(
    context: context,
    builder: (_) => ServerFileBrowserDialog(mode: mode, startPath: startPath),
  );
}

class ServerFileBrowserDialog extends ConsumerStatefulWidget {
  final ServerBrowseMode mode;
  final String? startPath;

  const ServerFileBrowserDialog({super.key, required this.mode, this.startPath});

  @override
  ConsumerState<ServerFileBrowserDialog> createState() => _ServerFileBrowserDialogState();
}

class _ServerFileBrowserDialogState extends ConsumerState<ServerFileBrowserDialog> {
  ServerBrowseResult? _result;
  String? _error;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load(widget.startPath ?? '');
  }

  Future<void> _load(String path) async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final result = await ref.read(apiClientProvider).browseServer(path);
      if (!mounted) return;
      setState(() {
        _result = result;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = L10n.t('server_browse_load_error', {'e': e.toString()});
        _loading = false;
      });
    }
  }

  void _onEntryTap(ServerBrowseEntry entry) {
    final base = _result?.path ?? '';
    // path/base joining is done with a plain "/" here, not package:path's
    // p.join — the backend may be Windows while this client is Linux/macOS
    // (or vice versa) under Faz 5's self-hosted model, so the *client's*
    // OS-specific separator would be wrong; App.BrowseServerPath
    // (internal/app/server_browse.go) always returns/accepts forward-slash-
    // joinable absolute paths regardless of its own OS via filepath.Abs,
    // and Go's filepath functions accept "/" on Windows too.
    final childPath = base.endsWith('/') ? '$base${entry.name}' : '$base/${entry.name}';
    if (entry.isDir) {
      _load(childPath);
    } else if (widget.mode == ServerBrowseMode.file) {
      Navigator.of(context).pop(childPath);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final result = _result;

    return Dialog(
      backgroundColor: c.bgPanel,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusLg)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 520, maxHeight: 560),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 18, 12, 8),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      L10n.t('server_browse_title'),
                      style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: c.textMain),
                    ),
                  ),
                  IconButton(
                    icon: Icon(Icons.close, size: 20, color: c.textDim),
                    onPressed: () => Navigator.of(context).pop(),
                  ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 20),
              child: Row(
                children: [
                  Tooltip(
                    message: L10n.t('server_browse_up_tooltip'),
                    child: IconButton(
                      icon: Icon(Icons.arrow_upward, size: 18, color: c.textMuted),
                      onPressed: (result != null && result.parent.isNotEmpty)
                          ? () => _load(result.parent)
                          : null,
                    ),
                  ),
                  Expanded(
                    child: Text(
                      result?.path ?? '',
                      style: TextStyle(fontSize: 12.5, color: c.textMuted, fontFamily: 'JetBrains Mono'),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 4),
            Divider(height: 1, color: c.borderSoft),
            Expanded(child: _buildBody(c, result)),
            if (widget.mode == ServerBrowseMode.file)
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 6, 20, 0),
                child: Text(
                  L10n.t('server_browse_tap_file_hint'),
                  style: TextStyle(fontSize: 12, color: c.textDim),
                ),
              ),
            Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  TextButton(
                    onPressed: () => Navigator.of(context).pop(),
                    child: Text(L10n.t('cancel')),
                  ),
                  const SizedBox(width: 8),
                  if (widget.mode == ServerBrowseMode.directory)
                    FilledButton(
                      onPressed: result != null ? () => Navigator.of(context).pop(result.path) : null,
                      child: Text(L10n.t('server_browse_select_folder')),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildBody(ThemeColors c, ServerBrowseResult? result) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator(strokeWidth: 2));
    }
    if (_error != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Text(_error!, style: TextStyle(color: MemoTheme.red, fontSize: 13), textAlign: TextAlign.center),
        ),
      );
    }
    final entries = result?.entries ?? const [];
    if (entries.isEmpty) {
      return Center(
        child: Text(L10n.t('server_browse_empty'), style: TextStyle(color: c.textDim, fontSize: 13)),
      );
    }
    return ListView.builder(
      padding: const EdgeInsets.symmetric(vertical: 4),
      itemCount: entries.length,
      itemBuilder: (_, i) {
        final entry = entries[i];
        final selectable = entry.isDir || widget.mode == ServerBrowseMode.file;
        return ListTile(
          dense: true,
          enabled: selectable,
          leading: Icon(
            entry.isDir ? Icons.folder_outlined : Icons.insert_drive_file_outlined,
            size: 20,
            color: entry.isDir ? MemoTheme.accent : c.textMuted,
          ),
          title: Text(entry.name, style: TextStyle(fontSize: 13.5, color: c.textMain)),
          onTap: selectable ? () => _onEntryTap(entry) : null,
        );
      },
    );
  }
}
