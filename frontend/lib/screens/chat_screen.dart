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
import '../widgets/backend_unreachable_view.dart';
import '../widgets/server_file_browser_dialog.dart';
import '../providers/agent_provider.dart';
import '../providers/auth_gate_provider.dart';
import '../providers/gate_guard.dart';
import '../core/friendly_error.dart';

/// Chat screen — sidebar + message list + input area.
class ChatScreen extends ConsumerWidget {
   const ChatScreen({super.key});

  // Below this width, a permanently-inline 260px ChatSidebar (chat_sidebar.dart)
  // leaves too little room for a usable chat area — confirmed live at a
  // 375px phone width, where it squeezed _ChatContent's available width down
  // to ~15px and threw a 189px RenderFlex overflow. Below the breakpoint the
  // sidebar moves into AppShell's shared mobile drawer (opened via a menu
  // button in _ChatTopBar) instead of sitting inline.

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final narrow = constraints.maxWidth < MemoTheme.mobileNavBreakpoint;
        final isIncognito = ref.watch(incognitoProvider);

        final content = Container(
          // Transparent in Glass Light so the soft gradient shows behind the
          // messages and the frosted input bar; opaque panel in dark. In
          // Incognito Mode a translucent wash of the theme's own red is
          // layered on top instead — background only, so message bubbles,
          // the input bar and the sidebar keep their normal colors and the
          // cue reads as "this chat isn't being saved" rather than a
          // jarring full recolor.
          color: isIncognito
              ? MemoTheme.red.withValues(alpha: 0.14)
              : (MemoTheme.of(context).isGlass
                  ? Colors.transparent
                  : MemoTheme.of(context).bgApp),
          child: _ChatContent(showMenuButton: narrow),
        );

        // Narrow mode used to wrap content in its own Scaffold+Drawer here
        // for ChatSidebar — removed in favor of AppShell's single shared
        // mobile drawer (Sohbetler/Menü tabs), which also sidesteps a
        // Scaffold-always-paints-its-own-opaque-background seam bug a
        // second nested Scaffold used to cause against the Glass Light
        // gradient (see the Session 19 fix this replaced). _ChatTopBar's
        // menu button still opens a drawer via Scaffold.of(context) — it
        // now finds AppShell's ancestor Scaffold instead of a local one.
        if (narrow) {
          return content;
        }

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
            Expanded(child: content),
          ],
        );
      },
    );
  }
}

class _ChatContent extends ConsumerStatefulWidget {
  final bool showMenuButton;
  const _ChatContent({this.showMenuButton = false});

  @override
  ConsumerState<_ChatContent> createState() => _ChatContentState();
}

