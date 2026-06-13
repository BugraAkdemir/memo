import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';
import '../providers/connection_provider.dart';
import '../widgets/branding.dart';
import '../widgets/chat_bubble.dart';
import '../widgets/message_input.dart';
import '../widgets/session_drawer.dart';

class ChatScreen extends ConsumerStatefulWidget {
  const ChatScreen({super.key});

  @override
  ConsumerState<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends ConsumerState<ChatScreen> {
  final _scroll = ScrollController();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(chatProvider.notifier).loadSessions();
    });
  }

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  void _toBottom({bool animate = true}) {
    if (!_scroll.hasClients) return;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scroll.hasClients) return;
      final target = _scroll.position.maxScrollExtent;
      if (animate) {
        _scroll.animateTo(target, duration: const Duration(milliseconds: 220), curve: Curves.easeOut);
      } else {
        _scroll.jumpTo(target);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(chatProvider);

    // Keep the conversation pinned to the latest message as it grows.
    ref.listen(chatProvider, (prev, next) {
      final grew = (next.messages.length != (prev?.messages.length ?? 0)) ||
          (next.currentStreamContent.length != (prev?.currentStreamContent.length ?? 0));
      if (grew) _toBottom();
    });

    final title = state.sessions
            .where((s) => s.id == state.activeSessionId)
            .firstOrNull
            ?.title ??
        'Memo';

    return Scaffold(
      backgroundColor: MemoTheme.bg,
      drawer: const SessionDrawer(),
      appBar: AppBar(
        titleSpacing: 4,
        leadingWidth: 52,
        leading: Builder(
          builder: (ctx) => IconButton(
            icon: const Icon(Icons.menu_rounded, color: MemoTheme.textDim),
            onPressed: () => Scaffold.of(ctx).openDrawer(),
          ),
        ),
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(title, maxLines: 1, overflow: TextOverflow.ellipsis, style: Theme.of(context).textTheme.titleLarge),
            const SizedBox(height: 2),
            const _EngineChip(),
          ],
        ),
        actions: [
          IconButton(
            tooltip: 'New chat',
            icon: const Icon(Icons.add_comment_outlined, size: 21, color: MemoTheme.textDim),
            onPressed: () {
              HapticFeedback.selectionClick();
              ref.read(chatProvider.notifier).newChat();
            },
          ),
          if (state.activeSessionId != null)
            PopupMenuButton<String>(
              icon: const Icon(Icons.more_vert_rounded, color: MemoTheme.textDim),
              color: MemoTheme.surface,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(14),
                side: const BorderSide(color: MemoTheme.border),
              ),
              onSelected: (v) async {
                if (v == 'delete') {
                  final ok = await _confirmDelete(context);
                  if (ok == true && state.activeSessionId != null) {
                    ref.read(chatProvider.notifier).deleteChat(state.activeSessionId!);
                  }
                }
              },
              itemBuilder: (_) => [
                PopupMenuItem(
                  value: 'delete',
                  child: Row(children: [
                    const Icon(Icons.delete_outline_rounded, size: 19, color: MemoTheme.error),
                    const SizedBox(width: 12),
                    Text('Delete chat', style: MemoTheme.body(14, color: MemoTheme.error)),
                  ]),
                ),
              ],
            ),
          const SizedBox(width: 4),
        ],
      ),
      body: Column(
        children: [
          if (state.error != null) _errorBanner(state.error!),
          Expanded(
            child: state.loading
                ? const Center(child: CircularProgressIndicator(color: MemoTheme.accent))
                : state.activeSessionId == null
                    ? _EmptyState(onNew: () => ref.read(chatProvider.notifier).newChat())
                    : state.messages.isEmpty && !state.streaming
                        ? _Greeting(onPick: (t) {
                            ref.read(chatProvider.notifier).sendMessage(t);
                          })
                        : _messageList(state),
          ),
          if (state.activeSessionId != null)
            MessageInput(
              streaming: state.streaming,
              onSend: (t) => ref.read(chatProvider.notifier).sendMessage(t),
              onStop: () => ref.read(chatProvider.notifier).cancelStream(),
            ),
        ],
      ),
    );
  }

  Widget _messageList(ChatState state) {
    final showTyping = state.streaming && state.currentStreamContent.isEmpty;
    final showStream = state.streaming && state.currentStreamContent.isNotEmpty;
    final extra = (showTyping || showStream) ? 1 : 0;

    return ListView.builder(
      controller: _scroll,
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      itemCount: state.messages.length + extra,
      itemBuilder: (context, index) {
        if (index < state.messages.length) {
          final m = state.messages[index];
          return ChatBubble(role: m.role, content: m.content, thinking: m.thinking);
        }
        if (showStream) {
          return ChatBubble(role: 'assistant', content: state.currentStreamContent, streaming: true);
        }
        return const TypingIndicator();
      },
    );
  }

  Widget _errorBanner(String msg) {
    return Material(
      color: MemoTheme.error.withValues(alpha: 0.12),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 10, 8, 10),
        child: Row(
          children: [
            const Icon(Icons.error_outline_rounded, size: 17, color: MemoTheme.error),
            const SizedBox(width: 10),
            Expanded(child: Text(msg, style: MemoTheme.body(13, color: MemoTheme.error), maxLines: 3, overflow: TextOverflow.ellipsis)),
            IconButton(
              icon: const Icon(Icons.close_rounded, size: 17, color: MemoTheme.error),
              onPressed: () => ref.read(chatProvider.notifier).clearError(),
              visualDensity: VisualDensity.compact,
            ),
          ],
        ),
      ),
    );
  }

  Future<bool?> _confirmDelete(BuildContext context) {
    return showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete this chat?'),
        content: const Text('The conversation will be removed from your desktop. This cannot be undone.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: Text('Keep', style: MemoTheme.body(14, color: MemoTheme.textDim))),
          TextButton(onPressed: () => Navigator.pop(ctx, true), child: Text('Delete', style: MemoTheme.body(14, w: FontWeight.w600, color: MemoTheme.error))),
        ],
      ),
    );
  }
}

