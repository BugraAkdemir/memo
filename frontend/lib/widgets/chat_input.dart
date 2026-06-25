import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import 'glass_surface.dart';
import '../models/agent.dart';
import '../models/chat.dart';
import '../models/provider_config.dart';
import '../providers/chat_provider.dart';
import '../providers/orchestra_provider.dart';
import '../providers/provider_provider.dart';
import '../providers/recording_provider.dart';
import '../providers/skill_provider.dart';
import '../providers/whatsapp_provider.dart';
import 'orchestra_config_dialog.dart';
import 'prompt_templates.dart';
import 'skill_config_dialog.dart';

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
  String? _pickedImagePath;

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
        setState(
          () => _selectedIndex = (_selectedIndex - 1 + itemCount) % itemCount,
        );
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
    final imagePath = _pickedImagePath;
    if (text.isEmpty && imagePath == null) return;

    // Intercept manual /command entries
    if (imagePath == null && text.startsWith('/')) {
      final handled = await _tryHandleManualCommand(text);
      if (handled) return;
    }

    if (ref.read(isSendingProvider)) return;

    _controller.clear();
    _dismissPopup();
    setState(() => _pickedImagePath = null);

    // WhatsApp chat mode — route to WhatsApp stream when active
    if (ref.read(whatsAppChatModeProvider)) {
      await _sendWhatsApp(text);
      return;
    }

    try {
      if (imagePath != null) {
        await ref.read(messagesProvider.notifier).sendFile(text, imagePath);
      } else {
        await ref.read(messagesProvider.notifier).sendMessage(text);
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('${L10n.t('error')}: $e')));
      }
    }

    if (!mounted) return;
    _focusNode.requestFocus();
  }

  Future<void> _sendWhatsApp(String text) async {
    final api = ref.read(apiClientProvider);
    final timestamp = DateTime.now().toIso8601String().substring(11, 19);
    final userMsg = ChatMessage(
      role: 'user',
      content: text,
      timestamp: timestamp,
    );
    final cancelToken = CancelToken();

    ref.read(isSendingProvider.notifier).state = true;
    ref.read(messagesProvider.notifier).addMessage(userMsg);

    ref.read(streamingContentProvider.notifier).state = '';
    ref.read(streamingAgentEventsProvider.notifier).state = [];

    List<AgentEvent> finalAgentEvents = [];
    try {
      await for (final chunk
          in api.sendWhatsAppChatStream(text, cancelToken: cancelToken)) {
        if (chunk.finishReason == 'agent_event') {
          // Tool activity — render as status badges, NOT as raw JSON text.
          try {
            final ev = AgentEvent.fromJson(
                json.decode(chunk.content) as Map<String, dynamic>);
            final events = [...ref.read(streamingAgentEventsProvider)];
            if (ev.type == 'tool_executing' ||
                ev.type == 'tool_result' ||
                ev.type == 'tool_error') {
              // Collapse the transient "executing" line into its result.
              if (events.isNotEmpty && events.last.type == 'tool_executing') {
                events[events.length - 1] = ev;
              } else {
                events.add(ev);
              }
              ref.read(streamingAgentEventsProvider.notifier).state = events;
              finalAgentEvents = events;
            }
            // Other event types (final_response, etc.) carry no badge — the
            // actual reply text arrives separately as plain content chunks.
          } catch (_) {
            // ignore malformed event payloads
          }
        } else {
          ref.read(streamingContentProvider.notifier).state =
              ref.read(streamingContentProvider) + chunk.content;
        }
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('WhatsApp hatası: $e')),
        );
      }
    }

    final full = ref.read(streamingContentProvider);
    if (full.isNotEmpty || finalAgentEvents.isNotEmpty) {
      ref.read(messagesProvider.notifier).addMessage(
            ChatMessage(
              role: 'assistant',
              content: full,
              timestamp: timestamp,
              agentEvents:
                  finalAgentEvents.isNotEmpty ? finalAgentEvents : null,
            ),
          );
    }
    ref.read(streamingContentProvider.notifier).state = '';
    ref.read(streamingAgentEventsProvider.notifier).state = [];
    ref.read(isSendingProvider.notifier).state = false;
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
    if (cmd == '/skill') {
      _showSkillManager();
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
    } else if (result is PopupSkillSelect) {
      _showSkillManager();
    }
  }

  Future<void> _showSkillManager() async {
    await showDialog(
      context: context,
      builder: (_) => const SkillConfigDialog(),
    );
    ref.invalidate(skillListProvider);
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
          SnackBar(
            content: Text(L10n.t('providers_load_failed', {'e': e.toString()})),
          ),
        );
      }
      return;
    }

    if (!mounted) return;

    final options = <_ModelOption>[];
    options.add(
      _ModelOption(
        type: 'local',
        name: L10n.t('local_model'),
        icon: '🖥️',
        subtitle: L10n.t('llama_cpp'),
      ),
    );
    for (final p in providers) {
      if (p.enabled) {
        options.add(
          _ModelOption(
            type: p.name,
            name: p.name,
            icon: providerIcon(p.type),
            subtitle: p.model,
          ),
        );
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
            content: Text(L10n.t('switched_to', {'name': name})),
            duration: const Duration(seconds: 2),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('switch_failed', {'e': e.toString()}))),
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
        title: Text(L10n.t('openrouter_connect')),
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

    keyController.dispose();
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
            SnackBar(
              content: Text('❌ ${result['error'] ?? 'Modeller alınamadı'}'),
            ),
          );
        }
        return;
      }
      models = (result['models'] as List).cast<Map<String, dynamic>>();
    } catch (e) {
      if (mounted) {
        Navigator.of(context).pop();
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Hata: $e')));
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
            backgroundColor: MemoTheme.green,
          ),
        );
        ref.invalidate(providerListProvider);
        ref.invalidate(activeProviderTypeProvider);
      } else if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('❌ ${result['error'] ?? 'Bağlantı hatası'}'),
            backgroundColor: MemoTheme.red,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Hata: $e')));
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

    // Starter text pushed from the welcome screen — fill the field and focus.
    ref.listen(composerDraftProvider, (_, next) {
      if (next != null && next.isNotEmpty) {
        _controller.text = next;
        _controller.selection = TextSelection.fromPosition(
          TextPosition(offset: next.length),
        );
        _focusNode.requestFocus();
        ref.read(composerDraftProvider.notifier).state = null;
      }
    });

    final popup = _showTemplates
        ? Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: PromptTemplatesPopup(
              query: _filterQuery,
              selectedIndex: _selectedIndex,
              onSelect: _onPopupResult,
              onDismiss: _dismissPopup,
            ),
          )
        : null;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (popup != null)
          // Height-0 anchor so the popup overlays upward without pushing layout
          SizedBox(
            height: 0,
            child: OverflowBox(
              maxHeight: double.infinity,
              alignment: Alignment.bottomCenter,
              child: Stack(
                clipBehavior: Clip.none,
                children: [
                  Positioned.fill(
                    child: GestureDetector(
                      onTap: _dismissPopup,
                      behavior: HitTestBehavior.opaque,
                    ),
                  ),
                  popup,
                ],
              ),
            ),
          ),
        // Image preview
        if (_pickedImagePath != null)
          Container(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
            decoration: BoxDecoration(
              color: MemoTheme.of(context).bgApp,
              border: Border(
                top: BorderSide(color: MemoTheme.of(context).borderSoft),
              ),
            ),
            child: Row(
              children: [
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: Image.file(
                    File(_pickedImagePath!),
                    width: 60,
                    height: 60,
                    fit: BoxFit.cover,
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    _pickedImagePath!.split('/').last,
                    style: TextStyle(
                      fontSize: 12,
                      color: MemoTheme.of(context).textDim,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                IconButton(
                  icon: Icon(Icons.close, size: 16),
                  padding: EdgeInsets.zero,
                  constraints: BoxConstraints(),
                  color: MemoTheme.of(context).textDim,
                  onPressed: () => setState(() => _pickedImagePath = null),
                ),
              ],
            ),
          ),
        // Input area — a floating frosted-glass bar in Glass Light; a flush
        // top-bordered bar in dark.
        Padding(
          padding: MemoTheme.of(context).isGlass
              ? const EdgeInsets.fromLTRB(12, 4, 12, 12)
              : EdgeInsets.zero,
          child: GlassBlur(
          borderRadius: MemoTheme.of(context).isGlass
              ? const BorderRadius.only(
                  topRight: Radius.circular(MemoTheme.radiusLg),
                  bottomRight: Radius.circular(MemoTheme.radiusLg),
                )
              : BorderRadius.zero,
          child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: MemoTheme.of(context).isGlass
              ? BoxDecoration(
                  color: MemoTheme.of(context).bgPanel,
                  borderRadius: const BorderRadius.only(
                    topRight: Radius.circular(MemoTheme.radiusLg),
                    bottomRight: Radius.circular(MemoTheme.radiusLg),
                  ),
                  border: Border.all(color: MemoTheme.of(context).borderSoft),
                  boxShadow: MemoTheme.shadowMd,
                )
              : BoxDecoration(
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
                    setState(() => _pickedImagePath = result.files.single.path);
                    _focusNode.requestFocus();
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
                          SnackBar(content: Text('${L10n.t('error')}: $e')),
                        );
                      }
                    }
                  }
                },
              ),
              // Orchestra quick toggle
              _InputIconButton(
                icon: orchestraEnabled ? Icons.queue_music : Icons.queue_music_outlined,
                tooltip: orchestraEnabled ? '🎵 Orchestra: Açık (düzenle)' : '🎵 Orchestra: Kapalı (aç)',
                disabled: false,
                iconColor: orchestraEnabled ? MemoTheme.accent : null,
                onTap: () async {
                  if (orchestraEnabled) {
                    // Already enabled → open config dialog
                    _showOrchestraConfig();
                  } else {
                    // Toggle on with a single tap
                    try {
                      final api = ref.read(apiClientProvider);
                      final current = await api.getOrchestraConfig();
                      await api.updateOrchestraConfig(current.copyWith(enabled: true));
                      ref.invalidate(orchestraConfigProvider);
                    } catch (e) {
                      if (mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('Orchestra açılamadı: $e')),
                        );
                      }
                    }
                  }
                },
              ),
              // Mic toggle for voice keyboard
              () {
                final recState = ref.watch(recordingProvider);
                final isRecording = recState == RecordingState.recording;
                final isTranscribing = recState == RecordingState.transcribing;

                return TweenAnimationBuilder<double>(
                  tween: Tween(begin: 1.0, end: isRecording ? 1.12 : 1.0),
                  duration: const Duration(milliseconds: 800),
                  curve: Curves.easeInOut,
                  builder: (context, scale, _) {
                    return AnimatedContainer(
                      duration: const Duration(milliseconds: 300),
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        boxShadow: isRecording
                            ? [
                                BoxShadow(
                                  color: MemoTheme.red.withAlpha(80),
                                  blurRadius: 16,
                                  spreadRadius: 3,
                                ),
                              ]
                            : [],
                      ),
                      child: Transform.scale(
                        scale: scale,
                        child: _InputIconButton(
                          icon: isRecording
                              ? Icons.mic
                              : isTranscribing
                                  ? Icons.hourglass_top
                                  : Icons.mic_outlined,
                          tooltip: isRecording
                              ? L10n.t('mic_stop_recording')
                              : isTranscribing
                                  ? L10n.t('mic_transcribing')
                                  : L10n.t('mic_start_recording'),
                          disabled: isSending || isTranscribing,
                          iconColor: isRecording
                              ? MemoTheme.red
                              : isTranscribing
                                  ? MemoTheme.accent
                                  : null,
                          onTap: () async {
                            final notifier = ref.read(recordingProvider.notifier);
                            if (isRecording) {
                              final text = await notifier.stop();
                              if (text != null && text.isNotEmpty && mounted) {
                                final current = _controller.text;
                                final sep = current.isEmpty ? '' : ' ';
                                _controller.text = '$current$sep$text';
                                _controller.selection = TextSelection.collapsed(
                                  offset: _controller.text.length,
                                );
                                _focusNode.requestFocus();
                              }
                            } else {
                              notifier.start();
                            }
                          },
                        ),
                      ),
                    );
                  },
                );
              }(),
              // Status label when recording/transcribing
              if (ref.watch(recordingProvider) != RecordingState.idle)
                Padding(
                  padding: const EdgeInsets.only(left: 8),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (ref.watch(recordingProvider) == RecordingState.transcribing)
                        SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: MemoTheme.accent,
                          ),
                        ),
                      if (ref.watch(recordingProvider) == RecordingState.transcribing)
                        const SizedBox(width: 6),
                      if (ref.watch(recordingProvider) == RecordingState.recording)
                        _RecordingDot(),
                      if (ref.watch(recordingProvider) == RecordingState.recording)
                        const SizedBox(width: 6),
                      Text(
                        ref.watch(recordingProvider) == RecordingState.recording
                            ? L10n.t('mic_recording')
                            : L10n.t('mic_transcribing'),
                        style: TextStyle(
                          fontSize: 12,
                          color: ref.watch(recordingProvider) == RecordingState.recording
                              ? MemoTheme.red
                              : MemoTheme.accent,
                        ),
                      ),
                    ],
                  ),
                ),
              const SizedBox(width: 8),

              // ─── Text Input ──────────────────────────
              Expanded(
                child: Container(
                  constraints: const BoxConstraints(maxHeight: 160),
                  decoration: BoxDecoration(
                    color: MemoTheme.of(context).bgPanel,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: MemoTheme.of(context).borderSoft),
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
                  color: isSending ? MemoTheme.red : MemoTheme.accent,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  child: InkWell(
                    onTap: isSending
                        ? () => ref
                              .read(messagesProvider.notifier)
                              .stopStreaming()
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
  final Color? iconColor;

  const _InputIconButton({
    required this.icon,
    required this.tooltip,
    required this.onTap,
    this.disabled = false,
    this.iconColor,
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
            color: iconColor ?? (disabled
                ? MemoTheme.of(context).textDim.withValues(alpha: 0.3)
                : MemoTheme.of(context).textDim),
          ),
        ),
      ),
    );
  }
}

