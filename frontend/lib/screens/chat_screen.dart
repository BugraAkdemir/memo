import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';
import 'package:path/path.dart' as p;
import 'dart:io';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';
import '../providers/whatsapp_provider.dart';
import '../widgets/chat_sidebar.dart';
import '../widgets/chat_message_list.dart';
import '../widgets/chat_input.dart';
import '../widgets/welcome_view.dart';
import '../providers/agent_provider.dart';

/// Chat screen — sidebar + message list + input area.
class ChatScreen extends ConsumerWidget {
   const ChatScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Row(
      children: [
        // ─── Sidebar ──────────────────────────────
        Padding(
          padding: MemoTheme.of(context).isGlass
              ? const EdgeInsets.fromLTRB(8, 12, 8, 12)
              : EdgeInsets.zero,
          child: ChatSidebar(),
        ),

        // ─── Main Chat Area ───────────────────────
        Expanded(
          // Transparent in Glass Light so the soft gradient shows behind the
          // messages and the frosted input bar; opaque panel in dark.
          child: Container(
            color: MemoTheme.of(context).isGlass
                ? Colors.transparent
                : MemoTheme.of(context).bgApp,
            child: _ChatContent(),
          ),
        ),
      ],
    );
  }
}

class _ChatContent extends ConsumerStatefulWidget {
  @override
  ConsumerState<_ChatContent> createState() => _ChatContentState();
}

class _ChatContentState extends ConsumerState<_ChatContent> {
  @override
  Widget build(BuildContext context) {
    final messagesAsync = ref.watch(messagesProvider);

    ref.listen(errorMessageProvider, (prev, next) {
      if (next.isNotEmpty) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(next), duration: const Duration(seconds: 4)),
        );
        ref.read(errorMessageProvider.notifier).state = '';
      }
    });

    return Column(
      children: [
        // ─── Top Bar ──────────────────────────────
         _ChatTopBar(),

        // ─── Messages or Welcome ──────────────────
        Expanded(
          child: messagesAsync.when(
            loading: () =>  Center(
              child: CircularProgressIndicator(color: MemoTheme.accent),
            ),
            error: (e, _) => Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline, color: MemoTheme.red, size: 40),
                   SizedBox(height: 12),
                  Text(
                    '$e',
                    style: TextStyle(color: MemoTheme.of(context).textMuted),
                    textAlign: TextAlign.center,
                  ),
                   SizedBox(height: 12),
                  OutlinedButton(
                    onPressed: () => ref.invalidate(messagesProvider),
                    child: Text(L10n.t('retry')),
                  ),
                ],
              ),
            ),
            data: (messages) {
              final isSending = ref.watch(isSendingProvider);
              final streamingContent = ref.watch(streamingContentProvider);
              final streamingThinking = ref.watch(streamingThinkingProvider);
              final streamingAgentEvents = ref.watch(streamingAgentEventsProvider);
              final streamingStatus = ref.watch(streamingStatusProvider);
              if (messages.isEmpty && !isSending && streamingContent.isEmpty && streamingAgentEvents.isEmpty) {
                return  WelcomeView();
              }
              return ChatMessageList(
                messages: messages,
                isTyping: isSending,
                streamingContent: streamingContent,
                streamingThinking: streamingThinking,
                streamingAgentEvents: streamingAgentEvents,
                statusText: streamingStatus,
                onEdit: (index, newContent) {
                  ref.read(messagesProvider.notifier).updateMessage(index, newContent);
                },
                onDelete: (index) {
                  ref.read(messagesProvider.notifier).deleteMessage(index);
                },
              );
            },
          ),
        ),

        // ─── Input ────────────────────────────────
         ChatInput(),
      ],
    );
  }
}

/// Compact live token usage chip. Hidden until a turn reports usage.
class _TokenCounter extends ConsumerWidget {
  const _TokenCounter();

  String _fmt(int n) {
    if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
    if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}k';
    return '$n';
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final usage = ref.watch(tokenUsageProvider);
    if (usage == null || usage.total == 0) return const SizedBox.shrink();
    final c = MemoTheme.of(context);
    final frac = usage.fraction;
    // Warn (amber) past 80% of the context budget.
    final near = frac != null && frac > 0.8;
    final color = near ? MemoTheme.warningOrange : c.textDim;