/// The signature: a live engine readout (active model + running dot) that
/// doubles as the model switcher. Echoes the desktop Engine Strip.
class _EngineChip extends ConsumerStatefulWidget {
  const _EngineChip();

  @override
  ConsumerState<_EngineChip> createState() => _EngineChipState();
}

class _EngineChipState extends ConsumerState<_EngineChip> {
  String _label = 'connecting…';
  bool _loaded = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final api = ref.read(apiClientProvider);
    try {
      final active = await api.getActiveProvider();
      String label;
      if (active.isEmpty) {
        label = 'local model';
      } else {
        final providers = await api.getProviders();
        label = providers
            .firstWhere((p) => p.type == active, orElse: () => ProviderConfig(type: active, name: active, model: ''))
            .name;
      }
      if (mounted) setState(() { _label = label; _loaded = true; });
    } catch (_) {
      if (mounted) setState(() { _label = 'offline'; _loaded = true; });
    }
  }

  @override
  Widget build(BuildContext context) {
    return StatPill(
      padding: const EdgeInsets.fromLTRB(8, 5, 9, 5),
      onTap: () async {
        await showModelSwitcher(context, ref);
        _load();
      },
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          StatusDot(color: _loaded ? MemoTheme.sage : MemoTheme.textFaint, size: 6, pulse: _loaded),
          const SizedBox(width: 7),
          Text(_label, style: MemoTheme.mono(11, color: MemoTheme.textDim)),
          const SizedBox(width: 4),
          const Icon(Icons.unfold_more_rounded, size: 13, color: MemoTheme.textFaint),
        ],
      ),
    );
  }
}

class _EmptyState extends StatelessWidget {
  final VoidCallback onNew;
  const _EmptyState({required this.onNew});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const MemoLogo(size: 60),
            const SizedBox(height: 20),
            Text('No chat open', style: Theme.of(context).textTheme.headlineMedium),
            const SizedBox(height: 8),
            Text('Start a new conversation, or pick one from the menu.',
                textAlign: TextAlign.center, style: MemoTheme.body(14, color: MemoTheme.textDim, height: 1.5)),
            const SizedBox(height: 24),
            FilledButton.icon(
              onPressed: onNew,
              icon: const Icon(Icons.add_rounded, size: 19),
              label: const Text('New chat'),
              style: FilledButton.styleFrom(
                backgroundColor: MemoTheme.accent,
                foregroundColor: MemoTheme.onAccent,
                padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 13),
                textStyle: MemoTheme.body(15, w: FontWeight.w600),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(13)),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Greeting extends StatelessWidget {
  final void Function(String prompt) onPick;
  const _Greeting({required this.onPick});

  String get _timeGreeting {
    final h = DateTime.now().hour;
    if (h < 12) return 'Good morning';
    if (h < 18) return 'Good afternoon';
    return 'Good evening';
  }

  @override
  Widget build(BuildContext context) {
    const prompts = [
      ('Summarise what I worked on', Icons.history_rounded),
      ('Draft a quick reply', Icons.edit_outlined),
      ('Explain this in simple terms', Icons.lightbulb_outline_rounded),
      ('What did I decide about…', Icons.psychology_outlined),
    ];
    return ListView(
      padding: const EdgeInsets.fromLTRB(24, 0, 24, 24),
      children: [
        const SizedBox(height: 60),
        const Center(child: MemoLogo(size: 56)),
        const SizedBox(height: 24),
        Text(_timeGreeting, textAlign: TextAlign.center, style: Theme.of(context).textTheme.headlineLarge),
        const SizedBox(height: 6),
        Text('Ask anything — Memo remembers the rest.',
            textAlign: TextAlign.center, style: MemoTheme.body(14.5, color: MemoTheme.textDim)),
        const SizedBox(height: 30),
        ...prompts.map((p) => Padding(
              padding: const EdgeInsets.only(bottom: 10),
              child: Material(
                color: MemoTheme.surface,
                borderRadius: BorderRadius.circular(14),
                child: InkWell(
                  onTap: () {
                    HapticFeedback.selectionClick();
                    onPick(p.$1);
                  },
                  borderRadius: BorderRadius.circular(14),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 15),
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(14),
                      border: Border.all(color: MemoTheme.border),
                    ),
                    child: Row(
                      children: [
                        Icon(p.$2, size: 19, color: MemoTheme.accent),
                        const SizedBox(width: 14),
                        Expanded(child: Text(p.$1, style: MemoTheme.body(14.5, color: MemoTheme.text))),
                        const Icon(Icons.arrow_outward_rounded, size: 16, color: MemoTheme.textFaint),
                      ],
                    ),
                  ),
                ),
              ),
            )),
      ],
    );
  }
}