// ─── Recording indicator dot ─────────────────────────────────────

class _RecordingDot extends StatefulWidget {
  @override
  State<_RecordingDot> createState() => _RecordingDotState();
}

class _RecordingDotState extends State<_RecordingDot>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 800),
    )..repeat(reverse: true);
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return FadeTransition(
      opacity: _ctrl,
      child: Container(
        width: 8,
        height: 8,
        decoration: const BoxDecoration(
          shape: BoxShape.circle,
          color: MemoTheme.red,
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
            ...options.map(
              (opt) => _ModelOptionTile(
                option: opt,
                isActive: opt.type == activeType,
                onTap: () => Navigator.of(context).pop(opt.type),
              ),
            ),
            const SizedBox(height: 16),
            TextButton.icon(
              onPressed: onOpenRouterOAuth,
              icon: const Text('🔑', style: TextStyle(fontSize: 16)),
              label: const Text('OpenRouter ile Giriş Yap'),
              style: TextButton.styleFrom(foregroundColor: MemoTheme.warningOrange),
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
        leading: SizedBox(
          width: 32,
          height: 32,
          child: option.type == 'local'
              ? Icon(Icons.computer_outlined, size: 24, color: MemoTheme.of(context).textMuted)
              : providerLogoWidget(option.type, size: 28),
        ),
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
      return ((a['name'] as String?) ?? '').compareTo(
        (b['name'] as String?) ?? '',
      );
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
        return ((a['name'] as String?) ?? '').compareTo(
          (b['name'] as String?) ?? '',
        );
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
                final promptPrice =
                    (m['prompt_price'] as num?)?.toDouble() ?? 0;
                final ctxLen = m['context_length'] as int?;

                return Card(
                  margin: const EdgeInsets.only(bottom: 4),
                  child: ListTile(
                    dense: true,
                    leading: Icon(
                      isFree ? Icons.check_circle : Icons.monetization_on,
                      size: 18,
                      color: isFree ? MemoTheme.green : MemoTheme.warningOrange,
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
                        if (ctxLen != null && ctxLen > 0) _contextStr(ctxLen),
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
