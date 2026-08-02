import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';
import 'package:path/path.dart' as p;
import 'dart:io';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/provider_config.dart';
import '../providers/chat_provider.dart';
import '../providers/provider_provider.dart';
import '../providers/whatsapp_provider.dart';
import '../widgets/chat_sidebar.dart';
import '../widgets/chat_message_list.dart';
import '../widgets/chat_input.dart';
import '../widgets/provider_config_dialog.dart';
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
      message: usage.budget > 0
          ? L10n.t('usage_tooltip_budget', {
              'input': _fmt(usage.input),
              'output': _fmt(usage.output),
              'budget': _fmt(usage.budget),
            })
          : L10n.t('usage_tooltip', {
              'input': _fmt(usage.input),
              'output': _fmt(usage.output),
            }),
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

/// Quick model/provider switcher — anchored dropdown opened from the top
/// bar. Lists the local model plus every *enabled* registered API provider
/// (from `data/providers.json`, via [providerListProvider]), and a trailing
/// shortcut to register a new one. Mirrors the selection logic already used
/// by `chat_input.dart`'s `/model` dialog (`ProviderConfig.name` is the
/// identifier `activeProviderTypeProvider`/`setActiveProvider` operate on,
/// not `ProviderConfig.type` — a provider type can be registered more than
/// once under different names), but as a compact anchored menu instead of a
/// modal dialog, matching where the user expects it in the toolbar.
class _QuickModelDropdown extends ConsumerWidget {
  const _QuickModelDropdown();

