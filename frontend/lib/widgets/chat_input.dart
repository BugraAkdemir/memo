import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';
import 'package:path/path.dart' as p;

import '../core/l10n.dart';
import '../core/theme.dart';
import 'glass_surface.dart';
import '../models/agent.dart';
import '../models/chat.dart';
import '../models/cli_command.dart';
import '../models/provider_config.dart';
import '../providers/chat_provider.dart';
import '../providers/models_provider.dart';
import '../providers/orchestra_provider.dart';
import '../providers/provider_provider.dart';
import '../providers/live_realtime_session_provider.dart';
import '../providers/recording_provider.dart';
import '../providers/settings_provider.dart';
import '../providers/skill_provider.dart';
import '../providers/tasklist_provider.dart';
import '../providers/voice_mode_provider.dart';
import '../providers/whatsapp_provider.dart';
import 'orchestra_config_dialog.dart';
import 'prompt_templates.dart';
import 'skill_config_dialog.dart';
import '../core/friendly_error.dart';

// Intents for the "/" and "@" popups' keyboard navigation. Bound via a
// Shortcuts widget wrapping the composer's TextField — see the Shortcuts/
// Actions nesting in _ChatInputState.build() for why a plain
// Focus.onKeyEvent wrapper (the pre-existing approach for "Enter sends")
// doesn't work for arrow keys: EditableText's own internal handling
// consumes ArrowUp/ArrowDown for cursor movement before an ancestor
// Focus.onKeyEvent would ever see them. Shortcuts+Actions is the same
// mechanism Flutter's own RawAutocomplete uses internally to make exactly
// this case work.
class _PopupArrowDownIntent extends Intent {
  const _PopupArrowDownIntent();
}

class _PopupArrowUpIntent extends Intent {
  const _PopupArrowUpIntent();
}

/// Plain Enter: confirm the popup if one's open, otherwise send the message.
class _PopupEnterIntent extends Intent {
  const _PopupEnterIntent();
}

/// Tab: confirm the popup. Only bound/enabled while a popup is open — falls
/// through to normal focus-traversal behavior otherwise.
class _PopupConfirmIntent extends Intent {
  const _PopupConfirmIntent();
}

class _PopupDismissIntent extends Intent {
  const _PopupDismissIntent();
}

/// A CallbackAction whose isEnabled is re-evaluated at invoke time rather
/// than fixed at construction — lets the SAME Shortcuts binding (e.g.
/// ArrowDown) do nothing and fall through to the TextField's default
/// behavior when no popup is open, and drive popup navigation when one is,
/// without swapping the shortcuts/actions maps themselves.
class _PopupCallbackAction<T extends Intent> extends CallbackAction<T> {
  final bool Function() isEnabledWhen;
  _PopupCallbackAction({required this.isEnabledWhen, required super.onInvoke});

  @override
  bool isEnabled(T intent, [BuildContext? context]) => isEnabledWhen();
}

class ChatInput extends ConsumerStatefulWidget {
  const ChatInput({super.key});

  @override
  ConsumerState<ChatInput> createState() => _ChatInputState();
}

/// Composer width below which the input bar stacks (text field on its own
/// row, action + send buttons underneath) instead of laying everything out
/// in a single Row — see the LayoutBuilder in [_ChatInputState.build].
/// Deliberately larger than ChatScreen's own sidebar breakpoint: the
/// composer runs out of room while the sidebar is still inline, since it
/// only ever gets the chat pane's width, not the window's.
const double _composerStackBelowWidth = 460;

class _ChatInputState extends ConsumerState<ChatInput> {
  final _controller = TextEditingController();
  final _focusNode = FocusNode();

  // Short phrases that resume a paused Self-Driving task instead of being sent
  // as a chat message (v4.6.0 Faz F). Kept deliberately tight — a longer
  // sentence is treated as a real instruction (recorded as a resume note).
  static final _resumeWordRe = RegExp(
    r'^(devam|devam et|devam edelim|devam ettir|kaldığın yerden devam et|'
    r'devam edebilirsin|sürdür|continue|resume|go on|keep going)[.!]?$',
    caseSensitive: false,
  );
  bool _showTemplates = false;
  String _filterQuery = '';
  int _selectedIndex = 0;
  String? _pickedImagePath;
  bool _whatsappStopped = false;
  CancelToken? _whatsappCancelToken;

  // "@" file mention — unlike "/" templates (whole-message trigger), this
  // triggers on the current word wherever the cursor is, so "look at
  // @lib/foo.dart please" works mid-sentence too.
  bool _showFileMentions = false;
  String _fileMentionQuery = '';
  List<String> _fileMentionResults = [];
  int _fileMentionSelectedIndex = 0;
  int _fileMentionAtPos = -1;
  int _fileMentionRequestGen = 0;

  // "/" in a CLI chat lists that CLI's own commands (fetched from the
  // backend, which can read .claude/commands & .codex/prompts) instead of
  // Memo's prompt templates — a coding agent's configured workflows are what
  // "/" means there, and Memo's canned prompt text would be noise.
  List<CLICommand> _cliCommands = [];
  int _cliCommandsRequestGen = 0;
  String _cliCommandsLoadedFor = '';

