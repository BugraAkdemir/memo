import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/provider_config.dart';
import '../providers/chat_provider.dart';
import '../providers/orchestra_provider.dart';
import '../providers/provider_provider.dart';
import 'orchestra_config_dialog.dart';
import 'prompt_templates.dart';
import 'agent/agent_mode_toggle.dart';

class ChatInput extends ConsumerStatefulWidget {
   ChatInput({super.key});

  @override
  ConsumerState<ChatInput> createState() => _ChatInputState();
}

class _ChatInputState extends ConsumerState<ChatInput> {
  final _controller = TextEditingController();
  final _focusNode = FocusNode();
  bool _showTemplates = false;
  String _filterQuery = '';
  int _selectedIndex = 0;

  List<PopupItem> get _filteredItems {
    if (_filterQuery.isEmpty) return templates;
    final q = _filterQuery.toLowerCase();
    return templates.where((item) {
      return item.key.toLowerCase().contains(q) ||
          item.label.toLowerCase().contains(q);
    }).toList();
  }

  @override
  void initState() {
    super.initState();
    _controller.addListener(_onTextChanged);
  }

  void _onTextChanged() {
    final text = _controller.text;
    final show = text.startsWith('/');
    final newQuery = show ? text.substring(1) : '';
    if (show != _showTemplates || newQuery != _filterQuery) {
      setState(() {
        _showTemplates = show;
        _filterQuery = newQuery;
        _selectedIndex = 0;
      });
    }
  }

