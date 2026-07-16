import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';
import '../providers/learning_provider.dart';

/// Floating card shown when the backend has a pending proactive suggestion
/// (see pendingProactiveSuggestionProvider's doc comment for why this exists
/// instead of the suggestion appearing as a chat message). Visible regardless
/// of which tab is active — mounted once in AppShell's overlay Stack, same as
/// VersionBanner.
class ProactiveSuggestionBanner extends ConsumerStatefulWidget {
  const ProactiveSuggestionBanner({super.key});

  @override
  ConsumerState<ProactiveSuggestionBanner> createState() => _ProactiveSuggestionBannerState();
}

class _ProactiveSuggestionBannerState extends ConsumerState<ProactiveSuggestionBanner> {
  // Tracks the suggestion id currently being responded to, so a double-tap
  // (e.g. the poll loop refreshing mid-response) can't fire two responses
  // for the same suggestion.
  String? _responding;

  Future<void> _respond(String id, String response) async {
    if (_responding == id) return;
    setState(() => _responding = id);
    try {
      final api = ref.read(apiClientProvider);
      await api.respondToSuggestion(id, response);
    } catch (_) {
      // Best-effort — the next poll tick will reconcile either way (the
      // suggestion either cleared server-side or is still pending and the
      // user can retry).
    } finally {
      // Guarded together: using ref after the widget (and its element) is
      // disposed throws, and there is nothing left to refresh for anyway.
      if (mounted) {
        setState(() => _responding = null);
        ref.invalidate(pendingProactiveSuggestionProvider);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final suggestionAsync = ref.watch(pendingProactiveSuggestionProvider);
    final suggestion = suggestionAsync.valueOrNull;
    if (suggestion == null) return const SizedBox.shrink();

    final theme = MemoTheme.of(context);
    final busy = _responding == suggestion.id;

    return Positioned(
      left: 24,
      right: 24,
      bottom: 88,
      child: Align(
        alignment: Alignment.bottomCenter,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 440),
          child: Material(
            color: Colors.transparent,
            child: Container(
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(
                color: theme.bgElement,
                borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
                border: Border.all(color: theme.borderSoft),
                boxShadow: MemoTheme.shadowLg,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(Icons.auto_awesome, size: 18, color: MemoTheme.accent),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          suggestion.message,
                          style: TextStyle(color: theme.textMain, fontSize: 13.5, height: 1.35),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  // Wrap, not Row: three buttons plus their own internal
                  // padding routinely exceed the card's width at narrower
                  // window sizes (found by
                  // proactive_suggestion_banner_test.dart — a Row overflowed
                  // by ~9px even at a plain 800x600 test viewport). Wrap
                  // just flows the last button onto its own line instead of
                  // clipping/painting past the card's edge.
                  Wrap(
                    alignment: WrapAlignment.end,
                    crossAxisAlignment: WrapCrossAlignment.center,
                    spacing: 4,
                    runSpacing: 4,
                    children: [
                      TextButton(
                        onPressed: busy ? null : () => _respond(suggestion.id, 'artık yapmıyorum'),
                        child: Text(
                          L10n.t('proactive_suggestion_stop'),
                          style: TextStyle(fontSize: 12, color: theme.textDim),
                        ),
                      ),
                      TextButton(
                        onPressed: busy ? null : () => _respond(suggestion.id, 'şimdi değil'),
                        child: Text(
                          L10n.t('proactive_suggestion_not_now'),
                          style: TextStyle(fontSize: 12, color: theme.textDim),
                        ),
                      ),
                      FilledButton(
                        onPressed: busy ? null : () => _respond(suggestion.id, 'evet'),
                        style: FilledButton.styleFrom(
                          backgroundColor: MemoTheme.accent,
                          visualDensity: VisualDensity.compact,
                        ),
                        child: Text(
                          L10n.t('proactive_suggestion_accept'),
                          style: const TextStyle(fontSize: 12, color: Colors.white),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
