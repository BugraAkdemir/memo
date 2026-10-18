import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/friendly_error.dart';
import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/server_browse_entry.dart';
import '../providers/chat_provider.dart';
import '../providers/settings_provider.dart';

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

/// Prefs key for the folders most recently picked — a list of paths that exist
/// on ONE backend, so it is in serverCoupledPrefsKeys (local_session_state.dart).
const recentServerDirsKey = 'memo_recent_server_dirs';
const _showHiddenKey = 'memo_server_browse_show_hidden';
const _maxRecent = 5;
const double _rowHeight = 40;

class ServerFileBrowserDialog extends ConsumerStatefulWidget {
  final ServerBrowseMode mode;
  final String? startPath;

  const ServerFileBrowserDialog({super.key, required this.mode, this.startPath});

  @override
  ConsumerState<ServerFileBrowserDialog> createState() => _ServerFileBrowserDialogState();
}

class _ServerFileBrowserDialogState extends ConsumerState<ServerFileBrowserDialog> {
  ServerBrowseResult? _result;
  String? _error; // the listing itself could not be read
  String? _pathError; // a typed/pasted path could not be opened (the list stays)
  bool _loading = true;
  bool _showHidden = false;
  String? _home;
  List<String> _recent = const [];
  int _highlight = 0;

  final _pathCtrl = TextEditingController();
  final _pathFocus = FocusNode();
  final _filterCtrl = TextEditingController();
  final _filterFocus = FocusNode();
  final _scroll = ScrollController();

  @override
  void initState() {
    super.initState();
    final prefs = ref.read(prefsProvider);
    _showHidden = prefs.getBool(_showHiddenKey) ?? false;
    _recent = prefs.getStringList(recentServerDirsKey) ?? const [];
    _pathFocus.addListener(() {
      // Selecting everything on focus makes "click, paste, Enter" one motion.
      if (_pathFocus.hasFocus) {
        _pathCtrl.selection = TextSelection(baseOffset: 0, extentOffset: _pathCtrl.text.length);
      }
    });
    _load(widget.startPath ?? '');
  }

  @override
  void dispose() {
    _pathCtrl.dispose();
    _pathFocus.dispose();
    _filterCtrl.dispose();
    _filterFocus.dispose();
    _scroll.dispose();
    super.dispose();
  }

  /// Opens [path]. A refused path keeps the current listing when [typed] (the
  /// person is mid-edit; wiping the list for a typo would be hostile) and shows
  /// the reason under the path bar; a failed first load shows it in place of the
  /// list. Returns whether the folder opened.
  Future<bool> _load(String path, {bool typed = false}) async {
    setState(() {
      _loading = !typed || _result == null;
      _error = null;
      _pathError = null;
    });
    try {
      final result = await ref.read(apiClientProvider).browseServer(path);
      if (!mounted) return false;
      if (path.isEmpty) _home = result.path;
      setState(() {
        _result = result;
        _loading = false;
        _highlight = 0;
        _filterCtrl.clear();
        _pathCtrl.text = result.path;
      });
      if (_scroll.hasClients) _scroll.jumpTo(0);
      return true;
    } catch (e) {
      if (!mounted) return false;
      final reason = L10n.t('server_browse_load_error', {'e': FriendlyError.describeGeneric(e)});
      setState(() {
        _loading = false;
        if (typed && _result != null) {
          _pathError = reason;
        } else {
          _error = reason;
        }
      });
      return false;
    }
  }

  /// "~" and "~/x" mean the backend's home folder, like in a shell.
  Future<String> _expand(String raw) async {
    var path = raw.trim();
    if ((path == '~' || path.startsWith('~/') || path.startsWith('~\\')) && path.isNotEmpty) {
      var home = _home;
      if (home == null) {
        try {
          home = (await ref.read(apiClientProvider).browseServer('')).path;
          _home = home;
        } catch (_) {
          return path;
        }
      }
      path = path.length == 1 ? home : '$home${path.substring(1)}';
    }
    return path;
  }