  @override
  void dispose() {
    _controller.removeListener(_onTextChanged);
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  void _dismissPopup() {
    setState(() => _showTemplates = false);
  }

  void _selectAtIndex(int index) {
    final items = _filteredItems;
    if (index < 0 || index >= items.length) return;
    final item = items[index];
    if (item.type == ItemType.template) {
      setState(() => _showTemplates = false);
      _controller.text = item.text;
      _controller.selection = TextSelection.fromPosition(
        TextPosition(offset: item.text.length),
      );
      _focusNode.requestFocus();
    } else if (item.key == '/model') {
      _dismissPopup();
      _showModelSwitcher();
    } else if (item.key == '/orchestra') {
      _dismissPopup();
      _showOrchestraConfig();
    }
  }

  bool _handleKeyEvent(KeyDownEvent event) {
    if (_showTemplates) {
      final itemCount = _filteredItems.length;
      if (itemCount == 0) return false;

      if (event.logicalKey == LogicalKeyboardKey.arrowDown) {
        setState(() => _selectedIndex = (_selectedIndex + 1) % itemCount);
        return true;
      }
      if (event.logicalKey == LogicalKeyboardKey.arrowUp) {
        setState(() => _selectedIndex = (_selectedIndex - 1 + itemCount) % itemCount);
        return true;
      }
      if (event.logicalKey == LogicalKeyboardKey.enter &&
          !HardwareKeyboard.instance.isShiftPressed) {
        _selectAtIndex(_selectedIndex);
        return true;
      }
      if (event.logicalKey == LogicalKeyboardKey.escape) {
        _dismissPopup();
        return true;
      }
      return false;
    }

    // Normal mode: Enter sends
    if (event.logicalKey == LogicalKeyboardKey.enter &&
        !HardwareKeyboard.instance.isShiftPressed) {
      _send();
      return true;
    }
    return false;
  }

  Future<void> _send() async {
    final text = _controller.text.trim();
    if (text.isEmpty) return;

    // Intercept manual /command entries
    if (text.startsWith('/')) {
      final handled = await _tryHandleManualCommand(text);
      if (handled) return;
    }

    if (ref.read(isSendingProvider)) return;

    _controller.clear();
    _dismissPopup();

    try {
      await ref.read(messagesProvider.notifier).sendMessage(text);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('${L10n.t('error')}: $e')));
      }
    }

    _focusNode.requestFocus();
  }

  /// Check if text matches a command key; if so, execute it instead of sending.
  Future<bool> _tryHandleManualCommand(String text) async {
    final cmd = text.split(' ').first;

    // Check template commands (fill input with template text)
    for (final item in templates) {
      if (item.type == ItemType.template && item.key == cmd) {
        final suffix = text.substring(cmd.length).trim();
        _controller.text = suffix.isNotEmpty
            ? '${item.text}$suffix'
            : item.text;
        _controller.selection = TextSelection.fromPosition(
          TextPosition(offset: _controller.text.length),
        );
        _focusNode.requestFocus();
        return true;
      }
    }

    // Action commands
    if (cmd == '/model') {
      _showModelSwitcher();
      return true;
    }
    if (cmd == '/orchestra') {
      _showOrchestraConfig();
      return true;
    }

    return false; // Not a known command, send as-is
  }

  void _onPopupResult(PopupResult result) {
    _dismissPopup();

    if (result is PopupInsertText) {
      _controller.text = result.text;
      _controller.selection = TextSelection.fromPosition(
        TextPosition(offset: result.text.length),
      );
      _focusNode.requestFocus();
    } else if (result is PopupModelSwitch) {
      _showModelSwitcher();
    } else if (result is PopupOrchestraSwitch) {
      _showOrchestraConfig();
    }
  }

  Future<void> _showModelSwitcher() async {
    final api = ref.read(apiClientProvider);
    String activeProvider;
    List<ProviderConfig> providers;

    try {
      activeProvider = await api.getActiveProvider();
      providers = await api.getProviders();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to load providers: $e')),
        );
      }
      return;
    }

    if (!mounted) return;

    final options = <_ModelOption>[];
    options.add(_ModelOption(
      type: 'local',
      name: 'Local Model',
      icon: '🖥️',
      subtitle: 'llama.cpp',
    ));
    for (final p in providers) {
      if (p.enabled) {
        options.add(_ModelOption(
          type: p.type,
          name: p.name,
          icon: providerIcon(p.type),
          subtitle: p.model,
        ));
      }
    }

    final selected = await showDialog<String>(
      context: context,
      builder: (ctx) => _ModelSwitcherDialog(
        options: options,
        activeType: activeProvider.isEmpty ? 'local' : activeProvider,
        onOpenRouterOAuth: () {
          Navigator.of(ctx).pop('__openrouter_oauth__');
        },
      ),
    );

    if (selected == null || !mounted) return;

    if (selected == '__openrouter_oauth__') {
      await _startOpenRouterOAuth();
      return;
    }

    try {
      if (selected == 'local') {
        await api.setActiveProvider('');
        ref.read(activeProviderTypeProvider.notifier).setActive('');
      } else {
        await api.setActiveProvider(selected);
        ref.read(activeProviderTypeProvider.notifier).setActive(selected);
      }
      if (mounted) {
        final name = options.firstWhere((o) => o.type == selected).name;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Switched to $name'),
            duration: const Duration(seconds: 2),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to switch: $e')),
        );
      }
    }
  }

  Future<void> _startOpenRouterOAuth() async {
    final api = ref.read(apiClientProvider);

    if (!mounted) return;

    // Step 1: Enter API key
    final keyController = TextEditingController();
    final keyResult = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('OpenRouter Bağlantısı'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Text(
              'openrouter.ai/keys adresinden API Key\'ini kopyalayıp aşağıya yapıştır:',
              style: TextStyle(fontSize: 13),
            ),
            const SizedBox(height: 4),
            const Text(
              'sk-or-... ile başlar',
              style: TextStyle(fontSize: 12, color: Colors.grey),
            ),
            const SizedBox(height: 16),
            TextField(
              controller: keyController,
              decoration: const InputDecoration(
                labelText: 'API Key',
                hintText: 'sk-or-...',
                border: OutlineInputBorder(),
              ),
              obscureText: true,
              autofocus: true,
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: const Text('İptal'),
          ),
          FilledButton(
            onPressed: () {
              final key = keyController.text.trim();
              if (key.isEmpty) return;
              Navigator.of(ctx).pop(key);
            },
            child: const Text('Devam'),
          ),
        ],
      ),
    );

    if (keyResult == null || keyResult.isEmpty || !mounted) return;

    // Step 2: Fetch models from OpenRouter
    // Show loading
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (_) => const Center(
        child: Card(
          child: Padding(
            padding: EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                CircularProgressIndicator(),
                SizedBox(height: 16),
                Text('Modeller yükleniyor...'),
              ],
            ),
          ),
        ),
      ),
    );

    List<Map<String, dynamic>> models;
    try {
      final result = await api.fetchOpenRouterModels(keyResult);
      if (mounted) Navigator.of(context).pop(); // close loading
      if (result['status'] != 'ok') {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('❌ ${result['error'] ?? 'Modeller alınamadı'}')),
          );
        }
        return;
      }
      models = (result['models'] as List)
          .cast<Map<String, dynamic>>();
    } catch (e) {
      if (mounted) {
        Navigator.of(context).pop();
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Hata: $e')),
        );
      }
      return;
    }

    if (!mounted) return;

    // Step 3: Show model browser
    final selectedModel = await showDialog<String>(
      context: context,
      builder: (ctx) => _OpenRouterModelDialog(models: models),
    );

    if (selectedModel == null || !mounted) return;

    // Step 4: Save provider with selected model
    try {
      final result = await api.connectOpenRouter(
        apiKey: keyResult,
        model: selectedModel,
      );
      if (result['status'] == 'done' && mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('✅ OpenRouter bağlandı!'),
            backgroundColor: Color(0xFF51B576),
          ),
        );
        ref.invalidate(providerListProvider);
        ref.invalidate(activeProviderTypeProvider);
      } else if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('❌ ${result['error'] ?? 'Bağlantı hatası'}'),
            backgroundColor: Color(0xFFD35F5F),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Hata: $e')),
        );
      }
    }
  }

  Future<void> _showOrchestraConfig() async {
    await showDialog(
      context: context,
      builder: (_) => const OrchestraConfigDialog(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final isSending = ref.watch(isSendingProvider);
    final orchestraAsync = ref.watch(orchestraConfigProvider);
    final orchestraEnabled = orchestraAsync.valueOrNull?.enabled ?? false;

    final popup = _showTemplates ? Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: PromptTemplatesPopup(
        query: _filterQuery,
        selectedIndex: _selectedIndex,
        onSelect: _onPopupResult,
        onDismiss: _dismissPopup,
      ),
    ) : null;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (popup != null)
          Stack(
            clipBehavior: Clip.none,
            children: [
              // Backdrop — catches taps outside the popup
              Positioned.fill(
                child: GestureDetector(
                  onTap: _dismissPopup,
                  behavior: HitTestBehavior.opaque,
                ),
              ),
              // Popup on top of backdrop (last child = on top)
              popup,
            ],
          ),
        // Input area
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgApp,
            border: Border(
              top: BorderSide(color: MemoTheme.of(context).borderSoft),
            ),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              _InputIconButton(
                icon: Icons.image_outlined,
                tooltip: orchestraEnabled
                    ? 'Orchestra modunda kullanılamaz'
                    : L10n.t('attach_image'),
                disabled: orchestraEnabled,
                onTap: () async {
                  final isSending = ref.read(isSendingProvider);
                  if (isSending || orchestraEnabled) return;

                  final result = await FilePicker.platform.pickFiles(
                    type: FileType.image,
                    allowMultiple: false,
                  );
                  if (result != null && result.files.single.path != null) {
                    final path = result.files.single.path!;
                    final text = _controller.text.trim();
                    _controller.clear();
                    _dismissPopup();
                    try {
                      await ref
                          .read(messagesProvider.notifier)
                          .sendFile(text, path);
                    } catch (e) {
                      if (mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(
                            content: Text('${L10n.t('error')}: $e'),
                          ),
                        );
                      }
                    }
                  }
                },
              ),
              const SizedBox(width: 4),
              _InputIconButton(
                icon: Icons.attach_file,
                tooltip: orchestraEnabled
                    ? 'Orchestra modunda kullanılamaz'
                    : L10n.t('attach_file'),
                disabled: orchestraEnabled,
                onTap: () async {
                  final isSending = ref.read(isSendingProvider);
                  if (isSending || orchestraEnabled) return;

                  final result = await FilePicker.platform.pickFiles(
                    type: FileType.any,
                    allowMultiple: false,
                  );
                  if (result != null && result.files.single.path != null) {
                    final path = result.files.single.path!;
                    final text = _controller.text.trim();
                    _controller.clear();
                    _dismissPopup();
                    try {
                      await ref
                          .read(messagesProvider.notifier)
                          .sendFile(text, path);
                    } catch (e) {
                      if (mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(
                            content: Text('${L10n.t('error')}: $e'),
                          ),
                        );
                      }
                    }
                  }
                },
              ),
              const SizedBox(width: 8),
              const AgentModeToggle(),
              const SizedBox(width: 12),

              // ─── Text Input ──────────────────────────
              Expanded(
                child: Container(
                  constraints: const BoxConstraints(maxHeight: 160),
                  decoration: BoxDecoration(
                    color: MemoTheme.of(context).bgPanel,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(
                      color: MemoTheme.of(context).borderSoft,
                    ),
                  ),
                  child: Focus(
                    onKeyEvent: (node, event) {
                      if (event is KeyDownEvent) {
                        if (_handleKeyEvent(event)) {
                          return KeyEventResult.handled;
                        }
                      }
                      return KeyEventResult.ignored;
                    },
                    child: TextField(
                      controller: _controller,
                      focusNode: _focusNode,
                      maxLines: null,
                      textInputAction: TextInputAction.newline,
                      style: TextStyle(
                        fontSize: 14,
                        color: MemoTheme.of(context).textMain,
                        height: 1.5,
                      ),
                      decoration: InputDecoration(
                        hintText: '${L10n.t('type_message')} (/)',
                        hintStyle: TextStyle(
                          color: MemoTheme.of(context).textDim,
                        ),
                        border: InputBorder.none,
                        contentPadding: const EdgeInsets.symmetric(
                          horizontal: 14,
                          vertical: 12,
                        ),
                        isDense: true,
                      ),
                    ),
                  ),
                ),
              ),

              const SizedBox(width: 12),

              // ─── Send / Stop Button ──────────────────
              AnimatedContainer(
                duration: const Duration(milliseconds: 150),
                child: Material(
                  color: isSending
                      ? MemoTheme.red
                      : MemoTheme.accent,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  child: InkWell(
                    onTap: isSending
                        ? () =>
                            ref.read(messagesProvider.notifier).stopStreaming()
                        : _send,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                    child: SizedBox(
                      width: 42,
                      height: 42,
                      child: isSending
                          ? Icon(
                              Icons.stop_rounded,
                              size: 22,
                              color: MemoTheme.of(context).textInverse,
                            )
                          : Icon(
                              Icons.send_rounded,
                              size: 20,
                              color: MemoTheme.of(context).textInverse,
                            ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _InputIconButton extends StatelessWidget {
  final IconData icon;
  final String tooltip;
  final VoidCallback onTap;
  final bool disabled;

  const _InputIconButton({
    required this.icon,
    required this.tooltip,
    required this.onTap,
    this.disabled = false,
  });

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: InkWell(
        onTap: disabled ? null : onTap,
        borderRadius: BorderRadius.circular(8),
        child: Padding(
          padding: const EdgeInsets.all(8),
          child: Icon(
            icon,
            size: 20,
            color: disabled
                ? MemoTheme.of(context).textDim.withValues(alpha: 0.3)
                : MemoTheme.of(context).textDim,
          ),
        ),
      ),
    );
  }
}

// ─── Model Switcher Dialog ──────────────────────────────────────

class _ModelOption {
  final String type;
  final String name;
  final String icon;
  final String subtitle;

  const _ModelOption({
    required this.type,
    required this.name,
    required this.icon,
    required this.subtitle,
  });
}

class _ModelSwitcherDialog extends StatelessWidget {
  final List<_ModelOption> options;
  final String activeType;
  final VoidCallback? onOpenRouterOAuth;

  const _ModelSwitcherDialog({
    required this.options,
    required this.activeType,
    this.onOpenRouterOAuth,
  });

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Switch Model'),
      content: SizedBox(
        width: 320,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              'Choose which model to use for chat:',
              style: TextStyle(
                fontSize: 13,
                color: MemoTheme.of(context).textDim,
              ),
            ),
            const SizedBox(height: 16),
            ...options.map((opt) => _ModelOptionTile(
              option: opt,
              isActive: opt.type == activeType,
              onTap: () => Navigator.of(context).pop(opt.type),
            )),
            const SizedBox(height: 16),
            TextButton.icon(
              onPressed: onOpenRouterOAuth,
              icon: const Text('🔑', style: TextStyle(fontSize: 16)),
              label: const Text('OpenRouter ile Giriş Yap'),
              style: TextButton.styleFrom(
                foregroundColor: Colors.orange,
              ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Cancel'),
        ),
      ],
    );
  }
}

class _ModelOptionTile extends StatelessWidget {
  final _ModelOption option;
  final bool isActive;
  final VoidCallback onTap;

  const _ModelOptionTile({
    required this.option,
    required this.isActive,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      color: isActive ? MemoTheme.accent.withValues(alpha: 0.08) : null,
      child: ListTile(
        leading: Text(option.icon, style: const TextStyle(fontSize: 24)),
        title: Text(
          option.name,
          style: TextStyle(
            fontWeight: isActive ? FontWeight.w600 : FontWeight.w500,
          ),
        ),
        subtitle: Text(
          option.subtitle,
          style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim),
        ),
        trailing: isActive
            ? Icon(Icons.check_circle, color: MemoTheme.accent, size: 20)
            : null,
        onTap: onTap,
        contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 0),
      ),
    );
  }
}

