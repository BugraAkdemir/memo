import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../core/app_restart.dart';
import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/app_update.dart';
import '../providers/update_provider.dart';

/// The "a new Memo is available" window.
///
/// Three shapes, decided by the backend's `mode`:
///  - auto / installer: one click updates (Memo downloads it, verifies it and
///    restarts — or hands over to the Windows installer);
///  - manual: this install can't update itself (AppImage, Docker, …), so the
///    window points at the download page;
///  - a required update hides "Skip this version" but keeps "Later".
class AppUpdateDialog extends ConsumerWidget {
  const AppUpdateDialog({super.key, required this.info});
  final AppUpdateInfo info;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    final flow = ref.watch(updateFlowProvider);
    final running = flow.running || flow.progress.isRestarting;
    final note = info.noteFor(L10n.locale == MemoLocale.tr).trim();
    final size = formatUpdateBytes(info.sizeBytes);

    return PopScope(
      canPop: !running,
      child: Dialog(
        insetPadding: const EdgeInsets.symmetric(horizontal: 24, vertical: 40),
        backgroundColor: theme.bgPanel,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusLg)),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 460),
          child: Padding(
            padding: const EdgeInsets.all(22),
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(children: [
                    Icon(Icons.system_update_alt_rounded, color: MemoTheme.accent, size: 22),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Text(
                        L10n.t('update_title'),
                        style: TextStyle(color: theme.textMain, fontSize: 17, fontWeight: FontWeight.w600),
                      ),
                    ),
                  ]),
                  const SizedBox(height: 14),
                  Text(
                    L10n.t('update_versions', {'current': info.current, 'latest': info.latest}),
                    key: const Key('update_versions'),
                    style: TextStyle(color: theme.textMain, fontSize: 14, fontWeight: FontWeight.w500),
                  ),
                  if (size.isNotEmpty && info.canUpdateInPlace && !running) ...[
                    const SizedBox(height: 4),
                    Text(
                      L10n.t('update_size', {'size': size}),
                      style: TextStyle(color: theme.textMuted, fontSize: 12.5),
                    ),
                  ],
                  if (note.isNotEmpty) ...[
                    const SizedBox(height: 14),
                    Text(L10n.t('update_whats_new'),
                        style: TextStyle(color: theme.textSecondary, fontSize: 12, fontWeight: FontWeight.w600)),
                    const SizedBox(height: 4),
                    // Plain Text on purpose: the note comes from a file nobody signs.
                    Text(note, key: const Key('update_note'), style: TextStyle(color: theme.textSecondary, fontSize: 13.5, height: 1.4)),
                  ],
                  if (info.required && !running) ...[
                    const SizedBox(height: 12),
                    Text(L10n.t('update_required_hint'),
                        key: const Key('update_required_hint'),
                        style: const TextStyle(color: MemoTheme.warningOrange, fontSize: 12.5)),
                  ],
                  if (!info.canUpdateInPlace) ...[
                    const SizedBox(height: 12),
                    Text(L10n.t('update_manual_body'),
                        key: const Key('update_manual_body'),
                        style: TextStyle(color: theme.textMuted, fontSize: 12.5, height: 1.4)),
                  ],
                  if (running || flow.progress.isDone) ...[
                    const SizedBox(height: 16),
                    _ProgressView(flow: flow, windowsInstaller: info.mode == 'installer'),
                  ],
                  if (flow.error.isNotEmpty) ...[
                    const SizedBox(height: 12),
                    Text(L10n.t('update_failed', {'err': flow.error}),
                        key: const Key('update_error'), style: const TextStyle(color: MemoTheme.red, fontSize: 12.5)),
                  ],
                  const SizedBox(height: 18),
                  _Actions(info: info, flow: flow),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _ProgressView extends StatelessWidget {
  const _ProgressView({required this.flow, required this.windowsInstaller});
  final UpdateFlow flow;
  final bool windowsInstaller;

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final p = flow.progress;
    String label;
    if (flow.backendBack) {
      label = L10n.t('update_back');
    } else if (p.isRestarting) {
      label = windowsInstaller ? L10n.t('update_windows_hint') : L10n.t('update_restarting');
    } else if (p.state == 'downloading') {
      final total = formatUpdateBytes(p.total);
      label = total.isEmpty
          ? L10n.t('update_downloading')
          : L10n.t('update_downloading_of', {'done': formatUpdateBytes(p.done).isEmpty ? '0 B' : formatUpdateBytes(p.done), 'total': total});
    } else if (p.state == 'installing') {
      label = L10n.t('update_installing');
    } else if (p.isDone) {
      label = L10n.t('update_done');
    } else {
      label = L10n.t('update_preparing');
    }
    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      if (!p.isDone && !flow.backendBack)
        ClipRRect(
          borderRadius: BorderRadius.circular(4),
          child: LinearProgressIndicator(
            key: const Key('update_progress'),
            value: p.state == 'downloading' ? p.fraction : null,
            minHeight: 6,
            backgroundColor: theme.bgElement,
            color: MemoTheme.accent,
          ),
        ),
      const SizedBox(height: 8),
      Text(label, key: const Key('update_status_label'), style: TextStyle(color: theme.textSecondary, fontSize: 12.5)),
      if (p.isRestarting && !windowsInstaller)
        Padding(
          padding: const EdgeInsets.only(top: 4),
          child: Text(L10n.t('update_restarting_hint'), style: TextStyle(color: theme.textMuted, fontSize: 12)),
        ),
    ]);
  }
}