  List<CLICommand> get _filteredCLICommands {
    if (_filterQuery.isEmpty) return _cliCommands;
    final q = _filterQuery.toLowerCase();
    return _cliCommands
        .where((c) =>
            c.name.toLowerCase().contains(q) ||
            c.description.toLowerCase().contains(q))
        .toList();
  }

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
    final wasShowing = _showTemplates;
    final newQuery = show ? text.substring(1) : '';
    if (show != _showTemplates || newQuery != _filterQuery) {
      setState(() {
        _showTemplates = show;
        _filterQuery = newQuery;
        _selectedIndex = 0;
      });
    }
    if (show) {
      // Refetch each time the popup opens, not once per chat: the backend's
      // list grows the first time the CLI actually runs (it reports its own
      // full command set on its init event), so a list cached from before
      // that would stay permanently short. Typing further characters just
      // filters what's already loaded.
      _ensureCLICommandsLoaded(refresh: !wasShowing);
    } else {
      _checkFileMentionTrigger(text);
    }
  }

  /// Loads the active CLI provider's slash commands. Not a CLI chat ->
  /// clears the list, so the dropdown falls back to Memo's own prompt
  /// templates. The previous list is deliberately left in place while a
  /// refresh is in flight, so reopening "/" never flashes empty.
  Future<void> _ensureCLICommandsLoaded({bool refresh = false}) async {
    final cliType = ref.read(activeChatCLIProviderProvider).valueOrNull ?? '';
    final chatId = ref.read(activeChatIdProvider).valueOrNull ?? '';
    final key = '$cliType|$chatId';
    if (!refresh && key == _cliCommandsLoadedFor) return;

    final gen = ++_cliCommandsRequestGen;
    final switchedChat = key != _cliCommandsLoadedFor;
    _cliCommandsLoadedFor = key;
    if (cliType.isEmpty) {
      if (_cliCommands.isNotEmpty) setState(() => _cliCommands = []);
      return;
    }
    // Another chat's commands are worse than none — drop them immediately
    // rather than showing them until the new list arrives.
    if (switchedChat && _cliCommands.isNotEmpty) {
      setState(() => _cliCommands = []);
    }
    try {
      final cmds = await ref.read(apiClientProvider).listCLICommands(cliType, chatId);
      if (!mounted || gen != _cliCommandsRequestGen) return;
      setState(() => _cliCommands = cmds);
    } catch (_) {
      if (!mounted || gen != _cliCommandsRequestGen) return;
      // A failed lookup must not strand the composer: an empty list renders
      // the popup's normal "no matching command" state, and typing the
      // command by hand still works — the CLI resolves it either way.
      setState(() => _cliCommands = []);
      _cliCommandsLoadedFor = '';
    }
  }

  /// True when "/" should list the CLI's commands rather than Memo's
  /// templates. Driven by the chat's CLI provider, not by whether the fetch
  /// happened to return anything, so a CLI chat with no custom commands
  /// still shows the CLI's (empty) list instead of silently falling back to
  /// prompt templates that would do nothing useful there.
  bool get _isCLIChat =>
      (ref.read(activeChatCLIProviderProvider).valueOrNull ?? '').isNotEmpty;

  /// Looks backward from the cursor to the nearest preceding whitespace (or
  /// start of text) for an "@" starting the current word. If found, fetches
  /// matching project files for the text typed after it.
  void _checkFileMentionTrigger(String text) {
    final cursor = _controller.selection.baseOffset;
    if (cursor < 0 || cursor > text.length) {
      if (_showFileMentions) setState(() => _showFileMentions = false);
      return;
    }
    final upToCursor = text.substring(0, cursor);
    final atIndex = upToCursor.lastIndexOf('@');
    final validStart = atIndex == 0 ||
        (atIndex > 0 && RegExp(r'\s').hasMatch(upToCursor[atIndex - 1]));
    final hasSpaceAfterAt =
        atIndex >= 0 && RegExp(r'\s').hasMatch(upToCursor.substring(atIndex + 1));

    if (atIndex < 0 || !validStart || hasSpaceAfterAt) {
      if (_showFileMentions) setState(() => _showFileMentions = false);
      return;
    }

    final query = upToCursor.substring(atIndex + 1);
    _fileMentionAtPos = atIndex;
    if (query != _fileMentionQuery || !_showFileMentions) {
      _fetchFileMentions(query);
    }
  }

  Future<String> _resolveProjectRoot() async {
    final cliWorkdir = await ref.read(activeChatCLIWorkdirProvider.future);
    if (cliWorkdir.isNotEmpty) return cliWorkdir;
    final chatId = ref.read(activeChatIdProvider).valueOrNull;
    final chats = ref.read(chatListProvider).valueOrNull ?? const [];
    return chats.where((c) => c.id == chatId).firstOrNull?.projectPath ?? '';
  }

  Future<void> _fetchFileMentions(String query) async {
    final gen = ++_fileMentionRequestGen;
    setState(() {
      _showFileMentions = true;
      _fileMentionQuery = query;
      _fileMentionSelectedIndex = 0;
    });
    final root = await _resolveProjectRoot();
    if (!mounted || gen != _fileMentionRequestGen) return;
    if (root.isEmpty) {
      setState(() => _fileMentionResults = []);
      return;
    }
    try {
      final results = await ref.read(apiClientProvider).listProjectFiles(root, query);
      if (!mounted || gen != _fileMentionRequestGen) return;
      setState(() => _fileMentionResults = results);
    } catch (_) {
      if (mounted && gen == _fileMentionRequestGen) {
        setState(() => _fileMentionResults = []);
      }
    }
  }

  void _dismissFileMentions() {
    setState(() => _showFileMentions = false);
  }

  void _selectFileMentionAt(int index) {
    if (index < 0 || index >= _fileMentionResults.length) return;
    final path = _fileMentionResults[index];
    final text = _controller.text;
    final cursor = _controller.selection.baseOffset;
    if (_fileMentionAtPos < 0 || _fileMentionAtPos > text.length || cursor < 0 || cursor > text.length) {
      _dismissFileMentions();
      return;
    }
    final before = text.substring(0, _fileMentionAtPos);
    final after = text.substring(cursor);
    final inserted = '@$path ';
    _controller.text = before + inserted + after;
    _controller.selection = TextSelection.collapsed(offset: before.length + inserted.length);
    setState(() => _showFileMentions = false);
    _focusNode.requestFocus();
  }

  @override
  void dispose() {
    _controller.removeListener(_onTextChanged);
    _controller.dispose();
    _focusNode.dispose();
    _whatsappCancelToken?.cancel();
    super.dispose();
  }

  void _dismissPopup() {
    setState(() => _showTemplates = false);
  }

  /// Inserts the picked CLI command into the composer without sending, so
  /// arguments can still be typed after it ("/review the auth code").
  void _selectCLICommandAt(int index) {
    final items = _filteredCLICommands;
    if (index < 0 || index >= items.length) return;
    final inserted = '${items[index].slash} ';
    setState(() => _showTemplates = false);
    _controller.text = inserted;
    _controller.selection = TextSelection.collapsed(offset: inserted.length);
    _focusNode.requestFocus();
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

  // Popup keyboard navigation (arrows/enter/tab/escape while the "/" or "@"
  // dropdown is open) is NOT handled through Focus.onKeyEvent below — a
  // plain Focus wrapping the TextField only sees keys EditableText's own
  // internal handling didn't already consume, and ArrowUp/ArrowDown are
  // consumed there directly for cursor movement, so an ancestor
  // Focus.onKeyEvent never even fires for them (confirmed: this is why the
  // popups' arrow-key navigation silently did nothing). The one proven
  // mechanism that actually wins against EditableText's own key handling is
  // the same one Flutter's own RawAutocomplete uses internally: a Shortcuts
  // widget mapping the key to an Intent, handled by an Actions widget whose
  // Action.isEnabled gates whether it's actually consumed — see the
  // Shortcuts/Actions wrapping in build() below. isEnabled() returning
  // false (no popup open) correctly lets the key fall through to
  // EditableText's normal behavior instead of swallowing it.
  bool get _popupActive => _showFileMentions || _showTemplates;

  void _movePopupSelection(int delta) {
    if (_showFileMentions) {
      final itemCount = _fileMentionResults.length;
      if (itemCount == 0) return;
      setState(() => _fileMentionSelectedIndex =
          (_fileMentionSelectedIndex + delta + itemCount) % itemCount);
    } else if (_showTemplates) {
      final itemCount = _isCLIChat ? _filteredCLICommands.length : _filteredItems.length;
      if (itemCount == 0) return;
      setState(() => _selectedIndex = (_selectedIndex + delta + itemCount) % itemCount);
    }
  }

  void _confirmPopupSelection() {
    if (_showFileMentions) {
      _selectFileMentionAt(_fileMentionSelectedIndex);
    } else if (_showTemplates) {
      if (_isCLIChat) {
        _selectCLICommandAt(_selectedIndex);
      } else {
        _selectAtIndex(_selectedIndex);
      }
    }
  }

  void _dismissAnyPopup() {
    if (_showFileMentions) {
      _dismissFileMentions();
    } else if (_showTemplates) {
      _dismissPopup();
    }
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

    // v4.6.0 Faz F: a Self-Driving task bound to this chat.
    final sendChatId = ref.read(activeChatIdProvider).valueOrNull ?? '';
    final sendChatTask =
        sendChatId.isEmpty ? null : ref.read(chatTaskForProvider(sendChatId));
    if (sendChatTask != null) {
      if (sendChatTask.running || sendChatTask.awaitingPlan) {
        // Composer is disabled in this state; guard the code path too.
        return;
      }
      if (sendChatTask.paused && _resumeWordRe.hasMatch(text)) {
        _controller.clear();
        _dismissPopup();
        await ref
            .read(runningTasksProvider.notifier)
            .resume(sendChatTask.listId);
        return;
      }
      // A paused task otherwise lets the message through as a side question —
      // and also records it so the coder sees it when the task resumes.
      if (sendChatTask.paused && text.isNotEmpty) {
        // Fire-and-forget; a failed note must not block the chat message.
        // ignore: unawaited_futures
        ref.read(apiClientProvider).addTaskNote(sendChatTask.listId, text);
      }
    }

    if (ref.read(isSendingProvider)) return;

    // WhatsApp chat mode — route to WhatsApp stream when active
    if (ref.read(whatsAppChatModeProvider)) {
      _controller.clear();
      _dismissPopup();
      setState(() => _pickedImagePath = null);
      await _sendWhatsApp(text);
      return;
    }

    // A native Live Mode session (Google Live/OpenAI Realtime) is
    // connected — route typed text into the open session instead of the
    // normal sendMessage() path. Requested by the user: something they'd
    // rather not say out loud (a code snippet, a long piece of text)
    // during a live conversation should still reach the live model.
    final liveStatus = ref.read(liveRealtimeSessionProvider).status;
    if (imagePath == null &&
        (liveStatus == LiveRealtimeSessionStatus.connecting || liveStatus == LiveRealtimeSessionStatus.connected)) {
      _controller.clear();
      _dismissPopup();
      _sendToLiveMode(text);
      return;
    }

    // Neither a local model nor an API provider is active — sending would
    // just come back as a raw backend error ("⚠️ Yerel model yüklenmemiş...")
    // that used to surface as a generic, unfriendly snackbar (BUG: the actual
    // backend message got re-wrapped by messages_notifier's catch block into
    // "Mesaj gönderilemedi (...)"). Catch it before it ever leaves the client
    // and show clear next steps instead — the input text is preserved so
    // nothing is lost.
    if (!_hasActiveModel()) {
      _showNoModelGuide();
      return;
    }

    _controller.clear();
    _dismissPopup();
    setState(() => _pickedImagePath = null);

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
        ).showSnackBar(SnackBar(content: Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}')));
      }
    }

    if (!mounted) return;
    _focusNode.requestFocus();
  }

  /// Routes typed [text] into an open native Live Mode session
  /// (liveRealtimeSessionProvider.injectText) instead of the normal
  /// sendMessage() path — see _send()'s branch above. Mirrors a spoken
  /// utterance's own display+persistence exactly (ChatMessage via
  /// messagesProvider.addMessage(), then apiClientProvider.appendMessage()
  /// to actually survive a chat switch/app restart — see
  /// live_realtime_session_provider.dart's _handleControlFrame, the same
  /// two calls in the same order) so a typed aside looks and behaves
  /// identically to something the user said out loud.
  void _sendToLiveMode(String text) {
    final timestamp = DateTime.now().toIso8601String();
    ref.read(messagesProvider.notifier).addMessage(ChatMessage(role: 'user', content: text, timestamp: timestamp));
    unawaited(
      ref.read(apiClientProvider).appendMessage('user', text).catchError((Object e) {
        debugPrint('live mode text: failed to persist message: $e');
      }),
    );
    ref.read(liveRealtimeSessionProvider.notifier).injectText(text);
  }

  Future<void> _sendWhatsApp(String text) async {
    final api = ref.read(apiClientProvider);
    final timestamp = DateTime.now().toIso8601String().substring(11, 19);
    final userMsg = ChatMessage(
      role: 'user',
      content: text,
      timestamp: timestamp,
    );

    _whatsappStopped = false;
    _whatsappCancelToken?.cancel();
    _whatsappCancelToken = CancelToken();

    if (!mounted) return;
    ref.read(isSendingProvider.notifier).state = true;
    ref.read(messagesProvider.notifier).addMessage(userMsg);

    ref.read(streamingContentProvider.notifier).state = '';
    ref.read(streamingAgentEventsProvider.notifier).state = [];

    List<AgentEvent> finalAgentEvents = [];
    String accumulatedContent = '';
    try {
      await for (final chunk in api
          .sendWhatsAppChatStream(
            text,
            cancelToken: _whatsappCancelToken,
          )
          .timeout(
            const Duration(seconds: 300),
            onTimeout: (sink) => sink.addError(Exception(
              L10n.t('whatsapp_timeout'),
            )),
          )) {
        if (chunk.finishReason == 'agent_event') {
          try {
            final decoded = json.decode(chunk.content);
            if (decoded is! Map<String, dynamic>) continue;
            final ev = AgentEvent.fromJson(decoded);
            final events = [...ref.read(streamingAgentEventsProvider)];
            if (ev.type == 'tool_executing' ||
                ev.type == 'tool_result' ||
                ev.type == 'tool_error') {
              if (events.isNotEmpty && events.last.type == 'tool_executing') {
                events[events.length - 1] = ev;
              } else {
                events.add(ev);
              }
              ref.read(streamingAgentEventsProvider.notifier).state = events;
              finalAgentEvents = events;
            }
          } catch (_) {
            // ignore malformed event payloads
          }
        } else {
          accumulatedContent += chunk.content;
          ref.read(streamingContentProvider.notifier).state = accumulatedContent;
        }
      }

      if (_whatsappStopped) {
        _whatsappStopped = false;
        ref.read(streamingContentProvider.notifier).state = '';
        ref.read(streamingAgentEventsProvider.notifier).state = [];
        return;
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('whatsapp_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
    } finally {
      final stopped = _whatsappStopped;
      _whatsappStopped = false;
      _whatsappCancelToken?.cancel();
      _whatsappCancelToken = null;

      if (!stopped) {
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
      }
      ref.read(streamingContentProvider.notifier).state = '';
      ref.read(streamingAgentEventsProvider.notifier).state = [];
      ref.read(isSendingProvider.notifier).state = false;
      ref.read(messagesProvider.notifier).refresh();
    }
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

  /// Whether a chat request would actually have somewhere to go: either an
  /// API provider is active, or the local llama.cpp server is up. Read from
  /// already-cached provider state (EngineStrip keeps both providers alive
  /// at the app-shell level) — no network round trip.
  bool _hasActiveModel() {
    final activeProviderType = ref.read(activeProviderTypeProvider).valueOrNull ?? '';
    if (activeProviderType.isNotEmpty) return true;
    if ((ref.read(activeChatCLIProviderProvider).valueOrNull ?? '').isNotEmpty) return true;
    return ref.read(modelStatusProvider).valueOrNull?.running ?? false;
  }

  void _showNoModelGuide() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('no_model_guide_title')),
        content: Text(L10n.t('no_model_guide_body')),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(L10n.t('cancel')),
          ),
          FilledButton(
            onPressed: () {
              Navigator.of(ctx).pop();
              _showModelSwitcher();
            },
            child: Text(L10n.t('choose_model_action')),
          ),
        ],
      ),
    );
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
            Text(
              L10n.t('openrouter_key_instructions'),
              style: const TextStyle(fontSize: 13),
            ),
            const SizedBox(height: 4),
            Text(
              L10n.t('openrouter_key_hint'),
              style: const TextStyle(fontSize: 12, color: Colors.grey),
            ),
            const SizedBox(height: 16),
            TextField(
              controller: keyController,
              decoration: InputDecoration(
                labelText: L10n.t('api_key'),
                hintText: 'sk-or-...',
                border: const OutlineInputBorder(),
              ),
              obscureText: true,
              autofocus: true,
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(L10n.t('cancel')),
          ),
          FilledButton(
            onPressed: () {
              final key = keyController.text.trim();
              if (key.isEmpty) return;
              Navigator.of(ctx).pop(key);
            },
            child: Text(L10n.t('continue_btn')),
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
      builder: (_) => Center(
        child: Card(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const CircularProgressIndicator(),
                const SizedBox(height: 16),
                Text(L10n.t('models_loading')),
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
              content: Text('❌ ${result['error'] ?? L10n.t('models_fetch_error_short')}'),
            ),
          );
        }
        return;
      }
      final rawModels = result['models'];
      models = (rawModels is List) ? rawModels.cast<Map<String, dynamic>>() : <Map<String, dynamic>>[];
    } catch (e) {
      if (mounted) {
        Navigator.of(context).pop();
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('error_with_detail', {'e': FriendlyError.describeGeneric(e)}))),
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
          SnackBar(
            content: Text(L10n.t('openrouter_connected')),
            backgroundColor: MemoTheme.green,
          ),
        );
        ref.invalidate(providerListProvider);
        ref.invalidate(activeProviderTypeProvider);
      } else if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('❌ ${result['error'] ?? L10n.t('connection_error')}'),
            backgroundColor: MemoTheme.red,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('error_with_detail', {'e': FriendlyError.describeGeneric(e)}))),
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

    // v4.6.0 Faz F: a Self-Driving task running in the active chat owns the
    // composer. The user stops it (red button -> pause) to ask a question,
    // then types "devam" to continue. A paused task leaves the input open.
    final sdChatId = ref.watch(activeChatIdProvider).valueOrNull ?? '';
    final sdTask = sdChatId.isEmpty
        ? null
        : ref.watch(chatTaskForProvider(sdChatId));
    final taskLocksInput =
        sdTask != null && (sdTask.running || sdTask.awaitingPlan);

    ref.listen(activeChatIdProvider, (prev, next) {
      final prevId = prev?.valueOrNull ?? '';
      final nextId = next.valueOrNull ?? '';
      if (prevId != nextId) {
        _controller.clear();
      }
    });

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

    final popup = _showFileMentions
        ? Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: _FileMentionPopup(
              results: _fileMentionResults,
              selectedIndex: _fileMentionSelectedIndex,
              onSelect: _selectFileMentionAt,
              onDismiss: _dismissFileMentions,
            ),
          )
        : _showTemplates
            ? Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: _isCLIChat
                    ? _CLICommandPopup(
                        commands: _filteredCLICommands,
                        selectedIndex: _selectedIndex,
                        onSelect: _selectCLICommandAt,
                      )
                    : PromptTemplatesPopup(
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
        // A plain Column child, not the previous SizedBox(height:0) +
        // OverflowBox + Stack "float without affecting layout" trick — that
        // construct made the popup untouchable by ANY pointer input (arrow
        // keys, mouse wheel scroll, even taps), reported live: neither
        // keyboard navigation nor scrolling worked at all. Zero-height
        // overflow-painted content sits outside the normal hit-test
        // geometry Flutter expects, so nothing reached it. This trades a
        // few pixels of the message list shifting up while a popup is open
        // for interaction that is guaranteed to actually work.
        if (popup != null) popup,
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
                    p.basename(_pickedImagePath!),
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
              ? const EdgeInsets.fromLTRB(12, 8, 12, 12)
              : EdgeInsets.zero,
          child: GlassBlur(
          borderRadius: MemoTheme.of(context).isGlass
              ? BorderRadius.circular(MemoTheme.radiusLg)
              : BorderRadius.zero,
          child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: MemoTheme.of(context).isGlass
              ? BoxDecoration(
                  color: MemoTheme.of(context).bgPanel,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
                  border: Border.all(color: MemoTheme.of(context).borderSoft),
                  boxShadow: MemoTheme.shadowLg,
                )
              : BoxDecoration(
                  color: MemoTheme.of(context).bgApp,
                  border: Border(
                    top: BorderSide(color: MemoTheme.of(context).borderSoft),
                  ),
                ),
          child: LayoutBuilder(
            builder: (context, constraints) {
            // The fixed-width children of this bar (4-6 icon buttons, their
            // gaps, and the 42px send button) take ~225px regardless of how
            // much width is available, so under a narrow viewport the
            // Expanded text field below is left with almost nothing and its
            // hint wraps one character per line — reported live as "there's
            // no text box" at phone width. Below the breakpoint, stack
            // instead: the field gets its own full-width row, with the
            // action buttons and send button on a second row underneath.
            final narrow = constraints.maxWidth < _composerStackBelowWidth;
            final actions = <Widget>[
              _InputIconButton(
                icon: Icons.image_outlined,
                tooltip: orchestraEnabled
                    ? L10n.t('orchestra_not_available')
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
                    ? L10n.t('orchestra_not_available')
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
                          SnackBar(content: Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}')),
                        );
                      }
                    }
                  }
                },
              ),
              // Orchestra quick toggle
              _InputIconButton(
                icon: orchestraEnabled ? Icons.queue_music : Icons.queue_music_outlined,
                tooltip: orchestraEnabled
                    ? L10n.t('orchestra_toggle_on')
                    : L10n.t('orchestra_toggle_off'),
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
                          SnackBar(
                            content: Text(L10n.t('orchestra_enable_failed', {'e': FriendlyError.describeGeneric(e)})),
                          ),
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
              // Voice mode toggle: continuous listen → send → speak-reply,
              // cross-modal with typing (either way still works while this
              // is on). Separate from the mic button above, which is a
              // single push-to-talk-into-the-text-field capture. Gated by
              // Live Mode's own Enabled toggle (see docs/plans/
              // PLAN_live_mode_v2.md) — graduated out of Beta, so this no
              // longer depends on betaFeaturesProvider.
              if (ref.watch(liveModeConfigProvider).valueOrNull?.enabled ??
                  false) ...[
                const SizedBox(width: 4),
                // Which engine is active decides which voice loop the
                // toggle button drives: google_live/openai_realtime are
                // full-duplex native sessions (liveRealtimeSessionProvider,
                // real mic streaming + playback — see
                // live_realtime_session_provider.dart); everything else
                // (local/elevenlabs/custom) stays on the discrete
                // VAD -> transcribe -> chat -> synthesize loop
                // (voiceModeProvider) unchanged. Found via real-world
                // testing that this branch was missing entirely — the
                // button always used voiceModeProvider regardless of the
                // selected engine, so native engines silently fell back to
                // local whisper.cpp transcription instead of ever opening
                // their own session.
                if (const {'google_live', 'openai_realtime'}
                    .contains(ref.watch(liveModeConfigProvider).valueOrNull?.activeEngine))
                  ..._buildRealtimeVoiceControls(ref)
                else
                  ..._buildDiscreteVoiceControls(ref),
              ],
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
            ];

            // ─── Text Input ──────────────────────────
            final field = Container(
                  constraints: const BoxConstraints(maxHeight: 160),
                  decoration: BoxDecoration(
                    color: MemoTheme.of(context).bgPanel,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: MemoTheme.of(context).borderSoft),
                  ),
                  child: Shortcuts(
                    shortcuts: const {
                      SingleActivator(LogicalKeyboardKey.arrowDown): _PopupArrowDownIntent(),
                      SingleActivator(LogicalKeyboardKey.arrowUp): _PopupArrowUpIntent(),
                      // Plain Enter only — SingleActivator defaults every
                      // modifier to false, so Shift+Enter (insert newline)
                      // isn't matched here and falls through to
                      // EditableText's own default behavior untouched.
                      SingleActivator(LogicalKeyboardKey.enter): _PopupEnterIntent(),
                      SingleActivator(LogicalKeyboardKey.tab): _PopupConfirmIntent(),
                      SingleActivator(LogicalKeyboardKey.escape): _PopupDismissIntent(),
                    },
                    child: Actions(
                      actions: {
                        _PopupArrowDownIntent: _PopupCallbackAction<_PopupArrowDownIntent>(
                          isEnabledWhen: () => _popupActive,
                          onInvoke: (_) => _movePopupSelection(1),
                        ),
                        _PopupArrowUpIntent: _PopupCallbackAction<_PopupArrowUpIntent>(
                          isEnabledWhen: () => _popupActive,
                          onInvoke: (_) => _movePopupSelection(-1),
                        ),
                        // Always enabled (unlike the others): plain Enter
                        // always does SOMETHING — confirm the popup if one
                        // is open, otherwise send the message. Previously
                        // "Enter sends" lived in a separate inner
                        // Focus.onKeyEvent; folded in here instead of
                        // leaving two different key-handling mechanisms
                        // both racing to claim the same key.
                        _PopupEnterIntent: CallbackAction<_PopupEnterIntent>(
                          onInvoke: (_) {
                            if (_popupActive) {
                              _confirmPopupSelection();
                            } else {
                              _send();
                            }
                            return null;
                          },
                        ),
                        _PopupConfirmIntent: _PopupCallbackAction<_PopupConfirmIntent>(
                          isEnabledWhen: () => _popupActive,
                          onInvoke: (_) => _confirmPopupSelection(),
                        ),
                        _PopupDismissIntent: _PopupCallbackAction<_PopupDismissIntent>(
                          isEnabledWhen: () => _popupActive,
                          onInvoke: (_) => _dismissAnyPopup(),
                        ),
                      },
                      child: TextField(
                          controller: _controller,
                          focusNode: _focusNode,
                          enabled: !taskLocksInput,
                          maxLines: null,
                          textInputAction: TextInputAction.newline,
                          style: TextStyle(
                            fontSize: 14,
                            color: MemoTheme.of(context).textMain,
                            height: 1.5,
                          ),
                          decoration: InputDecoration(
                            hintText: taskLocksInput
                                ? L10n.t('task_running_hint')
                                : '${L10n.t('type_message')} (/)',
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
                  );

            // ─── Send / Stop / Pause-task Button ──────
            final showStop = isSending || taskLocksInput;
            final send = AnimatedContainer(
                duration: const Duration(milliseconds: 150),
                child: Material(
                  color: showStop ? MemoTheme.red : MemoTheme.accent,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  child: InkWell(
                    onTap: isSending
                        ? () {
                              _whatsappStopped = true;
                              _whatsappCancelToken?.cancel();
                              ref
                                  .read(messagesProvider.notifier)
                                  .stopStreaming();
                            }
                        : taskLocksInput
                            ? () {
                                ref
                                    .read(runningTasksProvider.notifier)
                                    .pause(sdTask.listId);
                              }
                            : _send,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                    child: SizedBox(
                      width: 42,
                      height: 42,
                      child: showStop
                          ? Icon(
                              taskLocksInput && !isSending
                                  ? Icons.pause_rounded
                                  : Icons.stop_rounded,
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
              );

            if (narrow) {
              return Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  field,
                  const SizedBox(height: 8),
                  Row(
                    children: [
                      // Scrollable rather than a plain Row: with Beta on
                      // (voice button) plus a recording/transcribing status
                      // label, the action set can still outgrow a phone
                      // width even on its own row.
                      Expanded(
                        child: SingleChildScrollView(
                          scrollDirection: Axis.horizontal,
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            crossAxisAlignment: CrossAxisAlignment.center,
                            children: actions,
                          ),
                        ),
                      ),
                      const SizedBox(width: 12),
                      send,
                    ],
                  ),
                ],
              );
            }

            return Row(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                ...actions,
                const SizedBox(width: 8),
                Expanded(child: field),
                const SizedBox(width: 12),
                send,
              ],
            );
            },
          ),
          ),
        ),
        ),
      ],
    );
  }

  /// Local/ElevenLabs/Custom's discrete VAD -> transcribe -> chat ->
  /// synthesize loop — unchanged behavior, just extracted out of build() so
  /// it sits alongside its native-engine sibling below.
  List<Widget> _buildDiscreteVoiceControls(WidgetRef ref) {
    final voiceState = ref.watch(voiceModeProvider);
    final active = voiceState != VoiceModeState.idle;
    return [
      _InputIconButton(
        icon: active ? Icons.record_voice_over : Icons.record_voice_over_outlined,
        tooltip: active ? L10n.t('voice_mode_stop') : L10n.t('voice_mode_start'),
        disabled: false,
        iconColor: switch (voiceState) {
          VoiceModeState.idle => null,
          VoiceModeState.listening => MemoTheme.accent,
          VoiceModeState.thinking => MemoTheme.warningOrange,
          VoiceModeState.speaking => MemoTheme.accent,
        },
        onTap: () => ref.read(voiceModeProvider.notifier).toggle(),
      ),
      if (voiceState != VoiceModeState.idle)
        Padding(
          padding: const EdgeInsets.only(left: 8),
          child: Text(
            switch (voiceState) {
              VoiceModeState.idle => '',
              VoiceModeState.listening => L10n.t('live_screen_state_listening'),
              VoiceModeState.thinking => L10n.t('live_screen_state_thinking'),
              VoiceModeState.speaking => L10n.t('live_screen_state_speaking'),
            },
            style: TextStyle(fontSize: 12, color: MemoTheme.accent),
          ),
        ),
    ];
  }

  /// Google Live/OpenAI Realtime's full-duplex native session — real mic
  /// capture streamed continuously to the backend, real playback of
  /// whatever audio comes back (see live_realtime_session_provider.dart).
  /// Errors already reach the app-wide toast via the notifier's own
  /// _setError, so the label here only reflects connecting/connected —
  /// error/closed collapse back to the idle-looking start button rather
  /// than getting stuck showing a stale error label forever.
  List<Widget> _buildRealtimeVoiceControls(WidgetRef ref) {
    final session = ref.watch(liveRealtimeSessionProvider);
    final active = session.status == LiveRealtimeSessionStatus.connecting ||
        session.status == LiveRealtimeSessionStatus.connected;
    return [
      _InputIconButton(
        icon: active ? Icons.record_voice_over : Icons.record_voice_over_outlined,
        tooltip: active ? L10n.t('live_realtime_stop') : L10n.t('live_realtime_start'),
        disabled: session.status == LiveRealtimeSessionStatus.connecting,
        iconColor: switch (session.status) {
          LiveRealtimeSessionStatus.connected => MemoTheme.accent,
          LiveRealtimeSessionStatus.connecting => MemoTheme.warningOrange,
          _ => null,
        },
        onTap: () {
          final notifier = ref.read(liveRealtimeSessionProvider.notifier);
          if (active) {
            notifier.disconnect();
          } else {
            final engine = ref.read(liveModeConfigProvider).valueOrNull?.activeEngine ?? 'google_live';
            notifier.connect(engine);
          }
        },
      ),
      if (session.status == LiveRealtimeSessionStatus.connecting ||
          session.status == LiveRealtimeSessionStatus.connected)
        Padding(
          padding: const EdgeInsets.only(left: 8),
          child: Text(
            session.status == LiveRealtimeSessionStatus.connecting
                ? L10n.t('live_realtime_state_connecting')
                : L10n.t('live_realtime_state_connected'),
            style: TextStyle(fontSize: 12, color: MemoTheme.accent),
          ),
        ),
    ];
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
      title: Text(L10n.t('switch_model')),
      content: SizedBox(
        width: 320,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              L10n.t('switch_model_desc'),
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
              label: Text(L10n.t('login_openrouter')),
              style: TextButton.styleFrom(foregroundColor: MemoTheme.warningOrange),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(L10n.t('cancel')),
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
    if (p == 0) return L10n.t('free');
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
                  L10n.t('free_paid_legend'),
                  style: TextStyle(fontSize: 11, color: Colors.grey[500]),
                ),
                const Spacer(),
                TextButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: Text(L10n.t('cancel')),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// "@" file-mention dropdown — same visual shape as [PromptTemplatesPopup]
/// (prompt_templates.dart), for a flat list of project-relative file paths
/// instead of slash templates. Empty when [ChatInputState._resolveProjectRoot]
/// has no folder to search (no CLI workdir, no agent project path).
/// "/" dropdown for a CLI chat: the CLI's own commands, with a source badge
/// (project / user / skill / builtin) so it's clear where each one comes
/// from — a project command and a same-named personal one are genuinely
/// different things, and knowing which is about to run matters when the
/// command can edit files and run shell commands.
class _CLICommandPopup extends StatelessWidget {
  final List<CLICommand> commands;
  final int selectedIndex;
  final ValueChanged<int> onSelect;

  const _CLICommandPopup({
    required this.commands,
    required this.selectedIndex,
    required this.onSelect,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    if (commands.isEmpty) {
      return Container(
        margin: const EdgeInsets.symmetric(horizontal: 16),
        decoration: BoxDecoration(
          color: theme.bgApp,
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          border: Border.all(color: theme.borderSoft),
          boxShadow: MemoTheme.shadowMd,
        ),
        padding: const EdgeInsets.all(16),
        child: Text(
          L10n.t('cli_command_none'),
          style: TextStyle(fontSize: 13, color: theme.textDim, height: 1.4),
        ),
      );
    }

    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 16),
      constraints: const BoxConstraints(maxHeight: 280),
      decoration: BoxDecoration(
        color: theme.bgApp,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: theme.borderSoft),
        boxShadow: MemoTheme.shadowMd,
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        child: ListView.builder(
          padding: const EdgeInsets.symmetric(vertical: 4),
          shrinkWrap: true,
          itemCount: commands.length,
          itemBuilder: (context, index) {
            final cmd = commands[index];
            final isSelected = index == selectedIndex;
            return InkWell(
              onTap: () => onSelect(index),
              child: Container(
                color: isSelected ? theme.bgElement : Colors.transparent,
                padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 9),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(Icons.terminal_rounded,
                        size: 15,
                        color: isSelected ? MemoTheme.accent : theme.textDim),
                    const SizedBox(width: 9),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              Flexible(
                                child: Text(
                                  cmd.slash,
                                  style: TextStyle(
                                    fontSize: 13,
                                    fontWeight: FontWeight.w600,
                                    color: theme.textMain,
                                  ),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ),
                              if (cmd.source.isNotEmpty) ...[
                                const SizedBox(width: 7),
                                _CLICommandSourceBadge(source: cmd.source),
                              ],
                            ],
                          ),
                          if (cmd.description.isNotEmpty) ...[
                            const SizedBox(height: 2),
                            Text(
                              cmd.description,
                              style: TextStyle(fontSize: 11.5, color: theme.textDim),
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ],
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}

class _CLICommandSourceBadge extends StatelessWidget {
  final String source;

  const _CLICommandSourceBadge({required this.source});

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    // Keys are the backend's Source values; an unrecognized one falls back
    // to showing the raw value rather than vanishing.
    const labelKeys = {
      'project': 'cli_command_src_project',
      'user': 'cli_command_src_user',
      'skill': 'cli_command_src_skill',
      'builtin': 'cli_command_src_builtin',
    };
    final key = labelKeys[source];
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        key != null ? L10n.t(key) : source,
        style: TextStyle(
          fontSize: 9.5,
          fontWeight: FontWeight.w600,
          letterSpacing: 0.3,
          color: theme.textDim,
        ),
      ),
    );
  }
}

class _FileMentionPopup extends StatelessWidget {
  final List<String> results;
  final int selectedIndex;
  final ValueChanged<int> onSelect;
  final VoidCallback onDismiss;

  const _FileMentionPopup({
    required this.results,
    required this.selectedIndex,
    required this.onSelect,
    required this.onDismiss,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    if (results.isEmpty) {
      return Container(
        margin: const EdgeInsets.symmetric(horizontal: 16),
        constraints: const BoxConstraints(maxHeight: 80),
        decoration: BoxDecoration(
          color: theme.bgApp,
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          border: Border.all(color: theme.borderSoft),
          boxShadow: MemoTheme.shadowMd,
        ),
        padding: const EdgeInsets.all(16),
        child: Text(
          L10n.t('file_mention_none'),
          style: TextStyle(
            fontSize: 13,
            color: theme.textDim,
            fontStyle: FontStyle.italic,
          ),
        ),
      );
    }

    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 16),
      constraints: const BoxConstraints(maxHeight: 240),
      decoration: BoxDecoration(
        color: theme.bgApp,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: theme.borderSoft),
        boxShadow: MemoTheme.shadowMd,
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        child: ListView.builder(
          padding: const EdgeInsets.symmetric(vertical: 4),
          shrinkWrap: true,
          itemCount: results.length,
          itemBuilder: (context, index) {
            final path = results[index];
            final isSelected = index == selectedIndex;
            return InkWell(
              onTap: () => onSelect(index),
              child: Container(
                color: isSelected ? theme.bgElement : Colors.transparent,
                padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                child: Row(
                  children: [
                    Icon(Icons.insert_drive_file_outlined,
                        size: 15, color: theme.textDim),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        path,
                        style: TextStyle(fontSize: 13, color: theme.textMain),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}
