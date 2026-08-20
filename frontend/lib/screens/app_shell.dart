import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'dart:io';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/agent.dart';
import '../providers/settings_provider.dart';
import '../providers/auth_gate_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/gate_guard.dart';
import '../providers/models_provider.dart';
import '../providers/orchestra_provider.dart';
import '../providers/provider_provider.dart';
import '../providers/skill_provider.dart';
import '../providers/tasklist_provider.dart';
import '../providers/whatsapp_provider.dart';
import '../providers/swarm_provider.dart';
import '../providers/agent_provider.dart';
import '../widgets/settings_dialog.dart';
import '../widgets/llama_installer_view.dart';
import '../widgets/backend_unreachable_view.dart';
import '../widgets/auth_gate_overlay.dart';
import '../widgets/setup_wizard_view.dart';
import '../widgets/version_banner.dart';
import '../widgets/proactive_suggestion_banner.dart';
import '../widgets/engine_strip.dart';
import '../widgets/launchpad_view.dart';
import '../widgets/spotlight_tour.dart';
import '../widgets/glass_surface.dart';
import '../widgets/chat_sidebar.dart';
import '../widgets/agent/permission_dialog.dart';
import 'chat_screen.dart';
import 'agent_screen.dart';
import 'model_store_screen.dart';
import 'whatsapp_screen.dart';
import 'calendar_screen.dart';
import 'routines_screen.dart';
import 'developer_screen.dart';
import 'swarm_screen.dart';

/// Tracks which main tab is currently selected
/// (0=chat 1=agent 2=models 3=whatsapp 4=calendar 5=routines 6=developer
/// 7=swarm). Tasks are not a top-level tab — they're opened from within the
/// Agent screen (a task list is always bound to a specific agent chat, so
/// it makes no sense to reach it from a global nav item that could be
/// visited from the plain Chat tab too). Swarm (index 7) is Beta-only and
/// additionally hidden on macOS (no rpc-server binary in the Mac release —
/// see PLAN_memo_swarm.md). Voice Live Mode used to be a separate tab here
/// (index 8) — moved into the normal chat input bar instead (see
/// providers/voice_mode_provider.dart and chat_input.dart's mic-adjacent
/// toggle button), since talking to Memo and typing to Memo are the same
/// conversation, not two different screens.
final activeTabProvider = StateProvider<int>((ref) => 0);

/// Main app shell — NavRail + content area.
class AppShell extends ConsumerStatefulWidget {
  const AppShell({super.key});

  @override
  ConsumerState<AppShell> createState() => _AppShellState();
}

class _AppShellState extends ConsumerState<AppShell> {
  // 0=chat 1=agent 2=models 3=whatsapp 4=calendar 5=routines 6=developer 7=swarm 8=live
  int _currentIndex = 0;
  bool _showLaunchpad = false;
  bool _showTour = false;

  // Auto-quit: arka uca 3 ardışık denemede ulaşılamazsa frontend kendini kapatır.
  // Eski sistemlerde iki kere tıklanıp backend kapatılınca öksüz kalan
  // frontend'in sonsuza kadar açık kalmasını engeller.
  int _consecutiveBackendFailures = 0;
  bool _hasEverConnectedToBackend = false;
  bool _backendDeadDialogShown = false;

  final _navKeys = List.generate(8, (_) => GlobalKey());

