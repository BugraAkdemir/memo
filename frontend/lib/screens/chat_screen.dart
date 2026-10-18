import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';
import 'package:path/path.dart' as p;
import 'dart:convert';
import 'dart:typed_data';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/provider_config.dart';
import '../models/provider_models.dart';
import '../widgets/chat_model_selection.dart';
export '../widgets/chat_model_selection.dart' show providerModelPickerEntries;
import '../widgets/quota_notice_card.dart';
import '../widgets/model_picker.dart';
import '../models/model_display.dart';
import '../providers/chat_provider.dart';
import '../providers/provider_provider.dart';
import '../providers/whatsapp_provider.dart';
import '../widgets/chat_sidebar.dart';
import '../widgets/chat_message_list.dart';
import '../widgets/chat_input.dart';
import '../widgets/agent/browser_pane.dart';
import '../providers/tasklist_provider.dart';
import '../widgets/provider_config_dialog.dart';
import '../widgets/welcome_view.dart';
import '../widgets/live_realtime_view.dart';
import '../providers/live_realtime_session_provider.dart';
import '../widgets/backend_unreachable_view.dart';
import '../providers/agent_provider.dart';
import '../providers/settings_provider.dart';
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
  // button in ChatTopBar) instead of sitting inline.

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
        // gradient (see the Session 19 fix this replaced). ChatTopBar's
        // menu button still opens a drawer via Scaffold.of(context) — it
        // now finds AppShell's ancestor Scaffold instead of a local one.
        if (narrow) {
          // Full-width takeover, not a side-by-side split — see
          // BrowserPane's own doc comment for why narrow mode gets a
          // different layout than the desktop Row below.
          if (ref.watch(betaEnabledProvider) &&
              ref.watch(browserSessionActiveProvider)) {
            return const BrowserPane(narrow: true);
          }
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

            // ─── Agent Browser (only while a session is open) ──
            if (ref.watch(betaEnabledProvider) &&
                ref.watch(browserSessionActiveProvider))
              const BrowserPane(),
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
         ChatTopBar(showMenuButton: widget.showMenuButton),

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
              final streamingNarration = ref.watch(streamingNarrationProvider);
              final streamingStatus = ref.watch(streamingStatusProvider);
              final isCLIChat = (ref.watch(activeChatCLIProviderProvider).valueOrNull ?? '').isNotEmpty;

              // Google Live/OpenAI Realtime session active: takes over this
              // whole area (in place of both WelcomeView and the normal
              // ChatMessageList) with a dedicated full-screen UI — requested
              // by the user after real-world testing (see
              // docs/plans/PLAN_live_mode_v2.md's follow-up plan). Checked
              // before the messages.isEmpty branch below since a live
              // session can start on an empty chat.
              final liveSessionStatus = ref.watch(liveRealtimeSessionProvider).status;
              if (liveSessionStatus == LiveRealtimeSessionStatus.connecting ||
                  liveSessionStatus == LiveRealtimeSessionStatus.connected) {
                return LiveRealtimeView(
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
              }

              if (messages.isEmpty && !isSending && streamingContent.isEmpty && streamingAgentEvents.isEmpty) {
                return const Column(
                  children: [
                    _ChatConfigNoticeBanner(),
                    Expanded(child: WelcomeView()),
                  ],
                );
              }
              final taskChatId = ref.watch(activeChatIdProvider).valueOrNull ?? '';
              return ChatMessageList(
                modelRoute: ref.watch(streamingModelRouteProvider),
                messages: messages,
                isTyping: isSending,
                streamingContent: streamingContent,
                streamingThinking: streamingThinking,
                streamingAgentEvents: streamingAgentEvents,
                streamingNarration: streamingNarration,
                statusText: streamingStatus,
                streamTiming: ref.watch(streamTimingProvider),
                isCLIChat: isCLIChat,
                apiBaseUrl: ref.watch(apiClientProvider).baseUrl,
                taskActivity: taskChatId.isEmpty
                    ? null
                    : ref.watch(chatTaskForProvider(taskChatId)),
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

        // Quota notice (allowance ran out / running low) — only when there is one.
        const QuotaNoticeCard(),

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

/// Thin strip shown above an empty chat (before the first message is sent)
/// when a setting that quietly changes how replies behave is in a
/// non-default state — Minimal Mode ON (identity/persona/web-search prompt
/// injection off) or Memory OFF (this chat won't be remembered). Both
/// default states → renders nothing. It disappears the moment the first
/// message is sent (the empty-state branch in ChatScreen stops rendering).
class _ChatConfigNoticeBanner extends ConsumerWidget {
  const _ChatConfigNoticeBanner();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final minimalOn = ref.watch(minimalModeProvider).valueOrNull ?? false;
    final memoryOn = ref.watch(memoryEnabledProvider).valueOrNull ?? true;

    final notices = <String>[
      if (minimalOn) L10n.t('chat_notice_minimal_mode_on'),
      if (!memoryOn) L10n.t('chat_notice_memory_off'),
    ];
    if (notices.isEmpty) return const SizedBox.shrink();

    final c = MemoTheme.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
      color: MemoTheme.warningOrange.withValues(alpha: 0.12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(Icons.info_outline, size: 15, color: MemoTheme.warningOrange),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              notices.join('  ·  '),
              style: TextStyle(fontSize: 12, height: 1.35, color: c.textSecondary),
            ),
          ),
        ],
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

  Future<void> _openMenu(
    BuildContext context,
    WidgetRef ref,
    String cliType,
    String chatId,
  ) async {
    final button = context.findRenderObject() as RenderBox;
    final overlay = Overlay.of(context).context.findRenderObject() as RenderBox;
    final position = RelativeRect.fromRect(
      Rect.fromPoints(
        button.localToGlobal(
          button.size.bottomLeft(Offset.zero),
          ancestor: overlay,
        ),
        button.localToGlobal(
          button.size.bottomRight(Offset.zero),
          ancestor: overlay,
        ),
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
        ref.read(errorMessageProvider.notifier).state = L10n.t(
          'switch_failed',
          {'e': FriendlyError.describeGeneric(e)},
        );
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
            leading: Icon(
              Icons.auto_awesome_outlined,
              size: 15,
              color: MemoTheme.of(context).textMuted,
            ),
            label: L10n.t('cli_model_default'),
            isActive: current.isEmpty,
          ),
        ),
        if (models.isEmpty)
          PopupMenuItem(
            enabled: false,
            child: _QuickModelMenuRow(
              leading: Icon(
                Icons.info_outline,
                size: 15,
                color: MemoTheme.of(context).textDim,
              ),
              label: L10n.t('cli_model_none_available'),
              labelColor: MemoTheme.of(context).textDim,
            ),
          )
        else
          for (final m in models)
            PopupMenuItem(
              value: m,
              child: _QuickModelMenuRow(
                leading: Icon(
                  Icons.terminal_rounded,
                  size: 15,
                  color: MemoTheme.accent,
                ),
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
      ref.read(errorMessageProvider.notifier).state = L10n.t('switch_failed', {
        'e': FriendlyError.describeGeneric(e),
      });
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

/// What one load of the model picker's data returns.
class _PickerData {
  final String activeType;
  final List<ProviderConfig> providers;
  final String cliType;
  final List<ProviderModel> subsModels;
  const _PickerData({
    required this.activeType,
    required this.providers,
    required this.cliType,
    required this.subsModels,
  });
}

class _QuickModelDropdown extends ConsumerWidget {
  const _QuickModelDropdown({this.maxWidth = 220});

  /// The button's widest size. The phone top bar passes a share of the screen
  /// instead: at a fixed 220 px it left the chips and chat title next to it
  /// almost no room, and they were drawn over each other.
  final double maxWidth;

  Future<void> _openMenu(BuildContext context, WidgetRef ref) async {
    final button = context.findRenderObject() as RenderBox;
    final anchor = button.localToGlobal(Offset.zero) & button.size;

    final api = ref.read(apiClientProvider);
    final c = MemoTheme.of(context);

    // Everything the picker shows is fetched at once, never one request after
    // another, and the provider list is read straight from the backend (it is
    // millisecond-fast) so a provider registered since app start still shows up.
    // The model list answers from the backend's cache; fresh quota figures come
    // later, from the picker's own background refresh — opening it never waits
    // on a vendor (it used to, and the whole app seemed to freeze).
    Future<_PickerData> load({required bool fresh}) async {
      final chatId = ref.read(activeChatIdProvider).valueOrNull;
      final results = await Future.wait<Object?>([
        ref.read(activeProviderTypeProvider.future),
        api.getProviders(),
        chatId == null ? Future<String>.value('') : api.getChatCLIProvider(chatId),
        // The Subscriptions provider fronts every signed-in vendor account, so
        // it expands into that account's live models. A failed fetch just leaves
        // it as a single row (switching still works).
        api
            .listProviderModels(kSubscriptionsProviderName, fresh: fresh)
            .then<List<ProviderModel>>((l) => l.models)
            .catchError((Object e) {
          debugPrint('chat_screen: subscription model list unavailable, showing one row: $e');
          return const <ProviderModel>[];
        }),
      ]);
      return _PickerData(
        activeType: results[0] as String,
        providers: results[1] as List<ProviderConfig>,
        cliType: results[2] as String,
        subsModels: results[3] as List<ProviderModel>,
      );
    }

    List<ModelPickerEntry> entriesFor(_PickerData d) => chatModelPickerEntries(
      providers: d.providers, subscriptionModels: d.subsModels,
      activeProvider: d.activeType, cliType: d.cliType, mutedColor: c.textMuted,
    );

    final _PickerData first;
    try {
      first = await load(fresh: false);
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('providers_load_failed', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
      return;
    }
    if (!context.mounted) return;
    final enabledProviders = first.providers.where((p) => p.enabled).toList();
    // Keep the top-bar button's own provider list in step with what was just read.
    ref.invalidate(providerListProvider);

    final selected = await showModelPicker(
      context: context,
      anchor: anchor,
      entries: entriesFor(first),
      // The remaining-allowance figures follow usage: once soon after opening
      // (the first open may have had none cached yet), then every 50 seconds
      // while the picker stays open.
      refresh: () async => entriesFor(await load(fresh: true)),
      firstRefreshAfter: const Duration(milliseconds: 2500),
      refreshEvery: const Duration(seconds: 50),
      footer: ModelPickerEntry.item(
        value: '__add_provider__',
        title: L10n.t('add_provider'),
        leading: Icon(Icons.add, size: 16, color: MemoTheme.accent),
      ),
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

    final pickedModel = decodeModelSelection(selected);
    if (pickedModel != null) {
      try {
        await selectNonCLIChatModel(ref.read(apiClientProvider), selected,
            chatId: ref.read(activeChatIdProvider).valueOrNull);
        ref.invalidate(activeChatCLIProviderProvider);
        ref.read(activeProviderTypeProvider.notifier).setActive(pickedModel.provider);
        ref.invalidate(providerListProvider);
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text(L10n.t('switched_to', {'name': pickedModel.model})),
              duration: const Duration(seconds: 2),
            ),
          );
        }
      } catch (e) {
        ref.read(errorMessageProvider.notifier).state =
            L10n.t('switch_failed', {'e': FriendlyError.describeGeneric(e)});
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
        // ChatTopBar (it watches activeChatCLIProviderProvider), which can
        // deactivate this context before _setupCLIChat even runs — the
        // dialog/folder picker then silently never appears on the first
        // pick (only worked on a second attempt, once the rebuild had
        // already happened and settled).
        if (context.mounted) {
          await setupCLIChat(context, ref, chatId);
        }
        ref.invalidate(activeChatCLIProviderProvider);
      } else {
        final provider = await selectNonCLIChatModel(api, selected,
            chatId: ref.read(activeChatIdProvider).valueOrNull);
        ref.read(activeProviderTypeProvider.notifier).setActive(provider);
        ref.invalidate(activeChatCLIProviderProvider);
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
          SnackBar(content: Text(L10n.t('switch_failed', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
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
    } else if (activeType == kAutoRouteProviderName) {
      label = L10n.t('auto_route');
      leadingIcon = Icon(Icons.auto_awesome_outlined, size: 15, color: MemoTheme.accent);
    } else if (isLocal) {
      label = L10n.t('local_model');
      leadingIcon = Icon(Icons.computer_outlined, size: 15, color: c.textMuted);
    } else {
      final match = providers.where((p) => p.name == activeType);
      final providerType = match.isNotEmpty ? match.first.type : activeType;
      // The Subscriptions provider IS its model (one provider, many models), so
      // the button names the model rather than the constant provider name.
      final isSubsModel =
          activeType == kSubscriptionsProviderName && match.isNotEmpty && match.first.model.isNotEmpty;
      label = isSubsModel ? prettyModelName(match.first.model).label : activeType;
      leadingIcon = isSubsModel
          ? modelFamilyLogo(match.first.model, size: 15)
          : providerLogoWidget(providerType, size: 15);
    }

    return Builder(
      builder: (btnContext) => Tooltip(
        message: activeType == kAutoRouteProviderName ? L10n.t('auto_route_help') : L10n.t('switch_model'),
        child: MouseRegion(
          cursor: SystemMouseCursors.click,
          child: GestureDetector(
            onTap: () => _openMenu(btnContext, ref),
            child: Container(
              margin: const EdgeInsets.only(right: 8),
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
              constraints: BoxConstraints(maxWidth: maxWidth),
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

/// What the effort controls need: the active provider's config, the levels it
/// accepts for the current model, and the level in force right now ('' =
/// provider default). Null when there is nothing to choose (see
/// [_QuickEffortSelector]'s doc comment for every case), so callers hide
/// their control instead of showing an empty menu.
class _EffortState {
  const _EffortState(this.config, this.levels, this.current);
  final ProviderConfig config;
  final List<String> levels;
  final String current;
}

_EffortState? _readEffort(WidgetRef ref) {
  final activeName = ref.watch(activeProviderTypeProvider).valueOrNull ?? '';
  final cliType = ref.watch(activeChatCLIProviderProvider).valueOrNull ?? '';
  if (activeName.isEmpty || cliType.isNotEmpty) return null;

  final providers = ref.watch(providerListProvider).valueOrNull ?? const <ProviderConfig>[];
  final match = providers.where((p) => p.name == activeName);
  if (match.isEmpty) return null;
  final config = match.first;

  // model is passed unconditionally — the backend only uses it for the
  // types whose discovery actually depends on it (openrouter/claude/
  // gemini/ollama) and ignores it otherwise; see effortLevelsProvider's
  // doc comment.
  final effortType = config.name == kSubscriptionsProviderName
      ? 'subscriptions'
      : config.type;
  final levelsAsync = ref.watch(effortLevelsProvider((effortType, config.model)));
  final levels = levelsAsync.valueOrNull ?? const <String>[];
  if (levels.isEmpty) return null;
  final current = levels.contains(config.effortLevel) ? config.effortLevel : '';
  return _EffortState(config, levels, current);
}

/// Takes the [ProviderContainer] rather than a WidgetRef: the phone's effort
/// list is a second sheet that outlives the row which opened it, and a
/// WidgetRef must not be used once its widget is gone.
Future<void> _setEffort(ProviderContainer container, ProviderConfig config, String value) async {
  try {
    await container.read(apiClientProvider).updateProvider(config.copyWith(effortLevel: value));
    container.invalidate(providerListProvider);
  } catch (e) {
    container.read(errorMessageProvider.notifier).state =
        L10n.t('switch_failed', {'e': FriendlyError.describeGeneric(e)});
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
    final effort = _readEffort(ref);
    if (effort == null) return const SizedBox.shrink();
    final config = effort.config;
    final levels = effort.levels;
    final current = effort.current;

    final c = MemoTheme.of(context);
    final label = current.isEmpty ? L10n.t('effort_level_default') : current;

    Future<void> pick(String value) =>
        _setEffort(ProviderScope.containerOf(context), config, value);

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

/// The phone's way to the reasoning-effort choice: a full-width, labelled row
/// in the "more actions" sheet that shows the level in force and opens a list
/// of the levels in a second sheet. The desktop chip above was reused here
/// as-is — a small unlabelled pill with a popup menu opening on top of a modal
/// sheet — which is easy to miss and awkward to hit with a thumb. Hidden under
/// the same conditions as the chip (no levels to offer).
class _EffortSheetRow extends ConsumerWidget {
  const _EffortSheetRow({required this.onClose});

  /// Closes the actions sheet this row sits in before the list opens.
  final VoidCallback onClose;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final effort = _readEffort(ref);
    if (effort == null) return const SizedBox.shrink();
    final c = MemoTheme.of(context);
    final current = effort.current;
    final currentLabel = current.isEmpty ? L10n.t('effort_level_default') : current;
    // The sheet's own context dies when it closes; the list needs one that
    // outlives it, so it is opened from the parent route's navigator.
    final navigator = Navigator.of(context, rootNavigator: true);
    final container = ProviderScope.containerOf(context);

    return ListTile(
      key: const Key('effort_sheet_row'),
      leading: Icon(Icons.psychology_outlined, color: c.textDim),
      title: Text(L10n.t('effort_level_label'), style: TextStyle(color: c.textMain)),
      subtitle: Text(currentLabel, style: TextStyle(color: c.textDim)),
      trailing: Icon(Icons.chevron_right, color: c.textDim),
      onTap: () {
        onClose();
        showModalBottomSheet<void>(
          context: navigator.context,
          backgroundColor: c.bgPanel,
          isScrollControlled: true,
          shape: const RoundedRectangleBorder(
            borderRadius: BorderRadius.vertical(top: Radius.circular(MemoTheme.radiusLg)),
          ),
          builder: (sheetContext) => SafeArea(
            child: ConstrainedBox(
              constraints: BoxConstraints(
                maxHeight: MediaQuery.sizeOf(sheetContext).height * 0.7,
              ),
              child: ListView(
                shrinkWrap: true,
                children: [
                  Padding(
                    padding: const EdgeInsets.fromLTRB(20, 16, 20, 8),
                    child: Text(
                      L10n.t('effort_level_label'),
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: c.textMain,
                      ),
                    ),
                  ),
                  for (final value in ['', ...effort.levels])
                    ListTile(
                      leading: Icon(
                        value == current ? Icons.radio_button_checked : Icons.radio_button_unchecked,
                        color: value == current ? MemoTheme.accent : c.textDim,
                      ),
                      title: Text(
                        value.isEmpty ? L10n.t('effort_level_default') : value,
                        style: TextStyle(
                          color: c.textMain,
                          fontWeight: value == current ? FontWeight.w600 : FontWeight.w400,
                        ),
                      ),
                      onTap: () {
                        Navigator.of(sheetContext).pop();
                        _setEffort(container, effort.config, value);
                      },
                    ),
                  const SizedBox(height: 8),
                ],
              ),
            ),
          ),
        );
      },
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

/// The chat screen's top bar. Public only so a widget test can lay it out at
/// phone widths; nothing outside this file uses it.
@visibleForTesting
class ChatTopBar extends ConsumerWidget {
  final bool showMenuButton;
   const ChatTopBar({super.key, this.showMenuButton = false});

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
    // The browser panel is a beta feature: with Beta off its entry points are
    // not shown at all.
    final betaOn = ref.watch(betaEnabledProvider);
    final browserPaneActive = ref.watch(browserSessionActiveProvider);
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
            SnackBar(content: Text(L10n.t('agent_undo_failed', {'e': FriendlyError.describeGeneric(e)})), backgroundColor: MemoTheme.red),
          );
        }
      }
    }

    Future<void> handleExport() async {
      try {
        final api = ref.read(apiClientProvider);
        final md = await api.exportChat();
        if (md.isEmpty || !context.mounted) return;
        // saveFile writes the bytes itself and returns where they landed —
        // it no longer hands back a path for the caller to write to. That
        // matters beyond tidiness: the old shape was implemented only on
        // desktop and web, so this threw UnimplementedError on Android/iOS.
        // The returned Uri is not always a file:// path (content:// on
        // Android, blob:/data: on web), hence .path ?? .toString() for the
        // confirmation message rather than assuming a filesystem path.
        final saved = await FilePicker.saveFile(
          dialogTitle: L10n.t('export_chat'),
          fileName: 'chat_export.md',
          bytes: Uint8List.fromList(utf8.encode(md)),
          mimeType: 'text/markdown',
          type: FileType.custom,
          allowedExtensions: ['md'],
        );
        if (saved != null) {
          if (context.mounted) {
            final where = saved.scheme == 'file' ? saved.toFilePath() : saved.toString();
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text(L10n.t('chat_exported', {'path': where}))),
            );
          }
        }
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(L10n.t('export_failed', {'e': FriendlyError.describeGeneric(e)}))),
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

      // Manual browser-pane toggle — independent of agent mode entirely
      // (unlike the toggles above, it doesn't change how the chat request
      // itself is built), so it's shown even for a CLI-backed chat. Opening
      // it costs nothing (openBrowserPane starts no backend process); the
      // agent can also open/drive the same pane on its own via
      // browser_navigate — both paths converge on the same
      // browserSessionActiveProvider.
      if (betaOn)
        IconButton(
          icon: Icon(
            Icons.public,
            size: 20,
            color: browserPaneActive ? MemoTheme.green : MemoTheme.of(context).textDim,
          ),
          tooltip: browserPaneActive ? L10n.t('browser_pane_close') : L10n.t('browser_pane_open'),
          onPressed: () => browserPaneActive ? closeBrowserPane(ref) : openBrowserPane(ref),
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
                _EffortSheetRow(onClose: () => Navigator.of(sheetContext).pop()),
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
                if (betaOn)
                  row(
                    icon: Icons.public,
                    label: browserPaneActive ? L10n.t('browser_pane_close') : L10n.t('browser_pane_open'),
                    isActive: browserPaneActive,
                    onTap: () => browserPaneActive ? closeBrowserPane(ref) : openBrowserPane(ref),
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

    // The status chips (incognito, agent project, agent, auto-approve). On the
    // phone layout they are icon-only with the text as a tooltip: four of them
    // with labels, plus the chat title, the model button and the menu, do not
    // fit in 360-390 px, and the project chip was clipped and drawn underneath
    // the model button. Wide layouts keep the labelled chip exactly as before.
    final compactBadges = showMenuButton;
    final badgeGap = compactBadges ? 6.0 : 12.0;
    Widget badge({
      required IconData icon,
      required Color color,
      required String label,
      required String tooltip,
      VoidCallback? onTap,
      bool flexible = false,
    }) {
      final text = Text(
        label,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: color),
      );
      Widget chip = Container(
        padding: EdgeInsets.symmetric(horizontal: compactBadges ? 6 : 8, vertical: 3),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.12),
          borderRadius: BorderRadius.circular(6),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 14, color: color),
            if (!compactBadges) ...[
              const SizedBox(width: 4),
              if (flexible) Flexible(child: text) else text,
            ],
          ],
        ),
      );
      if (onTap != null) chip = GestureDetector(onTap: onTap, child: chip);
      chip = Tooltip(message: tooltip, child: chip);
      return flexible && !compactBadges ? Flexible(child: chip) : chip;
    }

    return Container(
      height: 56,
      margin: glass
          ? const EdgeInsets.fromLTRB(12, 12, 12, 8)
          : EdgeInsets.zero,
      padding: EdgeInsets.symmetric(horizontal: showMenuButton ? 12 : 20),
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
                  badge(
                    icon: Icons.visibility_off,
                    color: MemoTheme.warmBrown,
                    label: L10n.t('incognito_on'),
                    tooltip: L10n.t('incognito_tooltip'),
                  ),
                  SizedBox(width: badgeGap),
                ],
                if (isAgentEnabled && agentProjectPath != null) ...[
                  badge(
                    icon: Icons.folder_outlined,
                    color: MemoTheme.green,
                    label: '${L10n.t('agent_chat_project')}${p.basename(agentProjectPath!)}',
                    // The full name is what the narrow, icon-only form hides.
                    tooltip: '${L10n.t('agent_chat_project')}${p.basename(agentProjectPath!)}',
                    // A long project folder name must shrink, not push the
                    // model button off the bar (it did, at phone width).
                    flexible: true,
                  ),
                  SizedBox(width: badgeGap),
                ] else if (isAgentEnabled && agentProjectPath == null) ...[
                  badge(
                    icon: Icons.psychology,
                    color: MemoTheme.accent,
                    label: L10n.t('agent_badge'),
                    tooltip: L10n.t('mode_agent_desc'),
                  ),
                  SizedBox(width: badgeGap),
                ],
                if (isAgentEnabled && isAutoPermission) ...[
                  badge(
                    icon: Icons.auto_awesome,
                    color: MemoTheme.green,
                    label: L10n.t('auto_mode_label'),
                    tooltip: L10n.t('auto_permission_tooltip'),
                    onTap: () => ref.read(agentAutoPermissionProvider.notifier).toggle(),
                  ),
                  SizedBox(width: badgeGap),
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
            _QuickModelDropdown(
              maxWidth: (MediaQuery.sizeOf(context).width * 0.42).clamp(110.0, 220.0),
            ),
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