// ─── OpenRouter Model Browser Dialog ───────────────────────────

class _OpenRouterModelDialog extends StatefulWidget {
  final List<Map<String, dynamic>> models;
  const _OpenRouterModelDialog({required this.models});

  @override
  State<_OpenRouterModelDialog> createState() => _OpenRouterModelDialogState();
}

class _OpenRouterModelDialogState extends State<_OpenRouterModelDialog> {
  late List<Map<String, dynamic>> _filtered;
  final _searchCtrl = TextEditingController();

  @override
  void initState() {
    super.initState();
    // Show free models first, then sort by name
    _filtered = List.from(widget.models);
    _filtered.sort((a, b) {
      final af = (a['is_free'] as bool?) ?? false;
      final bf = (b['is_free'] as bool?) ?? false;
      if (af != bf) return af ? -1 : 1;
      return ((a['name'] as String?) ?? '').compareTo((b['name'] as String?) ?? '');
    });
  }

  @override
  void dispose() {
    _searchCtrl.dispose();
    super.dispose();
  }

  void _filter(String q) {
    final query = q.toLowerCase();
    setState(() {
      if (query.isEmpty) {
        _filtered = List.from(widget.models);
      } else {
        _filtered = widget.models.where((m) {
          final id = ((m['id'] as String?) ?? '').toLowerCase();
          final name = ((m['name'] as String?) ?? '').toLowerCase();
          return id.contains(query) || name.contains(query);
        }).toList();
      }
      _filtered.sort((a, b) {
        final af = (a['is_free'] as bool?) ?? false;
        final bf = (b['is_free'] as bool?) ?? false;
        if (af != bf) return af ? -1 : 1;
        return ((a['name'] as String?) ?? '').compareTo((b['name'] as String?) ?? '');
      });
    });
  }