    final label = usage.budget > 0
        ? '${_fmt(usage.total)} / ${_fmt(usage.budget)}'
        : _fmt(usage.total);

    return Tooltip(
      message:
          'Girdi ${_fmt(usage.input)} · Çıktı ${_fmt(usage.output)}${usage.budget > 0 ? ' · Bütçe ${_fmt(usage.budget)}' : ''}',
      child: Container(
        margin: const EdgeInsets.only(right: 4),
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        decoration: BoxDecoration(
          color: c.bgElement,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: c.borderSoft),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.data_usage, size: 13, color: color),
            const SizedBox(width: 5),
            Text(
              label,
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: near ? MemoTheme.warningOrange : c.textMuted,
                fontFeatures: const [FontFeature.tabularFigures()],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ChatTopBar extends ConsumerWidget {
   const _ChatTopBar();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isIncognito = ref.watch(incognitoProvider);
    final isAgentEnabled = ref.watch(agentEnabledProvider);
    final isAutoPermission = ref.watch(agentAutoPermissionProvider);
    final isWhatsAppMode = ref.watch(whatsAppChatModeProvider);
    final webSearchOn = ref.watch(webSearchModeProvider);
    final waStatus = ref.watch(whatsAppStatusProvider);
    final chatListAsync = ref.watch(chatListProvider);
    final activeChatAsync = ref.watch(activeChatIdProvider);

    String title = L10n.t('new_chat');
    String? agentProjectPath;
    activeChatAsync.whenData((activeId) {
      chatListAsync.whenData((chats) {
        final chat = chats.where((c) => c.id == activeId).firstOrNull;
        if (chat != null) {
          title = chat.title;
          agentProjectPath = chat.projectPath;
        }
      });
    });

    final c = MemoTheme.of(context);
    final glass = c.isGlass;
    return Container(
      height: 56,
      margin: glass
          ? const EdgeInsets.fromLTRB(12, 12, 12, 8)
          : EdgeInsets.zero,
      padding: const EdgeInsets.symmetric(horizontal: 20),
      decoration: glass
          ? BoxDecoration(
              color: c.bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
                  border: Border.all(color: c.borderSoft),
                  boxShadow: MemoTheme.shadowLg,
                )
          : BoxDecoration(
              color: c.bgApp,
              border: Border(bottom: BorderSide(color: c.borderSoft)),
            ),
      child: Row(
        children: [
          // Title
          Expanded(
            child: Row(
              children: [
                if (isIncognito) ...[
                  Tooltip(
                    message: L10n.t('incognito_tooltip'),
                    child: Container(
                    padding:  EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 3,
                    ),
                    decoration: BoxDecoration(
                      color: MemoTheme.warmBrown.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.visibility_off,
                          size: 14,
                          color: MemoTheme.warmBrown,
                        ),
                         SizedBox(width: 4),
                        Text(
                          L10n.t('incognito_on'),
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                            color: MemoTheme.warmBrown,
                          ),
                        ),
                      ],
                    ),
                  ),
                  ),
                   SizedBox(width: 12),
                ],
                if (isAgentEnabled && agentProjectPath != null) ...[
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 3,
                    ),
                    decoration: BoxDecoration(
                      color: MemoTheme.green.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.folder_outlined,
                          size: 14,
                          color: MemoTheme.green,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          '${L10n.t('agent_chat_project')}${p.basename(agentProjectPath!)}',
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                            color: MemoTheme.green,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 12),
                ] else if (isAgentEnabled && agentProjectPath == null) ...[
                  Tooltip(
                    message: L10n.t('mode_agent_desc'),
                    child: Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 3,
                    ),
                    decoration: BoxDecoration(
                      color: MemoTheme.accent.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.psychology,
                          size: 14,
                          color: MemoTheme.accent,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          L10n.t('agent_badge'),
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                            color: MemoTheme.accent,
                          ),
                        ),
                      ],
                    ),
                  ),
                  ),
                  const SizedBox(width: 12),
                ],
                if (isAgentEnabled && isAutoPermission) ...[
                  Tooltip(
                    message: 'Shift+Tab to toggle off — all tools auto-approved',
                    child: GestureDetector(
                    onTap: () => ref.read(agentAutoPermissionProvider.notifier).toggle(),
                    child: Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 3,
                    ),
                    decoration: BoxDecoration(
                      color: MemoTheme.green.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.auto_awesome,
                          size: 14,
                          color: MemoTheme.green,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          'Auto',
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                            color: MemoTheme.green,
                          ),
                        ),
                      ],
                    ),
                  ),
                  ),
                  ),
                  const SizedBox(width: 12),
                ],
                Flexible(
                  child: Text(
                    isIncognito ? L10n.t('incognito_mode') : title,
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                      color: MemoTheme.of(context).textMain,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ),

          // Live token counter (Claude-Code style)
          const _TokenCounter(),

          // Undo button (only when agent mode is on)
          if (isAgentEnabled)
            IconButton(
              icon: Icon(Icons.undo, size: 20),
              color: MemoTheme.of(context).textDim,
              tooltip: L10n.t('agent_undo'),
              onPressed: () async {
                try {
                  await ref.read(apiClientProvider).undoAgentEdit();
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(L10n.t('agent_undone'))),
                    );
                  }
                } catch (e) {
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(L10n.t('agent_undo_failed', {'e': e.toString()})), backgroundColor: MemoTheme.red),
                    );
                  }
                }
              },
            ),
          
          // Agent mode toggle — was previously only reachable from the
          // separate Agent tab, so plain Chat had no visible way to turn on
          // file/command tools; this mirrors the web-search toggle right
          // next to it. Session-scoped like web search, not persisted onto
          // this specific chat: switching away and back re-derives it from
          // the chat's own type (ActiveChatIdNotifier.switchTo).
          IconButton(
            icon: Icon(
              Icons.smart_toy,
              size: 20,
              color: isAgentEnabled
                  ? MemoTheme.green
                  : MemoTheme.of(context).textDim,
            ),
            tooltip: '${isAgentEnabled ? L10n.t('agent_mode_on') : L10n.t('agent_mode_off')} — ${L10n.t('agent_mode_tooltip')}',
            onPressed: () => ref.read(agentEnabledProvider.notifier).setEnabled(!isAgentEnabled),
          ),

          // Web search mode toggle — when on, every message uses live web results
          IconButton(
            icon: Icon(
              Icons.travel_explore,
              size: 20,
              color: webSearchOn
                  ? MemoTheme.green
                  : MemoTheme.of(context).textDim,
            ),
            tooltip: webSearchOn
                ? (L10n.locale == MemoLocale.tr
                    ? 'Web araması açık'
                    : 'Web search on')
                : (L10n.locale == MemoLocale.tr
                    ? 'Web araması kapalı'
                    : 'Web search off'),
            onPressed: () => ref.read(webSearchModeProvider.notifier).toggle(),
          ),

          // WhatsApp mode toggle (only when connected)
          if (waStatus.asData?.value.connected == true)
            IconButton(
              icon: Icon(
                Icons.chat,
                size: 20,
                color: isWhatsAppMode ? MemoTheme.green : MemoTheme.of(context).textDim,
              ),
              tooltip: isWhatsAppMode
                  ? '${L10n.t('whatsapp_mode_on')} — ${L10n.t('whatsapp_mode_tooltip')}'
                  : '${L10n.t('whatsapp_mode_off')} — ${L10n.t('whatsapp_mode_tooltip')}',
              onPressed: () => ref.read(whatsAppChatModeProvider.notifier).toggle(),
            ),
          
          // Export button
          IconButton(
            icon:  Icon(Icons.file_download_outlined, size: 20),
            color: MemoTheme.of(context).textDim,
            tooltip: L10n.t('export_chat'),
            onPressed: () async {
              try {
                final api = ref.read(apiClientProvider);
                final md = await api.exportChat();
                if (md.isEmpty || !context.mounted) return;
                final path = await FilePicker.platform.saveFile(
                  dialogTitle: L10n.t('export_chat'),
                  fileName: 'chat_export.md',
                  type: FileType.custom,
                  allowedExtensions: ['md'],
                );
                if (path != null) {
                  await File(path).writeAsString(md);
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(L10n.t('chat_exported', {'path': path}))),
                    );
                  }
                }
              } catch (e) {
                if (context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(L10n.t('export_failed', {'e': e.toString()}))),
                  );
                }
              }
            },
          ),
        ],
      ),
    );
  }
}
