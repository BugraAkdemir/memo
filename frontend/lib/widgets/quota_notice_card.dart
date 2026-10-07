import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/model_display.dart';
import '../providers/chat_provider.dart';
import '../providers/quota_notice_provider.dart';
import 'quota_badge.dart';

/// Sits between the conversation and the message box. Two cases:
///
///  * the allowance ran out — says when it refills and, by default, will type
///    "continue" in the chat by itself once it has (unticked, it only offers a
///    "Continue now" button);
///  * the allowance is running low — a heads-up, once per allowance window.
///
/// Renders nothing when the open chat has no notice.
class QuotaNoticeCard extends ConsumerStatefulWidget {
  /// The clock the countdown reads; tests move it by hand.
  final DateTime Function()? clock;

  const QuotaNoticeCard({super.key, this.clock});

  @override
  ConsumerState<QuotaNoticeCard> createState() => _QuotaNoticeCardState();
}

class _QuotaNoticeCardState extends ConsumerState<QuotaNoticeCard> {
  Timer? _tick;

  @override
  void dispose() {
    _tick?.cancel();
    super.dispose();
  }

  /// A once-a-second repaint for the countdown, only while a card is showing.
  void _ensureTicking(bool showing) {
    if (showing && _tick == null) {
      _tick = Timer.periodic(const Duration(seconds: 1), (_) {
        if (mounted) setState(() {});
      });
    } else if (!showing && _tick != null) {
      _tick!.cancel();
      _tick = null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final chatId = ref.watch(activeChatIdProvider).valueOrNull ?? '';
    final notice = chatId.isEmpty ? null : ref.watch(quotaNoticeProvider)[chatId];
    final auto = ref.watch(quotaAutoContinueProvider);
    final show = notice != null && (notice.exhausted || notice.low);
    // Schedule after the frame so the timer is never started mid-build.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _ensureTicking(show && notice.exhausted);
    });
    if (!show) return const SizedBox.shrink();

    final c = MemoTheme.of(context);
    final tone = notice.exhausted ? MemoTheme.red : MemoTheme.warningOrange;
    final model = notice.model.isEmpty ? '' : prettyModelName(notice.model).label;
    final now = (widget.clock ?? DateTime.now)();

    final lines = <Widget>[];
    if (notice.exhausted) {
      if (model.isNotEmpty) {
        lines.add(_line(c, L10n.t('quota_exhausted_for', {'model': model})));
      }
      if (notice.knownReset) {
        lines.add(_line(
          c,
          L10n.t('quota_resets_in_at', {
            'when': QuotaBadge.formatDuration(notice.resetAt!.difference(now)),
            'at': QuotaBadge.formatAt(notice.resetAt!),
          }),
        ));
      } else {
        lines.add(_line(
          c,
          L10n.t('quota_retry_in', {'when': QuotaBadge.formatDuration(notice.resumeAt.difference(now))}),
        ));
      }
    } else {
      final pct = '${notice.remainingPercent < 0 ? 0 : notice.remainingPercent}';
      lines.add(_line(
        c,
        model.isEmpty
            ? L10n.t('quota_low_body_nomodel', {'pct': pct})
            : L10n.t('quota_low_body', {'model': model, 'pct': pct}),
      ));
      if (notice.resetAt != null && notice.resetAt!.isAfter(now)) {
        lines.add(_line(
          c,
          L10n.t('quota_resets_in_at', {
            'when': QuotaBadge.formatDuration(notice.resetAt!.difference(now)),
            'at': QuotaBadge.formatAt(notice.resetAt!),
          }),
        ));
      }
    }

    return Container(
      key: const Key('quotaNoticeCard'),
      margin: const EdgeInsets.fromLTRB(16, 6, 16, 6),
      padding: const EdgeInsets.fromLTRB(12, 10, 4, 10),
      decoration: BoxDecoration(
        color: tone.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: tone.withValues(alpha: 0.35)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 1),
            child: Icon(notice.exhausted ? Icons.hourglass_bottom_rounded : Icons.warning_amber_rounded,
                size: 20, color: tone),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  L10n.t(notice.exhausted ? 'quota_exhausted_title' : 'quota_low_title'),
                  style: TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: c.textMain),
                ),
                const SizedBox(height: 2),
                ...lines,
                if (notice.exhausted) ...[
                  const SizedBox(height: 4),
                  Wrap(
                    crossAxisAlignment: WrapCrossAlignment.center,
                    spacing: 8,
                    runSpacing: 2,
                    children: [
                      InkWell(
                        key: const Key('quotaAutoContinueToggle'),
                        borderRadius: BorderRadius.circular(6),
                        onTap: () => ref.read(quotaAutoContinueProvider.notifier).set(!auto),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            SizedBox(
                              width: 24,
                              height: 24,
                              child: Checkbox(
                                value: auto,
                                visualDensity: VisualDensity.compact,
                                activeColor: MemoTheme.accent,
                                onChanged: (v) => ref.read(quotaAutoContinueProvider.notifier).set(v ?? true),
                              ),
                            ),
                            const SizedBox(width: 4),
                            Flexible(
                              child: Text(
                                L10n.t('quota_auto_continue'),
                                style: TextStyle(fontSize: 12, color: c.textMain),
                              ),
                            ),
                          ],
                        ),
                      ),
                      TextButton(
                        key: const Key('quotaContinueNow'),
                        style: TextButton.styleFrom(
                          visualDensity: VisualDensity.compact,
                          padding: const EdgeInsets.symmetric(horizontal: 8),
                          minimumSize: const Size(0, 28),
                        ),
                        onPressed: () => ref.read(quotaNoticeProvider.notifier).continueNow(chatId),
                        child: Text(L10n.t('quota_continue_now'), style: const TextStyle(fontSize: 12)),
                      ),
                    ],
                  ),
                  if (auto) _line(c, L10n.t('quota_auto_hint')),
                ],
              ],
            ),
          ),
          IconButton(
            key: const Key('quotaNoticeDismiss'),
            tooltip: L10n.t('close'),
            visualDensity: VisualDensity.compact,
            iconSize: 16,
            color: c.textDim,
            icon: const Icon(Icons.close),
            onPressed: () => ref.read(quotaNoticeProvider.notifier).dismiss(chatId),
          ),
        ],
      ),
    );
  }

  Widget _line(ThemeColors c, String text) => Padding(
        padding: const EdgeInsets.only(top: 1),
        child: Text(text, style: TextStyle(fontSize: 12, height: 1.35, color: c.textDim)),
      );
}
