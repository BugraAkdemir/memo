import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'dart:io';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/agent.dart';
import '../providers/settings_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/whatsapp_provider.dart';
import '../providers/swarm_provider.dart';
import '../providers/agent_provider.dart';
import '../widgets/settings_dialog.dart';
import '../widgets/llama_installer_view.dart';
import '../widgets/backend_unreachable_view.dart';
import '../widgets/setup_wizard_view.dart';
import '../widgets/version_banner.dart';
import '../widgets/proactive_suggestion_banner.dart';
import '../widgets/engine_strip.dart';
import '../widgets/launchpad_view.dart';
import '../widgets/spotlight_tour.dart';
import '../widgets/glass_surface.dart';
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
          backgroundColor: MemoTheme.of(context).bgApp,
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
                  _buildNavRail(),
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
                        EngineStrip(
                          onOpenModels: () => _handleTabChange(2),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              SetupWizardOverlay(),
              if (_showLaunchpad) _buildLaunchpadOverlay(),
              if (_showTour) _buildTourOverlay(),
              LlamaInstallerOverlay(),
              const BackendUnreachableOverlay(),
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

          // ─── Logo ────────────────────────────────────
          ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: Image.asset(
              'lib/icon/memo.png',
              width: 36,
              height: 36,
              fit: BoxFit.cover,
            ),
          ),

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
    if (ra != null && ra['beta'] == true) return true;
    // Fallback to the local prefs toggle used elsewhere until remote status loads.
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