class _ChatContentState extends ConsumerState<_ChatContent> {
  @override
  Widget build(BuildContext context) {
    final messagesAsync = ref.watch(messagesProvider);

    // errorMessageProvider's SnackBar is shown exactly once, in
    // app_shell.dart's own ref.listen (added first, June — this screen's
    // near-identical copy was added later, July, and duplicated it: both
    // are mounted simultaneously since ChatScreen lives inside AppShell's
    // IndexedStack, so every error toast showed twice back to back, same
    // text, different styling). Reported live by the user. Don't re-add a
    // listener here — app_shell.dart's already covers every screen.

    // BUG-ONB4/BUG-ONB6: messagesProvider/chatListProvider/
    // activeChatIdProvider all mount empty while the auth gate blocks
    // (their build() may not watch the gate's stream — Riverpod would
    // leave the future pending forever), so the gate flipping open after
    // login/setup must explicitly reload them — gate change is the one
    // signal that 401-era emptiness can turn into real data. Without this,
    // the chat sidebar/active-chat/message list stayed permanently stuck
    // on "Bir şeyler ters gitti" after connecting to a gated backend until
    // something else (creating a new chat) happened to re-fetch — on
    // desktop, with no page-refresh escape hatch, that meant forever stuck.
    ref.listen<AsyncValue<AuthGateInfo>>(authGateProvider, (prev, next) {
      if (authGateBlocked(prev?.valueOrNull) != authGateBlocked(next.valueOrNull)) {
        ref.invalidate(messagesProvider);
        ref.invalidate(chatListProvider);
        ref.invalidate(activeChatIdProvider);
      }
    });

    return Column(
      children: [
        // ─── Top Bar ──────────────────────────────
         _ChatTopBar(showMenuButton: widget.showMenuButton),

        // ─── Messages or Welcome ──────────────────
        Expanded(
          child: messagesAsync.when(
            loading: () =>  Center(
              child: CircularProgressIndicator(color: MemoTheme.accent),
            ),
            error: (e, _) => isBackendUnreachableError(e)
                ? const BackendUnreachableView()
                : Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.error_outline, color: MemoTheme.red, size: 40),
                         SizedBox(height: 12),
                        Text(
                          FriendlyError.describeGeneric(e),
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
              final isCLIChat = (ref.watch(activeChatCLIProviderProvider).valueOrNull ?? '').isNotEmpty;
              return ChatMessageList(
                messages: messages,
                isTyping: isSending,
                streamingContent: streamingContent,
                streamingThinking: streamingThinking,
                streamingAgentEvents: streamingAgentEvents,
                statusText: streamingStatus,
                isCLIChat: isCLIChat,
                apiBaseUrl: ref.watch(apiClientProvider).baseUrl,
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
/// Shows the active chat's CLI working directory (just the folder name,
/// full path on hover) next to the model picker.
class _CLIWorkdirBadge extends ConsumerWidget {
  const _CLIWorkdirBadge();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final workdir = ref.watch(activeChatCLIWorkdirProvider).valueOrNull ?? '';
    if (workdir.isEmpty) return const SizedBox.shrink();
    final c = MemoTheme.of(context);

    return Tooltip(
      message: workdir,
      child: Container(
        margin: const EdgeInsets.only(right: 8),
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        constraints: const BoxConstraints(maxWidth: 140),
        decoration: BoxDecoration(
          color: c.bgElement,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: c.borderSoft),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.folder_outlined, size: 14, color: c.textDim),
            const SizedBox(width: 6),
            Flexible(
              child: Text(
                p.basename(workdir),
                style: TextStyle(fontSize: 12, color: c.textSecondary),
                overflow: TextOverflow.ellipsis,
                maxLines: 1,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Tappable badge showing the active chat's CLI model override (or the
/// CLI's own default when none is set) — opens a small menu to pick from
/// that CLI's available models, right next to the working-directory badge.
class _CLIModelBadge extends ConsumerWidget {
  const _CLIModelBadge();

  Future<void> _openMenu(BuildContext context, WidgetRef ref, String cliType, String chatId) async {
    final button = context.findRenderObject() as RenderBox;
    final overlay = Overlay.of(context).context.findRenderObject() as RenderBox;
    final position = RelativeRect.fromRect(
      Rect.fromPoints(
        button.localToGlobal(button.size.bottomLeft(Offset.zero), ancestor: overlay),
        button.localToGlobal(button.size.bottomRight(Offset.zero), ancestor: overlay),
      ),
      Offset.zero & overlay.size,
    );

    List<String> models;
    String current;
    try {
      models = await ref.read(cliModelOptionsProvider(cliType).future);
      current = await ref.read(apiClientProvider).getChatCLIModel(chatId);
    } catch (e) {
      if (context.mounted) {
        ref.read(errorMessageProvider.notifier).state =
            L10n.t('switch_failed', {'e': e.toString()});
      }
      return;
    }
    if (!context.mounted) return;

    final selected = await showMenu<String>(
      context: context,
      position: position,
      items: [
        PopupMenuItem(
          value: '',
          child: _QuickModelMenuRow(
            leading: Icon(Icons.auto_awesome_outlined, size: 15, color: MemoTheme.of(context).textMuted),
            label: L10n.t('cli_model_default'),
            isActive: current.isEmpty,
          ),
        ),
        if (models.isEmpty)
          PopupMenuItem(
            enabled: false,
            child: _QuickModelMenuRow(
              leading: Icon(Icons.info_outline, size: 15, color: MemoTheme.of(context).textDim),
              label: L10n.t('cli_model_none_available'),
              labelColor: MemoTheme.of(context).textDim,
            ),
          )
        else
          for (final m in models)
            PopupMenuItem(
              value: m,
              child: _QuickModelMenuRow(
                leading: Icon(Icons.terminal_rounded, size: 15, color: MemoTheme.accent),
                label: m,
                isActive: current == m,
              ),
            ),
      ],
    );

    if (selected == null || !context.mounted) return;
    try {
      await ref.read(apiClientProvider).setChatCLIModel(chatId, selected);
      ref.invalidate(activeChatCLIModelProvider);
    } catch (e) {
      ref.read(errorMessageProvider.notifier).state =
          L10n.t('switch_failed', {'e': e.toString()});
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final cliType = ref.watch(activeChatCLIProviderProvider).valueOrNull ?? '';
    final chatId = ref.watch(activeChatIdProvider).valueOrNull ?? '';
    if (cliType.isEmpty || chatId.isEmpty) return const SizedBox.shrink();
    final model = ref.watch(activeChatCLIModelProvider).valueOrNull ?? '';
    final c = MemoTheme.of(context);
    final label = model.isEmpty ? L10n.t('cli_model_default') : model;

    return Builder(
      builder: (btnContext) => Tooltip(
        message: L10n.t('cli_model_switch_tooltip'),
        child: MouseRegion(
          cursor: SystemMouseCursors.click,
          child: GestureDetector(
            onTap: () => _openMenu(btnContext, ref, cliType, chatId),
            child: Container(
              margin: const EdgeInsets.only(right: 8),
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
              constraints: const BoxConstraints(maxWidth: 140),
              decoration: BoxDecoration(
                color: c.bgElement,
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: c.borderSoft),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.tune_rounded, size: 14, color: c.textDim),
                  const SizedBox(width: 6),
                  Flexible(
                    child: Text(
                      label,
                      style: TextStyle(fontSize: 12, color: c.textSecondary),
                      overflow: TextOverflow.ellipsis,
                      maxLines: 1,
                    ),
                  ),
                  const SizedBox(width: 4),
                  Icon(Icons.expand_more, size: 14, color: c.textDim),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

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
    String cliType = '';
    try {
      activeType = await ref.read(activeProviderTypeProvider.future);
      providers = await ref.read(providerListProvider.future);
      final chatId = ref.read(activeChatIdProvider).valueOrNull;
      if (chatId != null) {
        cliType = await ref.read(apiClientProvider).getChatCLIProvider(chatId);
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('providers_load_failed', {'e': e.toString()}))),
        );
      }
      return;
    }
    if (!context.mounted) return;

    final activeCLIName = cliType.isEmpty
        ? ''
        : (providers.where((p) => p.type == cliType).firstOrNull?.name ?? cliType);
    final effectiveActive =
        activeCLIName.isNotEmpty ? activeCLIName : (activeType.isEmpty ? 'local' : activeType);
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
      // CLI-backed providers (claude-code-cli, codex-cli, ...) are per-chat,
      // not the app-wide active provider — a real coding agent running for
      // one chat must never silently become every other chat's default too.
      final selectedProvider = enabledProviders
          .where((p) => p.name == selected)
          .firstOrNull;
      if (selectedProvider != null && ProviderDefaults.isCLIType(selectedProvider.type)) {
        final chatId = ref.read(activeChatIdProvider).valueOrNull;
        if (chatId == null) return;
        await api.setChatCLIProvider(chatId, selectedProvider.type);
        // _setupCLIChat first, invalidate after: invalidating rebuilds
        // _ChatTopBar (it watches activeChatCLIProviderProvider), which can
        // deactivate this context before _setupCLIChat even runs — the
        // dialog/folder picker then silently never appears on the first
        // pick (only worked on a second attempt, once the rebuild had
        // already happened and settled).
        if (context.mounted) {
          await _setupCLIChat(context, ref, chatId);
        }
        ref.invalidate(activeChatCLIProviderProvider);
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
      builder: (dialogContext) => AlertDialog(
        title: Text(L10n.t('cli_first_use_title')),
        content: Text(L10n.t('cli_first_use_body')),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: Text(L10n.t('cancel')),
          ),
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: Text(L10n.t('cli_first_use_continue')),
          ),
        ],
      ),
    );
    if (proceed != true || !context.mounted) return;

    final dir = await showServerFileBrowserDialog(context, mode: ServerBrowseMode.directory);
    if (dir == null) return;
    try {
      await api.setChatCLIWorkdir(chatId, dir);
    } catch (e) {
      // Not ScaffoldMessenger.of(context) — the native folder picker above
      // can stay open for a long, unpredictable time, and by the time it
      // resolves the widget this context came from (deep inside the model
      // dropdown's popup menu) may already be gone. errorMessageProvider is
      // a global, context-free sink a stable top-level listener shows as a
      // SnackBar (see _ChatContent's ref.listen) — this codebase's own
      // established way of avoiding exactly this class of crash.
      ref.read(errorMessageProvider.notifier).state =
          L10n.t('switch_failed', {'e': e.toString()});
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final activeType = ref.watch(activeProviderTypeProvider).valueOrNull ?? '';
    final providers = ref.watch(providerListProvider).valueOrNull ?? const <ProviderConfig>[];
    // A chat's own CLI provider takes priority over the app-wide active
    // provider in what's SHOWN here — it's what that chat actually sends
    // to, even though picking it never touched activeProviderTypeProvider
    // (deliberately — see chat_screen.dart's CLI branch above).
    final cliType = ref.watch(activeChatCLIProviderProvider).valueOrNull ?? '';
    final isLocal = activeType.isEmpty && cliType.isEmpty;

    final String label;
    final Widget leadingIcon;
    if (cliType.isNotEmpty) {
      final match = providers.where((p) => p.type == cliType);
      label = match.isNotEmpty ? match.first.name : cliType;
      leadingIcon = Icon(Icons.terminal_rounded, size: 15, color: MemoTheme.accent);
    } else if (isLocal) {
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

/// Compact reasoning-effort quick-select shown next to _QuickModelDropdown.
/// Hides itself entirely (SizedBox.shrink) whenever there's nothing useful
/// to show: local mode, a CLI-backed provider (Claude Code/Codex are real
/// agents with their own effort handling, not Memo's HTTP provider path —
/// see ProviderDefaults.isCLIType), no matching provider config, or a
/// provider/model combination effortLevelsProvider reports as having no
/// effort levels at all (e.g. OpenRouter before a model is chosen, or a
/// model that just doesn't support it).
class _QuickEffortSelector extends ConsumerWidget {
  const _QuickEffortSelector();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final activeName = ref.watch(activeProviderTypeProvider).valueOrNull ?? '';
    final cliType = ref.watch(activeChatCLIProviderProvider).valueOrNull ?? '';
    if (activeName.isEmpty || cliType.isNotEmpty) return const SizedBox.shrink();

    final providers = ref.watch(providerListProvider).valueOrNull ?? const <ProviderConfig>[];
    final match = providers.where((p) => p.name == activeName);
    if (match.isEmpty) return const SizedBox.shrink();
    final config = match.first;

    // model is passed unconditionally — the backend only uses it for the
    // types whose discovery actually depends on it (openrouter/claude/
    // gemini/ollama) and ignores it otherwise; see effortLevelsProvider's
    // doc comment.
    final levelsAsync = ref.watch(effortLevelsProvider((config.type, config.model)));
    final levels = levelsAsync.valueOrNull ?? const <String>[];
    if (levels.isEmpty) return const SizedBox.shrink();

    final c = MemoTheme.of(context);
    final current = levels.contains(config.effortLevel) ? config.effortLevel : '';
    final label = current.isEmpty ? L10n.t('effort_level_default') : current;

    Future<void> pick(String value) async {
      try {
        await ref.read(apiClientProvider).updateProvider(config.copyWith(effortLevel: value));
        ref.invalidate(providerListProvider);
      } catch (e) {
        ref.read(errorMessageProvider.notifier).state =
            L10n.t('switch_failed', {'e': e.toString()});
      }
    }

    return Tooltip(
      message: L10n.t('effort_level_label'),
      child: PopupMenuButton<String>(
        tooltip: '',
        onSelected: pick,
        itemBuilder: (_) => [
          PopupMenuItem(
            value: '',
            child: _QuickModelMenuRow(
              leading: Icon(Icons.psychology_outlined, size: 18, color: c.textMuted),
              label: L10n.t('effort_level_default'),
              isActive: current.isEmpty,
            ),
          ),
          for (final level in levels)
            PopupMenuItem(
              value: level,
              child: _QuickModelMenuRow(
                leading: Icon(Icons.psychology_outlined, size: 18, color: c.textMuted),
                label: level,
                isActive: current == level,
              ),
            ),
        ],
        child: Container(
          margin: const EdgeInsets.only(right: 8),
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
          constraints: const BoxConstraints(maxWidth: 110),
          decoration: BoxDecoration(
            color: c.bgElement,
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: c.borderSoft),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.psychology_outlined, size: 15, color: c.textMuted),
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
  final bool showMenuButton;
   const _ChatTopBar({this.showMenuButton = false});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isIncognito = ref.watch(incognitoProvider);
    // A CLI provider (Claude Code, ...) is its own complete agent — Memo's
    // own agent mode/web search/WhatsApp routing never runs for it (see
    // SendCLIMessageStream), so those toggles have zero effect here and are
    // hidden rather than shown doing nothing.
    final isCLIChat = (ref.watch(activeChatCLIProviderProvider).valueOrNull ?? '').isNotEmpty;
    final isAgentEnabled = !isCLIChat && ref.watch(agentEnabledProvider);
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

    // Built as a list (rather than inline in the Row below) so the narrow
    // layout can wrap it in a horizontally-scrollable Flexible instead of
    // letting it sit as unbounded fixed-width Row children — with the menu
    // button, title, and every one of these badges/toggles all fighting for
    // space, this overflowed by 70px at a 375px phone width (confirmed
    // live) once the sidebar fix (above) gave _ChatContent enough width for
    // the topbar's own overflow to surface on its own instead of being
    // swallowed by the sidebar's much larger one.
    // Extracted out of the desktop IconButtons below so the mobile
    // overflow sheet's ListTiles can call the exact same logic instead of
    // duplicating the try/catch + SnackBar handling.
    Future<void> handleUndo() async {
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
    }

    Future<void> handleExport() async {
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
    }

    final trailingActions = <Widget>[
      // Live token counter (Claude-Code style)
      const _TokenCounter(),

      // CLI working directory — always visible which folder a CLI
      // provider is actually operating in, right next to the picker
      // that shows which one it is.
      if (isCLIChat) const _CLIWorkdirBadge(),

      // Which model the CLI itself is using for this chat, tappable to
      // switch — the CLI's own equivalent of the provider picker below.
      if (isCLIChat) const _CLIModelBadge(),

      // Quick model/provider switch dropdown — local model, every
      // enabled API provider, and a shortcut to add a new one.
      const _QuickModelDropdown(),

      // Reasoning-effort quick-select — only renders itself when the active
      // provider/model actually has effort levels to offer.
      const _QuickEffortSelector(),

      // Undo button (only when agent mode is on)
      if (isAgentEnabled)
        IconButton(
          icon: Icon(Icons.undo, size: 20),
          color: MemoTheme.of(context).textDim,
          tooltip: L10n.t('agent_undo'),
          onPressed: handleUndo,
        ),

      // Agent mode toggle — was previously only reachable from the
      // separate Agent tab, so plain Chat had no visible way to turn on
      // file/command tools; this mirrors the web-search toggle right
      // next to it. Session-scoped like web search, not persisted onto
      // this specific chat: switching away and back re-derives it from
      // the chat's own type (ActiveChatIdNotifier.switchTo). Hidden for
      // a CLI-backed chat — see isCLIChat's doc comment above.
      if (!isCLIChat)
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
      if (!isCLIChat)
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
      if (!isCLIChat && waStatus.asData?.value.connected == true)
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
        icon: Icon(Icons.file_download_outlined, size: 20),
        color: MemoTheme.of(context).textDim,
        tooltip: L10n.t('export_chat'),
        onPressed: handleExport,
      ),
    ];

    // Mobile overflow sheet: everything from trailingActions except the
    // model dropdown (kept directly visible — the single most-used action)
    // collapses here as full-width, fully-labeled rows instead of a
    // horizontally-scrolling icon strip. Rebuilds handleUndo/handleExport's
    // sibling toggles as ListTiles rather than reusing the IconButtons
    // above verbatim, since a tap here must also close the sheet first.
    void openMobileActionsSheet() {
      showModalBottomSheet(
        context: context,
        backgroundColor: c.bgPanel,
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.vertical(top: Radius.circular(MemoTheme.radiusLg)),
        ),
        builder: (sheetContext) {
          Widget row({
            required IconData icon,
            required String label,
            required bool isActive,
            required VoidCallback onTap,
          }) {
            return ListTile(
              leading: Icon(icon, color: isActive ? MemoTheme.green : c.textDim),
              title: Text(
                label,
                style: TextStyle(color: isActive ? MemoTheme.green : c.textMain),
              ),
              onTap: () {
                Navigator.of(sheetContext).pop();
                onTap();
              },
            );
          }

          return SafeArea(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const SizedBox(height: 8),
                const Padding(
                  padding: EdgeInsets.symmetric(horizontal: 16),
                  child: Align(alignment: Alignment.centerLeft, child: _TokenCounter()),
                ),
                if (isCLIChat) ...[
                  const Padding(
                    padding: EdgeInsets.symmetric(horizontal: 16),
                    child: Row(children: [_CLIWorkdirBadge(), _CLIModelBadge()]),
                  ),
                  const SizedBox(height: 4),
                ],
                const Padding(
                  padding: EdgeInsets.symmetric(horizontal: 16),
                  child: Align(alignment: Alignment.centerLeft, child: _QuickEffortSelector()),
                ),
                if (isAgentEnabled)
                  row(
                    icon: Icons.undo,
                    label: L10n.t('agent_undo'),
                    isActive: false,
                    onTap: handleUndo,
                  ),
                if (!isCLIChat)
                  row(
                    icon: Icons.smart_toy,
                    label: isAgentEnabled ? L10n.t('agent_mode_on') : L10n.t('agent_mode_off'),
                    isActive: isAgentEnabled,
                    onTap: () => ref.read(agentEnabledProvider.notifier).setEnabled(!isAgentEnabled),
                  ),
                if (!isCLIChat)
                  row(
                    icon: Icons.travel_explore,
                    label: webSearchOn ? L10n.t('web_search_on') : L10n.t('web_search_off'),
                    isActive: webSearchOn,
                    onTap: () => ref.read(webSearchModeProvider.notifier).toggle(),
                  ),
                if (!isCLIChat && waStatus.asData?.value.connected == true)
                  row(
                    icon: Icons.chat,
                    label: isWhatsAppMode ? L10n.t('whatsapp_mode_on') : L10n.t('whatsapp_mode_off'),
                    isActive: isWhatsAppMode,
                    onTap: () => ref.read(whatsAppChatModeProvider.notifier).toggle(),
                  ),
                row(
                  icon: Icons.file_download_outlined,
                  label: L10n.t('export_chat'),
                  isActive: false,
                  onTap: handleExport,
                ),
                const SizedBox(height: 8),
              ],
            ),
          );
        },
      );
    }

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
          // AppShell's own floating hamburger (present on every screen, not
          // just this one) sits on top of this corner — reserve the same
          // space its predecessor here used to occupy so the title doesn't
          // render underneath it.
          if (showMenuButton) const SizedBox(width: 36),
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

          // Wide layout: natural width, exactly as before. Narrow layout
          // (see showMenuButton/ChatScreen's breakpoint): a full badge/
          // toggle set overflowed by 70px at 375px (confirmed live) even
          // as a horizontal scroller — readable but still a cramped strip
          // of unlabeled icons. Only the model dropdown (single most-used
          // action) stays directly visible; everything else collapses
          // behind one overflow icon opening openMobileActionsSheet's
          // full-width, fully-labeled rows.
          if (showMenuButton) ...[
            const _QuickModelDropdown(),
            IconButton(
              icon: Icon(Icons.more_vert, color: c.textDim),
              tooltip: L10n.t('more_actions'),
              onPressed: openMobileActionsSheet,
            ),
          ] else
            ...trailingActions,
        ],
      ),
    );
  }
}
