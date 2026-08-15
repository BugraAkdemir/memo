import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path/path.dart' as p;

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/local_model.dart';
import '../models/provider_config.dart';
import '../providers/chat_provider.dart';
import '../providers/models_provider.dart';
import '../providers/agent_provider.dart';
import 'mood_gauge.dart';
import '../providers/mood_provider.dart';
import '../providers/orchestra_provider.dart';
import '../providers/provider_provider.dart';
import '../providers/settings_provider.dart';

/// Signature element: a calm, always-present strip at the foot of the content
/// area showing Memo's live "engine" — which chat model and memory (embedding)
/// model are running — with one-tap stop. Surfaces state that used to be buried
/// inside the model screen, so you always know what's powering your chat.
class EngineStrip extends ConsumerWidget {
  /// Jump to the Models tab (used when nothing is running yet).
  final VoidCallback onOpenModels;
  const EngineStrip({super.key, required this.onOpenModels});

  String _fileName(String path) {
    final name = p.basename(path);
    return name.isEmpty ? path : name;
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final status = ref.watch(modelStatusProvider).valueOrNull;
    final emb = ref.watch(embeddingStatusProvider).valueOrNull;

    final chatRunning = status?.running ?? false;
    final embRunning = emb?.running ?? false;
    final memoryEnabled = ref.watch(memoryEnabledProvider).valueOrNull ?? false;
    final hasEmbeddingModel =
        (ref.watch(localModelsProvider).valueOrNull ?? []).any((m) => m.isEmbedding);
    final activeDownloads = (ref.watch(downloadProgressProvider).valueOrNull ?? [])
        .where((d) => d.active && (d.error == null || d.error!.isEmpty))
        .toList();
    final activeProviderType =
        ref.watch(activeProviderTypeProvider).valueOrNull ?? '';
    final isApiProvider =
        activeProviderType.isNotEmpty && activeProviderType != 'local';
    // The active chat's own CLI provider (Claude Code, ...) — takes
    // priority in the strip's first slot over the app-wide model/provider,
    // since it's what that chat actually sends to.
    final cliType = ref.watch(activeChatCLIProviderProvider).valueOrNull ?? '';
    final isCLIChat = cliType.isNotEmpty;

    // Whether the strip's first "slot" (chat model / API provider / offline
    // hint) rendered anything, and whether the second slot (embedding
    // model / memory warning) did — used to decide when a divider actually
    // has two neighbors to separate instead of leaving a stray line (or, as
    // reported, no gap at all) next to empty space.
    final firstSlotHasContent = isCLIChat || chatRunning || isApiProvider || !embRunning;
    final secondSlotHasContent = embRunning || memoryEnabled;

    return Container(
      height: 40,
      // Floating rounded glass pill in Glass Light; flush top-bordered bar in dark.
      margin: c.isGlass
          ? const EdgeInsets.fromLTRB(12, 0, 12, 8)
          : EdgeInsets.zero,
      padding: const EdgeInsets.symmetric(horizontal: 20),
      clipBehavior: c.isGlass ? Clip.antiAlias : Clip.none,
      decoration: c.isGlass
          ? BoxDecoration(
              color: c.bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
              border: Border.all(color: c.borderSoft),
              boxShadow: MemoTheme.shadowLg,
            )
          : BoxDecoration(
              color: c.bgPanel,
              border: Border(top: BorderSide(color: c.borderSoft)),
            ),
      child: Row(
        children: [
          // The indicator set grows with however many of these are active
          // at once (chat model, memory, downloads, auto-permission,
          // Orchestra, mood) plus their dividers, with no upper bound — fine
          // at desktop widths, but a plain Row here overflowed by ~300px on
          // a phone-width viewport (RenderFlex assertion, confirmed live at
          // 375px). Scroll horizontally instead of erroring/clipping;
          // Expanded (not Spacer, which needs bounded width and doesn't
          // work inside a scrollable) still pins "Open Models" to the right
          // edge on wide screens exactly as before.
          Expanded(
            child: SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: [
                  if (isCLIChat)
                    _CLIModeIndicator(cliType: cliType)
                  else if (chatRunning)
                    _LiveIndicator(
                      icon: Icons.memory,
                      label: _fileName(status!.modelPath),
                      onStop: () async {
                        await ref.read(apiClientProvider).stopModel();
                        ref.invalidate(modelStatusProvider);
                      },
                    )
                  else if (isApiProvider)
                    _ApiProviderIndicator(providerType: activeProviderType)
                  else if (!embRunning)
                    _OfflineHint(onOpenModels: onOpenModels),
                  if (embRunning) ...[
                    // The offline hint above only renders when !embRunning, so
                    // "chatRunning || isApiProvider" alone misses the case where it
                    // fired instead — same firstSlotHasContent check as below.
                    if (firstSlotHasContent) _divider(c.borderSoft),
                    _LiveIndicator(
                      icon: Icons.hub_outlined,
                      label: '${L10n.t('engine_memory')}: ${_fileName(emb!.modelPath)}',
                      onStop: () async {
                        await ref.read(apiClientProvider).stopEmbeddingModel();
                        ref.invalidate(embeddingStatusProvider);
                      },
                    ),
                  ] else if (memoryEnabled) ...[
                    // Memory is turned on in Settings but no embedding server is
                    // actually running — RAG silently does nothing until this is
                    // fixed, so surface it instead of failing invisibly.
                    //
                    // firstSlotHasContent (not "chatRunning || isApiProvider"): the
                    // offline hint renders whenever chat isn't running AND there's
                    // no active API provider AND embedding isn't running either —
                    // exactly the state a fully-offline install starts in. The old
                    // check missed that case, so the offline hint and this warning
                    // rendered stuck together with no gap between them.
                    if (firstSlotHasContent) _divider(c.borderSoft),
                    _MemoryWarningIndicator(hasModel: hasEmbeddingModel, onTap: onOpenModels),
                  ],
                  if (activeDownloads.isNotEmpty) ...[
                    if (firstSlotHasContent || secondSlotHasContent) _divider(c.borderSoft),
                    _DownloadIndicator(
                      downloads: activeDownloads,
                      onTap: onOpenModels,
                    ),
                  ],
                  // Auto-permission indicator
                  if (ref.watch(agentAutoPermissionProvider)) ...[
                    _divider(c.borderSoft),
                    Tooltip(
                      message: L10n.t('auto_permission_tooltip'),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Container(
                            width: 7, height: 7,
                            decoration: const BoxDecoration(color: MemoTheme.green, shape: BoxShape.circle),
                          ),
                          const SizedBox(width: 6),
                          const Icon(Icons.auto_awesome, size: 13, color: MemoTheme.green),
                          const SizedBox(width: 4),
                          Text(
                            L10n.t('auto_permission_short'),
                            style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: MemoTheme.green),
                          ),
                        ],
                      ),
                    ),
                  ],
                  // Orchestra mode indicator
                  if (ref.watch(orchestraConfigProvider).valueOrNull?.enabled == true) ...[
                    _divider(c.borderSoft),
                    Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Container(
                          width: 7, height: 7,
                          decoration: const BoxDecoration(color: MemoTheme.accent, shape: BoxShape.circle),
                        ),
                        const SizedBox(width: 8),
                        const Text('🎵', style: TextStyle(fontSize: 13)),
                        const SizedBox(width: 4),
                        Text(
                          'Orchestra',
                          style: TextStyle(fontSize: 12, fontWeight: FontWeight.w500, color: c.textMain),
                        ),
                      ],
                    ),
                  ],
                  if (ref.watch(moodEnabledProvider).valueOrNull == true) ...[
                    _divider(c.borderSoft),
                    const MoodGauge(),
                  ],
                ],
              ),
            ),
          ),
          const SizedBox(width: 12),
          MouseRegion(
            cursor: SystemMouseCursors.click,
            child: GestureDetector(
              onTap: onOpenModels,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    L10n.t('engine_open_models'),
                    style: TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w500,
                        color: c.textMuted),
                  ),
                  const SizedBox(width: 4),
                  Icon(Icons.chevron_right, size: 16, color: c.textMuted),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _divider(Color color) => Container(
        width: 1,
        height: 18,
        margin: const EdgeInsets.symmetric(horizontal: 14),
        color: color,
      );
}