  /// Enter in the path bar: open what was typed or pasted. In file mode, a path
  /// that names a file inside the folder it opens is the choice itself.
  Future<void> _submitPath() async {
    final raw = _pathCtrl.text.trim();
    if (raw.isEmpty) return;
    final path = await _expand(raw);
    if (!mounted) return;
    final ok = await _load(path, typed: true);
    if (!ok || !mounted) return;
    final opened = _result!.path;
    if (widget.mode == ServerBrowseMode.file && !_samePath(opened, path)) {
      final name = path.split(RegExp(r'[\\/]')).where((p) => p.isNotEmpty).lastOrNull ?? '';
      final hit = _result!.entries.where((e) => !e.isDir && e.name == name);
      if (hit.isNotEmpty) Navigator.of(context).pop(_join(opened, name));
    }
    if (mounted) _filterFocus.requestFocus();
  }

  bool _samePath(String a, String b) {
    String n(String x) => x.length > 1 && (x.endsWith('/') || x.endsWith('\\')) ? x.substring(0, x.length - 1) : x;
    return n(a) == n(b);
  }

  // Plain "/" joining, not package:path: the backend may be Windows while this
  // client is not (or the reverse), and App.BrowseServerPath accepts forward-
  // slash-joinable absolute paths whatever its own OS.
  String _join(String base, String name) => base.endsWith('/') ? '$base$name' : '$base/$name';

  Future<void> _newFolder() async {
    final parent = _result?.path ?? '';
    if (parent.isEmpty) return;
    final created = await showDialog<ServerBrowseResult>(
      context: context,
      builder: (_) => _NewFolderDialog(
        create: (name) => ref.read(apiClientProvider).makeServerDirectory(parent, name),
      ),
    );
    if (created == null || !mounted) return;
    setState(() {
      _result = created;
      _error = null;
      _pathError = null;
      _loading = false;
      _highlight = 0;
      _filterCtrl.clear();
      _pathCtrl.text = created.path;
    });
  }

  Future<void> _toggleHidden() async {
    setState(() {
      _showHidden = !_showHidden;
      _highlight = 0;
    });
    await ref.read(prefsProvider).setBool(_showHiddenKey, _showHidden);
  }

  void _choose(String path) {
    if (widget.mode == ServerBrowseMode.directory) {
      final prefs = ref.read(prefsProvider);
      final next = [path, ..._recent.where((p) => p != path)].take(_maxRecent).toList();
      prefs.setStringList(recentServerDirsKey, next);
    }
    Navigator.of(context).pop(path);
  }

  void _openEntry(ServerBrowseEntry entry) {
    final base = _result?.path ?? '';
    final child = _join(base, entry.name);
    if (entry.isDir) {
      _load(child);
      _filterFocus.requestFocus();
    } else if (widget.mode == ServerBrowseMode.file) {
      _choose(child);
    }
  }

  /// What the list shows: hidden (dot) entries only on request or when the
  /// filter itself starts with a dot, then narrowed by the filter text.
  List<ServerBrowseEntry> _visible() {
    final all = _result?.entries ?? const <ServerBrowseEntry>[];
    final q = _filterCtrl.text.trim().toLowerCase();
    final dots = _showHidden || q.startsWith('.');
    return [
      for (final e in all)
        if ((dots || !e.name.startsWith('.')) && (q.isEmpty || e.name.toLowerCase().contains(q))) e,
    ];
  }

  bool _selectable(ServerBrowseEntry e) => e.isDir || widget.mode == ServerBrowseMode.file;

