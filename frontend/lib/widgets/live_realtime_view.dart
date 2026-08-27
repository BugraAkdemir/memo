import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/agent.dart';
import '../models/chat.dart';
import '../providers/live_realtime_session_provider.dart';
import 'chat_message_list.dart';

/// Full-area view shown in place of [ChatMessageList]/`WelcomeView` while a
/// Google Live/OpenAI Realtime native session is connecting or connected —
/// requested by the user after real-world testing, styled after
/// `welcome_view.dart`'s own visual language (`_FadeIn` entrance, the same
/// [MemoTheme] tokens) so it reads as part of the same app rather than a
/// bolted-on screen. The user's own spoken words and Memo's spoken replies
/// arrive as ordinary [ChatMessage] entries (see
/// `live_realtime_session_provider.dart`'s `_handleControlFrame`) and are
/// rendered by the same [ChatMessageList] this view wraps — nothing here
/// re-implements bubble rendering, and the transcript naturally stays in
/// the chat's history once the session ends (confirmed with the user: this
/// is intentional, not a bug to fix later).
class LiveRealtimeView extends ConsumerWidget {
  final List<ChatMessage> messages;
  final bool isTyping;
  final String streamingContent;
  final String streamingThinking;
  final List<AgentEvent>? streamingAgentEvents;
  final String statusText;
  final bool isCLIChat;
  final String apiBaseUrl;
  final void Function(int index, String newContent)? onEdit;
  final void Function(int index)? onDelete;

  const LiveRealtimeView({
    super.key,
    required this.messages,
    this.isTyping = false,
    this.streamingContent = '',
    this.streamingThinking = '',
    this.streamingAgentEvents,
    this.statusText = '',
    this.isCLIChat = false,
    required this.apiBaseUrl,
    this.onEdit,
    this.onDelete,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final session = ref.watch(liveRealtimeSessionProvider);
    final connecting = session.status == LiveRealtimeSessionStatus.connecting;

    return Column(
      children: [
        const SizedBox(height: 18),
        _LivePill(connecting: connecting),
        const SizedBox(height: 22),
        _FadeIn(child: const _BreathingOrb()),
        const SizedBox(height: 14),
        _FadeIn(
          delayMs: 120,
          child: Text(
            connecting ? L10n.t('live_realtime_state_connecting') : L10n.t('live_realtime_state_listening'),
            style: TextStyle(fontSize: 13, fontWeight: FontWeight.w500, color: c.textDim),
          ),
        ),
        const SizedBox(height: 18),
        Expanded(
          child: messages.isEmpty
              ? Center(
                  child: _FadeIn(
                    delayMs: 200,
                    child: Text(
                      L10n.t('live_realtime_empty_hint'),
                      style: TextStyle(fontSize: 13, color: c.textDim),
                      textAlign: TextAlign.center,
                    ),
                  ),
                )
              : ChatMessageList(
                  messages: messages,
                  isTyping: isTyping,
                  streamingContent: streamingContent,
                  streamingThinking: streamingThinking,
                  streamingAgentEvents: streamingAgentEvents,
                  statusText: statusText,
                  isCLIChat: isCLIChat,
                  apiBaseUrl: apiBaseUrl,
                  onEdit: onEdit,
                  onDelete: onDelete,
                ),
        ),
      ],
    );
  }
}

/// Small pulsing "CANLI" badge — mirrors the mockup's live indicator.
class _LivePill extends StatelessWidget {
  final bool connecting;
  const _LivePill({required this.connecting});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 6),
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: c.borderSoft),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          _PulsingDot(color: connecting ? MemoTheme.warningOrange : MemoTheme.red),
          const SizedBox(width: 7),
          Text(
            connecting ? L10n.t('live_realtime_state_connecting') : L10n.t('live_realtime_state_connected'),
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w600,
              letterSpacing: 0.3,
              color: c.textSecondary,
            ),
          ),
        ],
      ),
    );
  }
}

class _PulsingDot extends StatefulWidget {
  final Color color;
  const _PulsingDot({required this.color});

  @override
  State<_PulsingDot> createState() => _PulsingDotState();
}

class _PulsingDotState extends State<_PulsingDot> with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(vsync: this, duration: const Duration(milliseconds: 1600))..repeat(reverse: true);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _controller,
      builder: (context, child) {
        final t = _controller.value;
        return Opacity(
          opacity: 0.55 + 0.45 * (1 - t),
          child: Container(
            width: 7,
            height: 7,
            decoration: BoxDecoration(color: widget.color, shape: BoxShape.circle),
          ),
        );
      },
    );
  }
}

/// The centerpiece: a soft bronze orb that breathes while the session is
/// open — the visual direction ("Canlı Orb") the user picked from three
/// HTML mockups shown before this was built. Pure CSS-animation-equivalent
/// (scale + glow opacity, no real audio-amplitude reactivity yet — that's
/// a natural follow-up once this ships, not required for the first pass).
class _BreathingOrb extends StatefulWidget {
  const _BreathingOrb();

  @override
  State<_BreathingOrb> createState() => _BreathingOrbState();
}

class _BreathingOrbState extends State<_BreathingOrb> with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(vsync: this, duration: const Duration(milliseconds: 3400))..repeat(reverse: true);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _controller,
      builder: (context, child) {
        final t = Curves.easeInOut.transform(_controller.value);
        final scale = 1.0 + 0.08 * t;
        final glowOpacity = 0.35 + 0.4 * t;
        return SizedBox(
          width: 160,
          height: 160,
          child: Center(
            child: Stack(
              alignment: Alignment.center,
              children: [
                // Outer glow halo.
                Container(
                  width: 116 + 52 * t,
                  height: 116 + 52 * t,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    gradient: RadialGradient(
                      colors: [
                        MemoTheme.accentLight.withValues(alpha: glowOpacity * 0.35),
                        MemoTheme.accentLight.withValues(alpha: 0),
                      ],
                    ),
                  ),
                ),
                // Thin expanding ring.
                Opacity(
                  opacity: (1 - t) * 0.5,
                  child: Transform.scale(
                    scale: 1.0 + 0.22 * t,
                    child: Container(
                      width: 116,
                      height: 116,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        border: Border.all(color: MemoTheme.accentLight.withValues(alpha: 0.4)),
                      ),
                    ),
                  ),
                ),
                // The orb itself.
                Transform.scale(
                  scale: scale,
                  child: Container(
                    width: 96,
                    height: 96,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      gradient: RadialGradient(
                        center: const Alignment(-0.3, -0.4),
                        colors: [
                          MemoTheme.accentLight,
                          MemoTheme.accent,
                          const Color(0xFF7A5F38),
                        ],
                        stops: const [0.0, 0.55, 1.0],
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}

/// Subtle one-shot fade+rise entrance — the same shape as
/// `welcome_view.dart`'s own `_FadeIn` (private there, so duplicated here
/// rather than exported/shared — a 20-line, self-contained pattern, not
/// worth a cross-file dependency for).
class _FadeIn extends StatelessWidget {
  final Widget child;
  final int delayMs;
  const _FadeIn({required this.child, this.delayMs = 0});

  @override
  Widget build(BuildContext context) {
    return TweenAnimationBuilder<double>(
      tween: Tween(begin: 0, end: 1),
      duration: Duration(milliseconds: 500 + delayMs),
      curve: Curves.easeOut,
      builder: (context, value, child) => Opacity(
        opacity: value.clamp(0, 1),
        child: Transform.translate(
          offset: Offset(0, 10 * (1 - value)),
          child: child,
        ),
      ),
      child: child,
    );
  }
}