class _LiveIndicator extends StatelessWidget {
  final IconData icon;
  final String label;
  final Future<void> Function() onStop;
  const _LiveIndicator({
    required this.icon,
    required this.label,
    required this.onStop,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          width: 7,
          height: 7,
          decoration: const BoxDecoration(
              color: MemoTheme.green, shape: BoxShape.circle),
        ),
        const SizedBox(width: 8),
        Icon(icon, size: 15, color: c.textMuted),
        const SizedBox(width: 6),
        ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 240),
          child: Text(
            label,
            style: TextStyle(
                fontSize: 12, fontWeight: FontWeight.w500, color: c.textMain),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
        const SizedBox(width: 8),
        Tooltip(
          message: L10n.t('stop_model'),
          child: MouseRegion(
            cursor: SystemMouseCursors.click,
            child: GestureDetector(
              onTap: onStop,
              child: Icon(Icons.stop_circle_outlined,
                  size: 16, color: c.textDim),
            ),
          ),
        ),
      ],
    );
  }
}

class _CLIModeIndicator extends StatelessWidget {
  final String cliType;
  const _CLIModeIndicator({required this.cliType});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final displayName = ProviderDefaults.displayNames[cliType] ?? cliType;

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          width: 7,
          height: 7,
          decoration: const BoxDecoration(
              color: MemoTheme.accent, shape: BoxShape.circle),
        ),
        const SizedBox(width: 8),
        Icon(Icons.terminal_rounded, size: 16, color: MemoTheme.accent),
        const SizedBox(width: 6),
        Text(
          L10n.t('engine_cli_mode', {'name': displayName}),
          style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w500,
              color: c.textMain),
        ),
      ],
    );
  }
}