  String _priceStr(double p) {
    if (p == 0) return 'Ücretsiz';
    if (p < 0.000001) return '\$${p.toStringAsExponential(1)}/tkn';
    return '\$${p.toStringAsFixed(7)}/tkn';
  }

  String _contextStr(int? ctx) {
    if (ctx == null || ctx == 0) return '';
    if (ctx >= 1000000) return '${(ctx / 1000).toStringAsFixed(0)}K';
    if (ctx >= 1000) return '${(ctx ~/ 1000)}K';
    return '$ctx';
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      insetPadding: const EdgeInsets.symmetric(horizontal: 32, vertical: 24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Header
          Container(
            padding: const EdgeInsets.fromLTRB(20, 16, 20, 8),
            child: Row(
              children: [
                const Icon(Icons.model_training, size: 20),
                const SizedBox(width: 8),
                const Expanded(
                  child: Text(
                    'OpenRouter Modelleri',
                    style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
                  ),
                ),
                Text(
                  '${widget.models.length} model',
                  style: TextStyle(fontSize: 12, color: Colors.grey[500]),
                ),
              ],
            ),
          ),
          // Search
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 20),
            child: TextField(
              controller: _searchCtrl,
              decoration: InputDecoration(
                hintText: 'Model ara...',
                prefixIcon: const Icon(Icons.search, size: 20),
                isDense: true,
                contentPadding: const EdgeInsets.symmetric(vertical: 8),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
              onChanged: _filter,
            ),
          ),
          const SizedBox(height: 8),
          // Model list
          Flexible(
            child: ListView.builder(
              shrinkWrap: true,
              itemCount: _filtered.length,
              padding: const EdgeInsets.symmetric(horizontal: 12),
              itemBuilder: (ctx, i) {
                final m = _filtered[i];
                final id = (m['id'] as String?) ?? '';
                final name = (m['name'] as String?) ?? id;
                final isFree = (m['is_free'] as bool?) ?? false;
                final promptPrice = (m['prompt_price'] as num?)?.toDouble() ?? 0;
                final ctxLen = m['context_length'] as int?;

                return Card(
                  margin: const EdgeInsets.only(bottom: 4),
                  child: ListTile(
                    dense: true,
                    leading: Icon(
                      isFree ? Icons.check_circle : Icons.monetization_on,
                      size: 18,
                      color: isFree ? const Color(0xFF51B576) : Colors.orange[400],
                    ),
                    title: Text(
                      id,
                      style: const TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w500,
                        fontFamily: 'monospace',
                      ),
                    ),
                    subtitle: Text(
                      [
                        if (name != id) name,
                        if (ctxLen != null && ctxLen > 0)
                          _contextStr(ctxLen),
                        _priceStr(promptPrice),
                      ].join(' · '),
                      style: TextStyle(fontSize: 11, color: Colors.grey[500]),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    trailing: const Icon(Icons.arrow_forward_ios, size: 14),
                    onTap: () => Navigator.of(context).pop(id),
                  ),
                );
              },
            ),
          ),
          // Footer
          Container(
            padding: const EdgeInsets.fromLTRB(20, 8, 20, 12),
            child: Row(
              children: [
                Text(
                  '🟢 Ücretsiz · 🟡 Ücretli',
                  style: TextStyle(fontSize: 11, color: Colors.grey[500]),
                ),
                const Spacer(),
                TextButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: const Text('İptal'),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