  KeyEventResult _onFilterKey(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent && event is! KeyRepeatEvent) return KeyEventResult.ignored;
    final list = _visible();
    if (event.logicalKey == LogicalKeyboardKey.arrowDown && list.isNotEmpty) {
      setState(() => _highlight = (_highlight + 1).clamp(0, list.length - 1));
      _scrollToHighlight();
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.arrowUp && list.isNotEmpty) {
      setState(() => _highlight = (_highlight - 1).clamp(0, list.length - 1));
      _scrollToHighlight();
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.enter || event.logicalKey == LogicalKeyboardKey.numpadEnter) {
      if (list.isNotEmpty) {
        final e = list[_highlight.clamp(0, list.length - 1)];
        if (_selectable(e)) _openEntry(e);
      }
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  void _scrollToHighlight() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scroll.hasClients) return;
      final top = _highlight * _rowHeight;
      final bottom = top + _rowHeight;
      final view = _scroll.position.viewportDimension;
      final off = _scroll.offset;
      if (top < off) {
        _scroll.jumpTo(top);
      } else if (bottom > off + view) {
        _scroll.jumpTo(bottom - view);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final result = _result;

    return Dialog(
      backgroundColor: c.bgPanel,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusLg)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 640, maxHeight: 640),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 14, 8, 4),
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
            // Path bar: type or paste any path, Enter opens it.
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 0, 12, 0),
              child: Row(
                children: [
                  Tooltip(
                    message: L10n.t('server_browse_up_tooltip'),
                    child: IconButton(
                      icon: Icon(Icons.arrow_upward, size: 18, color: c.textMuted),
                      onPressed: (result != null && result.parent.isNotEmpty) ? () => _load(result.parent) : null,
                    ),
                  ),
                  Tooltip(
                    message: L10n.t('server_browse_home'),
                    child: IconButton(
                      icon: Icon(Icons.home_outlined, size: 19, color: c.textMuted),
                      onPressed: () => _load(''),
                    ),
                  ),
                  Expanded(
                    child: TextField(
                      controller: _pathCtrl,
                      focusNode: _pathFocus,
                      style: TextStyle(fontSize: 12.5, color: c.textMain, fontFamily: 'JetBrains Mono'),
                      textInputAction: TextInputAction.go,
                      onSubmitted: (_) => _submitPath(),
                      decoration: InputDecoration(
                        isDense: true,
                        hintText: L10n.t('server_browse_path_hint'),
                        suffixIcon: IconButton(
                          tooltip: L10n.t('server_browse_go'),
                          icon: Icon(Icons.subdirectory_arrow_left, size: 18, color: c.textMuted),
                          onPressed: _submitPath,
                        ),
                      ),
                    ),
                  ),
                  Tooltip(
                    message: _showHidden ? L10n.t('server_browse_hide_hidden') : L10n.t('server_browse_show_hidden'),
                    child: IconButton(
                      icon: Icon(_showHidden ? Icons.visibility : Icons.visibility_off_outlined,
                          size: 19, color: _showHidden ? MemoTheme.accent : c.textMuted),
                      onPressed: _toggleHidden,
                    ),
                  ),
                  // Only when a folder is being chosen: a file picker has
                  // nothing to put a new folder in for.
                  if (widget.mode == ServerBrowseMode.directory)
                    Tooltip(
                      message: L10n.t('server_browse_new_folder'),
                      child: IconButton(
                        icon: Icon(Icons.create_new_folder_outlined, size: 20, color: c.textMuted),
                        onPressed: result != null && !_loading ? _newFolder : null,
                      ),
                    ),
                ],
              ),
            ),
            if (_pathError != null)
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 4, 20, 0),
                child: Text(_pathError!, style: TextStyle(color: MemoTheme.red, fontSize: 12)),
              ),
            if (_recent.isNotEmpty && widget.mode == ServerBrowseMode.directory)
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 8, 20, 0),
                child: Wrap(
                  spacing: 6,
                  runSpacing: 4,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    Text(L10n.t('server_browse_recent'), style: TextStyle(fontSize: 11.5, color: c.textDim)),
                    for (final r in _recent)
                      ActionChip(
                        visualDensity: VisualDensity.compact,
                        label: Text(r.split(RegExp(r'[\\/]')).where((x) => x.isNotEmpty).lastOrNull ?? r,
                            style: TextStyle(fontSize: 12, color: c.textMain)),
                        tooltip: r,
                        onPressed: () => _load(r, typed: true),
                      ),
                  ],
                ),
              ),
            // Filter: narrows this folder as you type.
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 8, 20, 8),
              child: Focus(
                onKeyEvent: _onFilterKey,
                child: TextField(
                  controller: _filterCtrl,
                  focusNode: _filterFocus,
                  autofocus: true,
                  onChanged: (_) => setState(() => _highlight = 0),
                  style: TextStyle(fontSize: 13, color: c.textMain),
                  decoration: InputDecoration(
                    isDense: true,
                    prefixIcon: Icon(Icons.search, size: 18, color: c.textMuted),
                    hintText: L10n.t('server_browse_filter_hint'),
                  ),
                ),
              ),
            ),
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
              // A Wrap, not a Row: in a 280 px phone dialog the localized
              // "select this folder" button plus Cancel do not fit side by
              // side, and the second one must drop to its own line instead
              // of overflowing.
              child: Wrap(
                alignment: WrapAlignment.end,
                spacing: 8,
                runSpacing: 4,
                children: [
                  TextButton(
                    onPressed: () => Navigator.of(context).pop(),
                    child: Text(L10n.t('cancel')),
                  ),
                  if (widget.mode == ServerBrowseMode.directory)
                    FilledButton(
                      onPressed: result != null ? () => _choose(result.path) : null,
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
    final entries = _visible();
    if (entries.isEmpty) {
      final narrowed = _filterCtrl.text.trim().isNotEmpty;
      return Center(
        child: Text(
          L10n.t(narrowed ? 'server_browse_no_match' : 'server_browse_empty'),
          style: TextStyle(color: c.textDim, fontSize: 13),
        ),
      );
    }
    final highlight = _highlight.clamp(0, entries.length - 1);
    return Scrollbar(
      controller: _scroll,
      child: ListView.builder(
        controller: _scroll,
        padding: const EdgeInsets.symmetric(vertical: 4),
        itemExtent: _rowHeight,
        itemCount: entries.length,
        itemBuilder: (_, i) {
          final entry = entries[i];
          final selectable = _selectable(entry);
          return ListTile(
            dense: true,
            enabled: selectable,
            // tileColor, not a Container around the tile: a coloured box above
            // a ListTile hides its ink splash.
            tileColor: i == highlight ? c.bgElement : null,
            leading: Icon(
              entry.isDir ? Icons.folder_outlined : Icons.insert_drive_file_outlined,
              size: 20,
              color: entry.isDir ? MemoTheme.accent : c.textMuted,
            ),
            title: Text(entry.name, style: TextStyle(fontSize: 13.5, color: c.textMain), overflow: TextOverflow.ellipsis),
            onTap: selectable ? () => _openEntry(entry) : null,
          );
        },
      ),
    );
  }
}

/// Name prompt for [ServerFileBrowserDialog]'s new-folder button. Runs the
/// creation itself so a refusal can be shown under the field with the dialog
/// still open; pops the created folder on success.
class _NewFolderDialog extends StatefulWidget {
  const _NewFolderDialog({required this.create});

  final Future<ServerBrowseResult> Function(String name) create;

  @override
  State<_NewFolderDialog> createState() => _NewFolderDialogState();
}

class _NewFolderDialogState extends State<_NewFolderDialog> {
  final _controller = TextEditingController();
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final name = _controller.text.trim();
    if (name.isEmpty || _busy) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final created = await widget.create(name);
      if (mounted) Navigator.of(context).pop(created);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = L10n.t('server_browse_create_error', {'e': FriendlyError.describeGeneric(e)});
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(L10n.t('server_browse_new_folder')),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          TextField(
            controller: _controller,
            autofocus: true,
            enabled: !_busy,
            textInputAction: TextInputAction.done,
            onChanged: (_) => setState(() => _error = null),
            onSubmitted: (_) => _submit(),
            decoration: InputDecoration(labelText: L10n.t('server_browse_folder_name')),
          ),
          if (_error != null) ...[
            const SizedBox(height: 10),
            Text(_error!, style: TextStyle(color: MemoTheme.red, fontSize: 12.5)),
          ],
        ],
      ),
      actions: [
        TextButton(
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
          child: Text(L10n.t('cancel')),
        ),
        FilledButton(
          onPressed: _busy ? null : _submit,
          child: Text(L10n.t('server_browse_create')),
        ),
      ],
    );
  }
}