class _Actions extends ConsumerWidget {
  const _Actions({required this.info, required this.flow});
  final AppUpdateInfo info;
  final UpdateFlow flow;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final notifier = ref.read(updateFlowProvider.notifier);
    final busy = flow.running || flow.progress.isRestarting;

    // The backend came back after restarting: that, not "restarting", is the
    // state to act on.
    if (flow.backendBack) {
      return Align(
        alignment: Alignment.centerRight,
        child: FilledButton(
          key: const Key('update_reload'),
          onPressed: () {
            if (kIsWeb) {
              restartApp(); // reloads the page on web
            } else if (Navigator.of(context).canPop()) {
              Navigator.of(context).pop();
            }
          },
          child: Text(L10n.t(kIsWeb ? 'update_reload' : 'update_close')),
        ),
      );
    }

    if (busy) return const SizedBox.shrink();

    return Wrap(
      alignment: WrapAlignment.end,
      spacing: 8,
      runSpacing: 8,
      children: [
        if (!info.required)
          TextButton(
            key: const Key('update_skip'),
            onPressed: () async {
              await ref.read(skippedUpdateProvider.notifier).skip(info.latest);
              if (context.mounted) Navigator.of(context).pop();
            },
            child: Text(L10n.t('update_skip')),
          ),
        TextButton(
          key: const Key('update_later'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(L10n.t('update_later')),
        ),
        if (info.canUpdateInPlace)
          FilledButton(
            key: const Key('update_now'),
            onPressed: () => notifier.start('app'),
            child: Text(L10n.t(flow.error.isNotEmpty ? 'update_retry' : 'update_now')),
          )
        else
          FilledButton(
            key: const Key('update_open_site'),
            onPressed: () async {
              final uri = Uri.tryParse(info.downloadUrl);
              if (uri != null && await canLaunchUrl(uri)) {
                await launchUrl(uri, mode: LaunchMode.externalApplication);
              }
            },
            child: Text(L10n.t('update_open_site')),
          ),
      ],
    );
  }
}

/// The engine-update window: llama.cpp / whisper.cpp / the Subscriptions
/// connector have newer builds. Applying one stops the engine and starts it
/// again, so it asks first.
class ComponentUpdateDialog extends ConsumerWidget {
  const ComponentUpdateDialog({super.key, required this.components});
  final List<ComponentUpdate> components;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    final flow = ref.watch(updateFlowProvider);
    final running = flow.running;
    final current = flow.progress.component;

    return PopScope(
      canPop: !running,
      child: Dialog(
        insetPadding: const EdgeInsets.symmetric(horizontal: 24, vertical: 40),
        backgroundColor: theme.bgPanel,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusLg)),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 480),
          child: Padding(
            padding: const EdgeInsets.all(22),
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(children: [
                    Icon(Icons.memory_rounded, color: MemoTheme.accent, size: 22),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Text(L10n.t('update_components_title'),
                          style: TextStyle(color: theme.textMain, fontSize: 17, fontWeight: FontWeight.w600)),
                    ),
                  ]),
                  const SizedBox(height: 12),
                  Text(L10n.t('update_components_body'),
                      style: TextStyle(color: theme.textSecondary, fontSize: 13.5, height: 1.4)),
                  const SizedBox(height: 14),
                  for (final c in components)
                    Padding(
                      padding: const EdgeInsets.symmetric(vertical: 5),
                      child: Row(children: [
                        Expanded(
                          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                            Text(L10n.t('update_component_${c.name}'),
                                style: TextStyle(color: theme.textMain, fontSize: 13.5, fontWeight: FontWeight.w500)),
                            Text(
                              '${c.installed.isEmpty ? '?' : c.installed} → ${c.latest}'
                              '${c.sizeBytes > 0 ? '  ·  ${formatUpdateBytes(c.sizeBytes)}' : ''}',
                              key: Key('update_component_${c.name}_versions'),
                              style: TextStyle(color: theme.textMuted, fontSize: 12),
                            ),
                          ]),
                        ),
                        if (running && current == c.name)
                          const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))
                        else
                          TextButton(
                            key: Key('update_component_${c.name}_apply'),
                            onPressed: running ? null : () => ref.read(updateFlowProvider.notifier).start('component', component: c.name),
                            child: Text(L10n.t('update_now')),
                          ),
                      ]),
                    ),
                  if (running) ...[
                    const SizedBox(height: 8),
                    _ProgressView(flow: flow, windowsInstaller: false),
                  ],
                  if (flow.error.isNotEmpty) ...[
                    const SizedBox(height: 10),
                    Text(L10n.t('update_failed', {'err': flow.error}),
                        key: const Key('update_error'), style: const TextStyle(color: MemoTheme.red, fontSize: 12.5)),
                  ],
                  const SizedBox(height: 14),
                  Align(
                    alignment: Alignment.centerRight,
                    child: TextButton(
                      key: const Key('update_later'),
                      onPressed: running ? null : () => Navigator.of(context).pop(),
                      child: Text(L10n.t(flow.progress.isDone ? 'update_close' : 'update_later')),
                    ),
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