  // Lets the floating mobile-nav button (see _buildMobileNavButton) open
  // this Scaffold's drawer without needing a descendant BuildContext —
  // it's built directly inside this same build() call, above the Scaffold
  // itself, so Scaffold.of(context) isn't reachable from there.
  final GlobalKey<ScaffoldState> _scaffoldKey = GlobalKey<ScaffoldState>();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final setupComplete = ref.read(setupCompleteProvider);
      if (setupComplete) {
        _checkOnboarding();
        _startFreshChatOnLaunch();
      }
    });
  }

  /// Every cold start opens a brand-new, empty chat instead of silently
  /// resuming whatever chat — possibly still in Claude Code CLI / Codex CLI
  /// mode — was last active. Mirrors internal/replcli's Run(), which already
  /// does the same via startFreshChat(): the old chat is untouched and stays
  /// reachable from the sidebar, this just stops it from being the default.
  /// Skipped on a true first run (no chats at all yet) so it doesn't
  /// interfere with _checkOnboarding's own "chats.isEmpty" launchpad check.
  Future<void> _startFreshChatOnLaunch() async {
    final chats = await ref.read(chatListProvider.future);
    if (chats.isEmpty || !mounted) return;
    final id = await ref.read(chatListProvider.notifier).createNew();
    if (!mounted) return;
    await ref.read(activeChatIdProvider.notifier).switchTo(id);
  }

  @override
  Widget build(BuildContext context) {
    final locale = ref.watch(localeProvider);
    L10n.setLocale(locale);
    // Below this width NavRail hides entirely (it never got a mobile
    // breakpoint of its own — confirmed live to permanently eat ~19% of a
    // 375px screen, labels truncating) in favor of a single hamburger
    // opening _buildMobileNavDrawer(), reached from each screen's own menu
    // button via the ancestor Scaffold this build() returns below.
    final narrow = MediaQuery.sizeOf(context).width < MemoTheme.mobileNavBreakpoint;

    // Global error toast: any provider can report a user-facing error here and
    // it will be shown regardless of which screen is currently active.
    ref.listen<String>(errorMessageProvider, (previous, next) {
      if (next.isNotEmpty && mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(next),
            backgroundColor: MemoTheme.red,
            behavior: SnackBarBehavior.floating,
          ),
        );
        ref.read(errorMessageProvider.notifier).state = '';
      }
    });

    ref.listen<AsyncValue<AgentEvent>>(agentEventStreamProvider, (prev, next) {
      if (next.hasValue && next.value != null && mounted) {
        final event = next.value!;
        if (event.type == 'permission_request') {
          showDialog(
            context: context,
            barrierDismissible: false,
            builder: (context) => PopScope(
              canPop: false,
              child: PermissionDialog(event: event),
            ),
          );
        }
      }
    });

    ref.listen(setupCompleteProvider, (prev, next) {
      if (prev == true && next == false) {
        setState(() {
          _showLaunchpad = false;
          _showTour = false;
        });
        ref.read(launchpadSeenProvider.notifier).reset();
        ref.read(tourSeenProvider.notifier).resetTour();
      } else if (prev == false && next == true) {
        WidgetsBinding.instance.addPostFrameCallback((_) => _checkOnboarding());
      }
    });

    ref.listen(launchpadSeenProvider, (prev, next) {
      if (prev == true && next == false) {
        WidgetsBinding.instance.addPostFrameCallback((_) => _checkOnboarding());
      }
    });

    ref.listen(tourSeenProvider, (prev, next) {
      if (prev == true && next == false) {
        WidgetsBinding.instance.addPostFrameCallback((_) => _checkOnboarding());
      }
    });

    ref.listen(connectionStatusProvider, (prev, next) {
      next.whenData((connected) {
        if (connected) {
          _hasEverConnectedToBackend = true;
          _consecutiveBackendFailures = 0;
        } else if (_hasEverConnectedToBackend) {
          _consecutiveBackendFailures++;
          if (_consecutiveBackendFailures >= 3 && !_backendDeadDialogShown && mounted) {
            _backendDeadDialogShown = true;
            WidgetsBinding.instance.addPostFrameCallback((_) {
              if (mounted) _showBackendDeadDialog();
            });
          }
        }
      });
    });

    // BUG-ONB6: every one-shot AsyncNotifier below mounts a safe empty/
    // default value instead of erroring while the auth gate is blocked
    // (see each one's own comment, and chat_screen.dart's identical
    // listener for messagesProvider/chatListProvider/activeChatIdProvider,
    // the first three this bug was found on) — but a Notifier's build()
    // can't reliably ref.watch a StreamProvider itself (Riverpod leaves the
    // future pending forever), so something always-mounted has to trigger
    // the reload once the gate actually opens. AppShell is that: the one
    // widget alive for the entire app session regardless of which tab is
    // active, so this single listener covers every settings/agent/skill/
    // model/task-list/provider/orchestra/swarm screen at once instead of
    // each needing its own copy of this listener.
    ref.listen<AsyncValue<AuthGateInfo>>(authGateProvider, (prev, next) {
      if (authGateBlocked(prev?.valueOrNull) != authGateBlocked(next.valueOrNull)) {
        ref.invalidate(llamaSettingsProvider);
        ref.invalidate(systemPromptProvider);
        ref.invalidate(memoryFilesProvider);
        ref.invalidate(memorySettingsProvider);
        ref.invalidate(usageStatsProvider);
        ref.invalidate(devGatewayConfigProvider);
        ref.invalidate(memoryEnabledProvider);
        ref.invalidate(minimalModeProvider);
        ref.invalidate(minimalModeOverridesProvider);
        ref.invalidate(agentPermissionsProvider);
        ref.invalidate(skillListProvider);
        ref.invalidate(localModelsProvider);
        ref.invalidate(taskListsProvider);
        ref.invalidate(providerListProvider);
        ref.invalidate(activeProviderTypeProvider);
        ref.invalidate(orchestraConfigProvider);
        ref.invalidate(swarmStatusProvider);
        // BUG-ONB11: plain FutureProviders behind an IndexedStack screen.
        // They have no retry loop of their own, so this listener is the
        // only thing that ever refetches them. gpuInfoProvider is also
        // invalidated from auth_gate_overlay's login paths (BUG-ONB5);
        // doing it here too costs nothing and covers the gate transitions
        // those five call sites don't know about.
        ref.invalidate(gatewayModelsProvider);
        ref.invalidate(gpuInfoProvider);
        // Drives whether the Swarm tab is shown at all — a stale answer
        // here silently hands that decision to the local mirror pref.
        ref.invalidate(remoteAccessProvider);
      }
    });

    return Shortcuts(
      shortcuts: {
        const SingleActivator(LogicalKeyboardKey.tab, shift: true):
            _ToggleAutoPermissionIntent(),
      },
      child: Actions(
        actions: {
          _ToggleAutoPermissionIntent: CallbackAction<_ToggleAutoPermissionIntent>(
            onInvoke: (intent) {
              ref.read(agentAutoPermissionProvider.notifier).toggle();
              return null;
            },
          ),
        },
        child: Scaffold(
          key: _scaffoldKey,
          backgroundColor: MemoTheme.of(context).bgApp,
          drawer: narrow ? _buildMobileNavDrawer() : null,
          body: Stack(
            children: [
              // Glass Light paints a soft gradient behind everything so the
              // frosted surfaces have something to diffuse. Dark themes have no
              // gradient and fall through to the solid scaffold background.
              if (MemoTheme.of(context).backgroundGradient != null)
                Positioned.fill(
                  child: DecoratedBox(
                    decoration: BoxDecoration(
                      gradient: MemoTheme.of(context).backgroundGradient,
                    ),
                  ),
                ),
              Row(
                children: [
                  if (!narrow) _buildNavRail(),
                  Expanded(
                    child: Column(
                      children: [
                        Expanded(
                          child: IndexedStack(
                            index: _currentIndex,
                            children: [
                              ChatScreen(key: ValueKey('chat_$locale')),
                              const AgentScreen(),
                              ModelStoreScreen(
                                  key: ValueKey('models_$locale')),
                              const WhatsAppScreen(),
                              const CalendarScreen(),
                              const RoutinesScreen(),
                              const DeveloperScreen(),
                              // Always present in the stack so index 7 stays
                              // stable; the nav button is gated separately
                              // (Beta + !macOS). IndexedStack keeps this
                              // mounted forever — polling is started/stopped
                              // in _handleTabChange (KNOWN_ISSUES M04).
                              const SwarmScreen(),
                            ],
                          ),
                        ),
                        // Hidden on mobile — the model/memory status strip
                        // reads as clutter at phone width and its info is
                        // secondary to the actual conversation there.
                        if (!narrow)
                          EngineStrip(
                            onOpenModels: () => _handleTabChange(2),
                          ),
                      ],
                    ),
                  ),
                ],
              ),
              // One floating hamburger, present on every screen regardless
              // of whether that screen has its own header — Developer/
              // Models/WhatsApp/Calendar/Routines/Swarm never had a menu
              // button of their own (only Chat/Agent did, for their own
              // sidebars), so without this a mobile user navigating there
              // from _buildMobileNavDrawer had no way back. Replaces the
              // per-screen menu buttons Chat/Agent used to render.
              if (narrow) _buildMobileNavButton(),
              SetupWizardOverlay(),
              if (_showLaunchpad) _buildLaunchpadOverlay(),
              if (_showTour) _buildTourOverlay(),
              LlamaInstallerOverlay(),
              const BackendUnreachableOverlay(),
              const AuthGateOverlay(),
              const VersionBanner(),
              const ProactiveSuggestionBanner(),
            ],
          ),
        ),
      ),
    );
  }

  void _showBackendDeadDialog() {
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('backend_dead_title')),
        content: Text(L10n.t('backend_dead_body')),
        actions: [
          // A backend can go dead because a custom Remote Access URL
          // (Settings) points at a server that's since been shut down —
          // "restart the app" alone doesn't help there, it just reloads the
          // same dead URL. Opening Settings first lets the user clear/fix
          // it without forcing a full quit.
          TextButton(
            onPressed: () {
              Navigator.of(ctx).pop();
              showDialog(
                context: context,
                builder: (context) => SettingsDialog(
                  initialTab: SettingsDialog.remoteAccessTabIndex,
                ),
              );
            },
            child: Text(L10n.t('settings')),
          ),
          TextButton(
            onPressed: () => exit(0),
            child: Text(L10n.t('ok')),
          ),
        ],
      ),
    );
  }

  void _checkOnboarding() {
    if (!ref.read(setupCompleteProvider)) return;

    final launchpadSeen = ref.read(launchpadSeenProvider);
    final tourSeen = ref.read(tourSeenProvider);

    if (!launchpadSeen) {
      final forceShow = ref.read(launchpadSeenProvider.notifier).forceShow;
      if (forceShow) {
        setState(() => _showLaunchpad = true);
        return;
      }
      final chatsAsync = ref.read(chatListProvider);
      chatsAsync.whenData((chats) {
        if (chats.isEmpty && mounted) {
          setState(() => _showLaunchpad = true);
        } else {
          ref.read(launchpadSeenProvider.notifier).markSeen();
          if (!tourSeen && mounted) setState(() => _showTour = true);
        }
      });
    } else if (!tourSeen) {
      if (mounted) setState(() => _showTour = true);
    }
  }

  Widget _buildNavRail() {
    final c = MemoTheme.of(context);
    final glass = c.isGlass;
    // Incognito is a single global backend flag (internal/app/chat.go),
    // not scoped to the chat screen — but its only visible cue used to be
    // the chat screen's own red background/pill, invisible from every
    // other tab. A user who switched to Settings/Model Store/WhatsApp
    // while it was on had no way to notice it was still on. This dot on
    // the logo (always present in the rail, regardless of active tab) is
    // the app-wide equivalent.
    final isIncognito = ref.watch(incognitoProvider);
    return Padding(
      // In Glass Light the rail floats off the window edge as a rounded card;
      // dark keeps it flush against the edge.
      padding: glass
          ? const EdgeInsets.fromLTRB(12, 12, 8, 12)
          : EdgeInsets.zero,
      child: SizedBox(
        width: glass ? 64 : 72,
          child: GlassSurface(
            borderRadius: glass
                ? BorderRadius.circular(MemoTheme.radiusLg)
                : BorderRadius.zero,
            border: glass
              ? Border.all(color: c.borderSoft)
              : Border(right: BorderSide(color: c.borderSoft)),
          shadow: glass ? MemoTheme.shadowLg : null,
          child: Column(
        children: [
          const SizedBox(height: 14),

          // ─── Logo (+ incognito indicator) ─────────────
          _buildLogoWithIncognitoDot(c, isIncognito),

          const SizedBox(height: 20),

          _NavRailButton(
            key: _navKeys[0],
            icon: Icons.chat_bubble_outline,
            activeIcon: Icons.chat_bubble,
            label: L10n.t('chats'),
            isActive: _currentIndex == 0,
            onTap: () => _handleTabChange(0),
          ),

          _NavRailButton(
            key: _navKeys[1],
            icon: Icons.smart_toy_outlined,
            activeIcon: Icons.smart_toy,
            label: 'Ajan',
            isActive: _currentIndex == 1,
            onTap: () => _handleTabChange(1),
          ),

          _NavRailButton(
            key: _navKeys[2],
            icon: Icons.memory_outlined,
            activeIcon: Icons.memory,
            label: L10n.t('model_store'),
            isActive: _currentIndex == 2,
            onTap: () => _handleTabChange(2),
          ),

          _NavRailButton(
            key: _navKeys[3],
            icon: Icons.message_outlined,
            activeIcon: Icons.message,
            label: 'WhatsApp',
            isActive: _currentIndex == 3,
            onTap: () => _handleTabChange(3),
          ),

          _NavRailButton(
            key: _navKeys[4],
            icon: Icons.calendar_month_outlined,
            activeIcon: Icons.calendar_month,
            label: 'Takvim',
            isActive: _currentIndex == 4,
            onTap: () => _handleTabChange(4),
          ),

          _NavRailButton(
            key: _navKeys[5],
            icon: Icons.schedule_outlined,
            activeIcon: Icons.schedule,
            label: L10n.t('routines_title'),
            isActive: _currentIndex == 5,
            onTap: () => _handleTabChange(5),
          ),

          _NavRailButton(
            key: _navKeys[6],
            icon: Icons.code_outlined,
            activeIcon: Icons.code,
            label: L10n.t('tab_dev_gateway'),
            isActive: _currentIndex == 6,
            onTap: () => _handleTabChange(6),
          ),

          // Swarm: Beta-only, and macOS has no rpc-server binary in the
          // bundled release (PLAN_memo_swarm.md Stage 0 verification).
          if (_showSwarmNav())
            _NavRailButton(
              key: _navKeys[7],
              icon: Icons.hub_outlined,
              activeIcon: Icons.hub,
              label: L10n.t('tab_swarm'),
              isActive: _currentIndex == 7,
              onTap: () => _handleTabChange(7),
            ),

          const Spacer(),

          _NavRailButton(
            icon: Icons.settings_outlined,
            activeIcon: Icons.settings,
            label: L10n.t('settings'),
            isActive: false,
            onTap: () => showDialog(
              context: context,
              builder: (context) => SettingsDialog(),
            ),
          ),

          const SizedBox(height: 12),
        ],
        ),
        ),
      ),
    );
  }

  // Mobile replacement for the always-inline NavRail above: one hamburger
  // (opened from whichever screen's own menu button — Scaffold.of(context)
  // finds this Scaffold's drawer since narrow-mode screens no longer wrap
  // themselves in their own Scaffold) with two tabs. "Sohbetler" reuses the
  // exact same ChatSidebar the desktop Chat tab shows inline — picking or
  // creating a chat there also switches to the Chat tab and closes the
  // drawer, since ChatSidebar only ever sets the globally-shared active
  // chat and has no idea which tab is currently visible. "Menü" is
  // NavRail's own destination list, laid out as a full-width, fully-labeled
  // list instead of 9px icon labels — the thing that actually needed
  // fixing (NavRail was never given a mobile breakpoint at all).
  Widget _buildMobileNavDrawer() {
    final onChatsTab = _currentIndex == 0 || _currentIndex == 1;
    return Drawer(
      width: 300,
      child: SafeArea(
        child: DefaultTabController(
          length: 2,
          initialIndex: onChatsTab ? 0 : 1,
          child: Column(
            children: [
              TabBar(
                labelColor: MemoTheme.accent,
                unselectedLabelColor: MemoTheme.of(context).textDim,
                indicatorColor: MemoTheme.accent,
                tabs: [
                  Tab(text: L10n.t('mobile_nav_chats_tab')),
                  Tab(text: L10n.t('mobile_nav_menu_tab')),
                ],
              ),
              Expanded(
                child: TabBarView(
                  children: [
                    ChatSidebar(
                      onChatSelected: () {
                        _handleTabChange(0);
                        Navigator.of(context).pop();
                      },
                    ),
                    _buildMobileNavMenu(),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildMobileNavButton() {
    final c = MemoTheme.of(context);
    return Positioned(
      top: 12,
      left: 12,
      child: SafeArea(
        bottom: false,
        child: Material(
          color: c.bgPanel,
          shape: const CircleBorder(),
          elevation: 2,
          child: InkWell(
            customBorder: const CircleBorder(),
            onTap: () => _scaffoldKey.currentState?.openDrawer(),
            child: Padding(
              padding: const EdgeInsets.all(10),
              child: Icon(Icons.menu, size: 20, color: c.textMain),
            ),
          ),
        ),
      ),
    );
  }

  void _selectMobileTab(int index) {
    _handleTabChange(index);
    Navigator.of(context).pop();
  }

  Widget _buildMobileNavMenu() {
    final c = MemoTheme.of(context);

    Widget item({
      required IconData icon,
      required IconData activeIcon,
      required String label,
      required bool isActive,
      required VoidCallback onTap,
    }) {
      return ListTile(
        leading: Icon(isActive ? activeIcon : icon, color: isActive ? MemoTheme.accent : c.textDim),
        title: Text(
          label,
          style: TextStyle(
            color: isActive ? MemoTheme.accent : c.textMain,
            fontWeight: isActive ? FontWeight.w600 : FontWeight.w400,
          ),
        ),
        selected: isActive,
        selectedTileColor: MemoTheme.accentMuted,
        onTap: onTap,
      );
    }

    return ListView(
      padding: const EdgeInsets.symmetric(vertical: 8),
      children: [
        item(
          icon: Icons.chat_bubble_outline,
          activeIcon: Icons.chat_bubble,
          label: L10n.t('chats'),
          isActive: _currentIndex == 0,
          onTap: () => _selectMobileTab(0),
        ),
        item(
          icon: Icons.smart_toy_outlined,
          activeIcon: Icons.smart_toy,
          label: 'Ajan',
          isActive: _currentIndex == 1,
          onTap: () => _selectMobileTab(1),
        ),
        item(
          icon: Icons.memory_outlined,
          activeIcon: Icons.memory,
          label: L10n.t('model_store'),
          isActive: _currentIndex == 2,
          onTap: () => _selectMobileTab(2),
        ),
        item(
          icon: Icons.message_outlined,
          activeIcon: Icons.message,
          label: 'WhatsApp',
          isActive: _currentIndex == 3,
          onTap: () => _selectMobileTab(3),
        ),
        item(
          icon: Icons.calendar_month_outlined,
          activeIcon: Icons.calendar_month,
          label: 'Takvim',
          isActive: _currentIndex == 4,
          onTap: () => _selectMobileTab(4),
        ),
        item(
          icon: Icons.schedule_outlined,
          activeIcon: Icons.schedule,
          label: L10n.t('routines_title'),
          isActive: _currentIndex == 5,
          onTap: () => _selectMobileTab(5),
        ),
        item(
          icon: Icons.code_outlined,
          activeIcon: Icons.code,
          label: L10n.t('tab_dev_gateway'),
          isActive: _currentIndex == 6,
          onTap: () => _selectMobileTab(6),
        ),
        if (_showSwarmNav())
          item(
            icon: Icons.hub_outlined,
            activeIcon: Icons.hub,
            label: L10n.t('tab_swarm'),
            isActive: _currentIndex == 7,
            onTap: () => _selectMobileTab(7),
          ),
        const Divider(height: 24),
        item(
          icon: Icons.settings_outlined,
          activeIcon: Icons.settings,
          label: L10n.t('settings'),
          isActive: false,
          onTap: () {
            Navigator.of(context).pop();
            showDialog(context: context, builder: (context) => SettingsDialog());
          },
        ),
      ],
    );
  }

  // Small red dot pinned to the logo's corner when incognito is on — see
  // _buildNavRail's isIncognito comment for why this lives on the always-
  // visible logo rather than only on the chat screen.
  Widget _buildLogoWithIncognitoDot(ThemeColors c, bool isIncognito) {
    final logo = Stack(
      clipBehavior: Clip.none,
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(12),
          child: Image.asset(
            'lib/icon/memo.png',
            width: 36,
            height: 36,
            fit: BoxFit.cover,
          ),
        ),
        if (isIncognito)
          Positioned(
            right: -2,
            top: -2,
            child: Container(
              width: 12,
              height: 12,
              decoration: BoxDecoration(
                color: MemoTheme.red,
                shape: BoxShape.circle,
                border: Border.all(color: c.bgPanel, width: 2),
              ),
            ),
          ),
      ],
    );
    if (!isIncognito) return logo;
    return Tooltip(message: L10n.t('incognito_mode'), child: logo);
  }

  Widget _buildLaunchpadOverlay() {
    return Positioned.fill(
      child: Material(
        color: MemoTheme.of(context).bgApp,
        child: LaunchpadView(
          whatsAppConnected: _isWhatsAppConnected(),
          onStartChat: () {
            ref.read(launchpadSeenProvider.notifier).markSeen();
            setState(() => _showLaunchpad = false);
            _handleTabChange(0);
            _maybeStartTour();
          },
          onNavigateAgent: () {
            ref.read(launchpadSeenProvider.notifier).markSeen();
            setState(() => _showLaunchpad = false);
            _handleTabChange(1);
            _maybeStartTour();
          },
          onNavigateWhatsApp: () {
            ref.read(launchpadSeenProvider.notifier).markSeen();
            setState(() => _showLaunchpad = false);
            _handleTabChange(3);
            _maybeStartTour();
          },
          onNavigateCalendar: () {
            ref.read(launchpadSeenProvider.notifier).markSeen();
            setState(() => _showLaunchpad = false);
            _handleTabChange(4);
            _maybeStartTour();
          },
          onNavigateModels: () {
            ref.read(launchpadSeenProvider.notifier).markSeen();
            setState(() => _showLaunchpad = false);
            _handleTabChange(2);
            _maybeStartTour();
          },
        ),
      ),
    );
  }

  void _maybeStartTour() {
    final tourSeen = ref.read(tourSeenProvider);
    if (!tourSeen && mounted) {
      // Delay to let the nav rail render after the tab switch
      Future.delayed(const Duration(milliseconds: 400), () {
        if (mounted) setState(() => _showTour = true);
      });
    }
  }

  Widget _buildTourOverlay() {
    final steps = <String>[
      L10n.t('tour_step_chat'),
      L10n.t('tour_step_agent'),
      L10n.t('tour_step_whatsapp'),
      L10n.t('tour_step_calendar'),
    ];
    return Positioned.fill(
      child: SpotlightTour(
        targetKeys: [..._navKeys.take(4)],
        stepTexts: steps,
        onComplete: () {
          ref.read(tourSeenProvider.notifier).markSeen();
          setState(() => _showTour = false);
        },
        onSkip: () {
          ref.read(tourSeenProvider.notifier).markSeen();
          setState(() => _showTour = false);
        },
      ),
    );
  }

  void _handleTabChange(int index) {
    setState(() => _currentIndex = index);
    ref.read(activeTabProvider.notifier).state = index;
    final waNotifier = ref.read(whatsAppStatusProvider.notifier);
    if (index == 3) {
      waNotifier.startPolling();
    } else {
      waNotifier.stopPolling();
    }
    final logsNotifier = ref.read(gatewayLogsProvider.notifier);
    if (index == 6) {
      logsNotifier.startPolling();
    } else {
      logsNotifier.stopPolling();
    }
    // Swarm polling (index 7) — required because IndexedStack keeps every
    // screen mounted forever (KNOWN_ISSUES M04); without start/stop here
    // the swarm tab would poll in the background forever once first opened.
    final swarmNotifier = ref.read(swarmStatusProvider.notifier);
    if (index == 7) {
      swarmNotifier.startPolling();
    } else {
      swarmNotifier.stopPolling();
    }
  }

  bool _isWhatsAppConnected() {
    final statusAsync = ref.read(whatsAppStatusProvider);
    return statusAsync.valueOrNull?.connected ?? false;
  }

  /// Swarm nav is Beta-gated (backend truth via remoteAccessProvider) and
  /// hidden on macOS (no rpc-server binary in the Mac release). dart:io's
  /// Platform throws UnsupportedError the instant any of its getters are
  /// touched on web (confirmed live: this exact line was why the whole
  /// app rendered a blank screen and nothing else on the page ever got a
  /// chance to run — app_shell is the root shell, built immediately after
  /// boot) — kIsWeb must be checked first, every time, never Platform.*
  /// unguarded.
  bool _showSwarmNav() {
    if (!kIsWeb && Platform.isMacOS) return false;
    final ra = ref.watch(remoteAccessProvider).valueOrNull;
    // The backend's answer is authoritative once it actually arrives — it
    // owns cfg.Beta, the local pref is only a mirror of it. The old code
    // fell through to that mirror whenever the answer was anything but
    // beta:true, so a stale local true kept the Swarm tab visible against
    // a backend with beta:false, permanently. Reported live.
    //
    // "Arrived" has to be tested by the key, not by non-null:
    // remoteAccessProvider swallows its own failure into {'enabled': false},
    // which is indistinguishable from a real response except that the real
    // one always carries 'beta' (RemoteAccessStatus.Beta, no omitempty).
    if (ra != null && ra.containsKey('beta')) return ra['beta'] == true;
    return ref.watch(betaFeaturesProvider);
  }

}

class _NavRailButton extends StatelessWidget {
  final IconData icon;
  final IconData activeIcon;
  final String label;
  final bool isActive;
  final VoidCallback onTap;

  const _NavRailButton({
    super.key,
    required this.icon,
    required this.activeIcon,
    required this.label,
    required this.isActive,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final hue = isActive ? MemoTheme.accent : c.textDim;

    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 150),
          width: 52,
          padding: const EdgeInsets.symmetric(vertical: 6),
          decoration: BoxDecoration(
            color: isActive ? MemoTheme.accentMuted : Colors.transparent,
            borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                isActive ? activeIcon : icon,
                size: 20,
                color: hue,
              ),
              const SizedBox(height: 2),
              Text(
                label,
                style: TextStyle(
                  fontSize: 9,
                  fontWeight: isActive ? FontWeight.w600 : FontWeight.w400,
                  color: hue,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ToggleAutoPermissionIntent extends Intent {
  const _ToggleAutoPermissionIntent();
}