  Future<void> _openMenu(BuildContext context, WidgetRef ref) async {
    final button = context.findRenderObject() as RenderBox;
    final overlay = Overlay.of(context).context.findRenderObject() as RenderBox;
    final position = RelativeRect.fromRect(
      Rect.fromPoints(
        button.localToGlobal(button.size.bottomLeft(Offset.zero), ancestor: overlay),
        button.localToGlobal(button.size.bottomRight(Offset.zero), ancestor: overlay),
      ),
      Offset.zero & overlay.size,
    );

    String activeType;
    List<ProviderConfig> providers;
    try {
      activeType = await ref.read(activeProviderTypeProvider.future);
      providers = await ref.read(providerListProvider.future);
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('providers_load_failed', {'e': e.toString()}))),
        );
      }
      return;
    }
    if (!context.mounted) return;

    final effectiveActive = activeType.isEmpty ? 'local' : activeType;
    final enabledProviders = providers.where((p) => p.enabled).toList();
    final c = MemoTheme.of(context);

    final selected = await showMenu<String>(
      context: context,
      position: position,
      items: [
        PopupMenuItem(
          value: 'local',
          child: _QuickModelMenuRow(
            leading: Icon(Icons.computer_outlined, size: 18, color: c.textMuted),
            label: L10n.t('local_model'),
            isActive: effectiveActive == 'local',
          ),
        ),
        for (final p in enabledProviders)
          PopupMenuItem(
            value: p.name,
            child: _QuickModelMenuRow(
              leading: providerLogoWidget(p.type, size: 18),
              label: p.name,
              isActive: effectiveActive == p.name,
            ),
          ),
        const PopupMenuDivider(),
        PopupMenuItem(
          value: '__add_provider__',
          child: _QuickModelMenuRow(
            leading: Icon(Icons.add, size: 18, color: MemoTheme.accent),
            label: L10n.t('add_provider'),
            labelColor: MemoTheme.accent,
          ),
        ),
      ],
    );

    if (selected == null || !context.mounted) return;

    if (selected == '__add_provider__') {
      final added = await showDialog<bool>(
        context: context,
        builder: (_) => const ProviderConfigDialog(),
      );
      if (added == true) {
        ref.invalidate(providerListProvider);
      }
      return;
    }

    try {
      final api = ref.read(apiClientProvider);
      // CLI-backed providers (claude-code-cli, ...) are per-chat, not the
      // app-wide active provider — a real coding agent running for one chat
      // must never silently become every other chat's default too.
      final selectedProvider = enabledProviders
          .where((p) => p.name == selected)
          .firstOrNull;
      if (selectedProvider != null && selectedProvider.type == 'claude-code-cli') {
        final chatId = ref.read(activeChatIdProvider).valueOrNull;
        if (chatId == null) return;
        await api.setChatCLIProvider(chatId, selectedProvider.type);
        if (context.mounted) {
          await _setupCLIChat(context, ref, chatId);
        }
      } else if (selected == 'local') {
        await api.setActiveProvider('');
        ref.read(activeProviderTypeProvider.notifier).setActive('');
      } else {
        await api.setActiveProvider(selected);
        ref.read(activeProviderTypeProvider.notifier).setActive(selected);
      }
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(L10n.t('switched_to', {'name': selected == 'local' ? L10n.t('local_model') : selected})),
            duration: const Duration(seconds: 2),
          ),
        );
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('switch_failed', {'e': e.toString()}))),
        );
      }
    }
  }

  /// First time a chat is pointed at a CLI provider: warns the user this is
  /// a real agent (file/shell access), then asks which folder it should run
  /// in. Only runs once per chat — a workdir already set means both of
  /// these already happened.
  Future<void> _setupCLIChat(BuildContext context, WidgetRef ref, String chatId) async {
    final api = ref.read(apiClientProvider);
    final existing = await api.getChatCLIWorkdir(chatId);
    if (existing.isNotEmpty || !context.mounted) return;

    final proceed = await showDialog<bool>(
      context: context,
      builder: (_) => AlertDialog(
        title: Text(L10n.t('cli_first_use_title')),
        content: Text(L10n.t('cli_first_use_body')),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(L10n.t('cancel')),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(L10n.t('cli_first_use_continue')),
          ),
        ],
      ),
    );
    if (proceed != true || !context.mounted) return;

    final dir = await FilePicker.platform.getDirectoryPath(
      dialogTitle: L10n.t('cli_pick_workdir_title'),
    );
    if (dir == null) return;
    try {
      await api.setChatCLIWorkdir(chatId, dir);
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('switch_failed', {'e': e.toString()}))),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final activeType = ref.watch(activeProviderTypeProvider).valueOrNull ?? '';
    final providers = ref.watch(providerListProvider).valueOrNull ?? const <ProviderConfig>[];
    final isLocal = activeType.isEmpty;

    final String label;
    final Widget leadingIcon;
    if (isLocal) {
      label = L10n.t('local_model');
      leadingIcon = Icon(Icons.computer_outlined, size: 15, color: c.textMuted);
    } else {
      final match = providers.where((p) => p.name == activeType);
      final providerType = match.isNotEmpty ? match.first.type : activeType;
      label = activeType;
      leadingIcon = providerLogoWidget(providerType, size: 15);
    }

    return Builder(
      builder: (btnContext) => Tooltip(
        message: L10n.t('switch_model'),
        child: MouseRegion(
          cursor: SystemMouseCursors.click,
          child: GestureDetector(
            onTap: () => _openMenu(btnContext, ref),
            child: Container(
              margin: const EdgeInsets.only(right: 8),
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
              constraints: const BoxConstraints(maxWidth: 170),
              decoration: BoxDecoration(
                color: c.bgElement,
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: c.borderSoft),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  leadingIcon,
                  const SizedBox(width: 6),
                  Flexible(
                    child: Text(
                      label,
                      style: TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: c.textMain,
                      ),
                      overflow: TextOverflow.ellipsis,
                      maxLines: 1,
                    ),
                  ),
                  const SizedBox(width: 4),
                  Icon(Icons.expand_more, size: 16, color: c.textDim),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _QuickModelMenuRow extends StatelessWidget {
  final Widget leading;
  final String label;
  final bool isActive;
  final Color? labelColor;

  const _QuickModelMenuRow({
    required this.leading,
    required this.label,
    this.isActive = false,
    this.labelColor,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        leading,
        const SizedBox(width: 10),
        Expanded(
          child: Text(
            label,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              fontWeight: isActive ? FontWeight.w600 : FontWeight.w400,
              color: labelColor,
            ),
          ),
        ),
        if (isActive) ...[
          const SizedBox(width: 8),
          Icon(Icons.check, size: 16, color: MemoTheme.accent),
        ],
      ],
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
                    message: L10n.t('auto_permission_tooltip'),
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

          // Quick model/provider switch dropdown — local model, every
          // enabled API provider, and a shortcut to add a new one.
          const _QuickModelDropdown(),

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
                ? (L10n.t('web_search_on'))
                : (L10n.t('web_search_off')),
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