class _ApiProviderIndicator extends StatelessWidget {
  final String providerType;
  const _ApiProviderIndicator({required this.providerType});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final displayName =
        ProviderDefaults.displayNames[providerType] ?? providerType;

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          width: 7,
          height: 7,
          decoration: const BoxDecoration(
              color: MemoTheme.green, shape: BoxShape.circle),
        ),
        const SizedBox(width: 8),
        providerLogoWidget(providerType, size: 16),
        const SizedBox(width: 6),
        Text(
          displayName,
          style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w500,
              color: c.textMain),
        ),
      ],
    );
  }
}

class _OfflineHint extends StatelessWidget {
  final VoidCallback onOpenModels;
  const _OfflineHint({required this.onOpenModels});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: onOpenModels,
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 7,
              height: 7,
              decoration: BoxDecoration(
                  color: c.textDim, shape: BoxShape.circle),
            ),
            const SizedBox(width: 8),
            Text(
              L10n.t('engine_no_model'),
              style: TextStyle(fontSize: 12, color: c.textDim),
            ),
            const SizedBox(width: 10),
            Text(
              '· ${L10n.t('engine_start_model')}',
              style: const TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w500,
                  color: MemoTheme.accent),
            ),
          ],
        ),
      ),
    );
  }
}

/// Memory is enabled in Settings but the embedding server isn't running —
/// previously this failed silently (a backend log line + an internal event
/// nothing in Flutter reads), so RAG just stopped working with zero visible
/// indication. Surfaces it here instead, with a call to action tailored to
/// the actual cause: no embedding model downloaded yet vs. one exists but
/// isn't running.
class _MemoryWarningIndicator extends StatelessWidget {
  final bool hasModel;
  final VoidCallback onTap;
  const _MemoryWarningIndicator({required this.hasModel, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: onTap,
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 7,
              height: 7,
              decoration: const BoxDecoration(
                  color: MemoTheme.warningOrange, shape: BoxShape.circle),
            ),
            const SizedBox(width: 8),
            Text(
              hasModel
                  ? L10n.t('engine_memory_stopped')
                  : L10n.t('engine_memory_missing'),
              style: const TextStyle(fontSize: 12, color: MemoTheme.warningOrange),
            ),
            const SizedBox(width: 10),
            Text(
              '· ${hasModel ? L10n.t('engine_memory_stopped_action') : L10n.t('engine_memory_missing_action')}',
              style: const TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w500,
                  color: MemoTheme.accent),
            ),
          ],
        ),
      ),
    );
  }
}

/// Ambient download status, always visible regardless of screen (this strip
/// lives at the app-shell level). One download shows its filename; several
/// at once collapse into a single "downloading models" label so the strip
/// stays compact — with the percentage as the average of all of them.
class _DownloadIndicator extends StatelessWidget {
  final List<DownloadProgress> downloads;
  final VoidCallback onTap;
  const _DownloadIndicator({required this.downloads, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final isMulti = downloads.length > 1;
    final avgPercent =
        downloads.map((d) => d.percent).reduce((a, b) => a + b) / downloads.length;
    final label = isMulti
        ? L10n.t('engine_downloading_models')
        : p.basename(downloads.first.filename);

    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: onTap,
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            SizedBox(
              width: 14,
              height: 14,
              child: CircularProgressIndicator(
                strokeWidth: 2,
                value: avgPercent > 0 ? avgPercent / 100 : null,
                color: MemoTheme.accent,
              ),
            ),
            const SizedBox(width: 8),
            Icon(Icons.download_rounded, size: 14, color: c.textMuted),
            const SizedBox(width: 6),
            ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 160),
              child: Text(
                label,
                style: TextStyle(
                    fontSize: 12, fontWeight: FontWeight.w500, color: c.textMain),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            const SizedBox(width: 6),
            Text(
              '${avgPercent.toStringAsFixed(0)}%',
              style: const TextStyle(
                  fontSize: 12, fontWeight: FontWeight.w600, color: MemoTheme.accent),
            ),
          ],
        ),
      ),
    );
  }
}
