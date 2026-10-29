import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/clipboard_copy.dart';
import '../core/friendly_error.dart';
import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/telemetry_provider.dart';

/// What the optional daily summary contains and does not contain, plus the exact
/// text that would be sent next. Shared by Settings › Privacy and the setup
/// wizard, so the person sees the same words in both places.
class TelemetryDetails extends ConsumerStatefulWidget {
  const TelemetryDetails({super.key});

  @override
  ConsumerState<TelemetryDetails> createState() => _TelemetryDetailsState();
}

class _TelemetryDetailsState extends ConsumerState<TelemetryDetails> {
  /// The preview is only fetched once asked for: reading the day's usage to
  /// build it is work nobody needs until they want to look.
  bool _showPreview = false;

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final heading = TextStyle(fontWeight: FontWeight.bold, fontSize: 14, color: theme.textMain);
    final body = TextStyle(height: 1.5, fontSize: 13, color: theme.textMuted);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(L10n.t('telemetry_never_title'), style: heading),
        const SizedBox(height: 4),
        Text(L10n.t('telemetry_never_body'), style: body),
        const SizedBox(height: 14),
        Text(L10n.t('telemetry_sent_title'), style: heading),
        const SizedBox(height: 4),
        Text(L10n.t('telemetry_sent_body'), style: body),
        const SizedBox(height: 10),
        Align(
          alignment: Alignment.centerLeft,
          child: TextButton.icon(
            key: const Key('telemetryPreviewToggle'),
            onPressed: () => setState(() => _showPreview = !_showPreview),
            icon: Icon(_showPreview ? Icons.visibility_off_outlined : Icons.visibility_outlined, size: 18),
            label: Text(L10n.t(_showPreview ? 'telemetry_preview_hide' : 'telemetry_preview_show')),
          ),
        ),
        if (_showPreview) const _PreviewBox(),
      ],
    );
  }
}

class _PreviewBox extends ConsumerWidget {
  const _PreviewBox();

  /// Indented only for reading; the copy button hands over the exact text.
  static String _pretty(String raw) {
    try {
      return const JsonEncoder.withIndent('  ').convert(jsonDecode(raw));
    } catch (_) {
      return raw;
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    final async = ref.watch(telemetryPreviewProvider);
    return async.when(
      loading: () => const Padding(
        padding: EdgeInsets.all(12),
        child: SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2)),
      ),
      error: (e, _) => Text(
        L10n.t('telemetry_failed', {'error': FriendlyError.describeGeneric(e)}),
        style: TextStyle(color: theme.textDim, fontSize: 13),
      ),
      data: (p) {
        if (p.empty) {
          return Text(L10n.t('telemetry_preview_empty'), style: TextStyle(color: theme.textDim, fontSize: 13, height: 1.5));
        }
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(L10n.t('telemetry_preview_hint', {'day': p.day}), style: TextStyle(color: theme.textDim, fontSize: 12, height: 1.5)),
            const SizedBox(height: 6),
            Container(
              key: const Key('telemetryPreviewJson'),
              constraints: const BoxConstraints(maxHeight: 260),
              width: double.infinity,
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: theme.bgElement,
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: theme.borderSoft),
              ),
              child: SingleChildScrollView(
                child: SelectableText(
                  _pretty(p.json),
                  style: TextStyle(fontFamily: 'JetBrains Mono', fontSize: 11.5, height: 1.45, color: theme.textMain),
                ),
              ),
            ),
            Align(
              alignment: Alignment.centerLeft,
              child: TextButton.icon(
                onPressed: () async {
                  final messenger = ScaffoldMessenger.of(context);
                  final ok = await copyToClipboard(p.json);
                  if (ok) messenger.showSnackBar(SnackBar(content: Text(L10n.t('copied'))));
                },
                icon: const Icon(Icons.copy, size: 16),
                label: Text(L10n.t('copy')),
              ),
            ),
          ],
        );
      },
    );
  }
}
