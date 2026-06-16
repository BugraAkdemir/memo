import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/provider_config.dart';
import '../providers/chat_provider.dart';
import '../providers/models_provider.dart';
import 'mood_gauge.dart';
import '../providers/orchestra_provider.dart';
import '../providers/provider_provider.dart';

/// Signature element: a calm, always-present strip at the foot of the content
/// area showing Memo's live "engine" — which chat model and memory (embedding)
/// model are running — with one-tap stop. Surfaces state that used to be buried
/// inside the model screen, so you always know what's powering your chat.
class EngineStrip extends ConsumerWidget {
  /// Jump to the Models tab (used when nothing is running yet).
  final VoidCallback onOpenModels;
  const EngineStrip({super.key, required this.onOpenModels});

  String _fileName(String path) {
    final name = path.split('/').last.split('\\').last;
    return name.isEmpty ? path : name;
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final status = ref.watch(modelStatusProvider).valueOrNull;
    final emb = ref.watch(embeddingStatusProvider).valueOrNull;

    final chatRunning = status?.running ?? false;
    final embRunning = emb?.running ?? false;
    final activeProviderType =
        ref.watch(activeProviderTypeProvider).valueOrNull ?? '';
    final isApiProvider =
        activeProviderType.isNotEmpty && activeProviderType != 'local';

    return Container(
      height: 40,
      padding: const EdgeInsets.symmetric(horizontal: 20),
      decoration: BoxDecoration(
        color: c.bgPanel,
        border: Border(top: BorderSide(color: c.borderSoft)),
      ),
      child: Row(
        children: [
          if (chatRunning) ...[
            _LiveIndicator(
              icon: Icons.memory,
              label: _fileName(status!.modelPath),
              onStop: () async {
                await ref.read(apiClientProvider).stopModel();
                ref.invalidate(modelStatusProvider);
              },
            ),
            if (embRunning) ...[
              _divider(c.borderSoft),
              _LiveIndicator(
                icon: Icons.hub_outlined,
                label: '${L10n.t('engine_memory')}: ${_fileName(emb!.modelPath)}',
                onStop: () async {
                  await ref.read(apiClientProvider).stopEmbeddingModel();
                  ref.invalidate(embeddingStatusProvider);
                },
              ),
            ],
          ] else if (isApiProvider)
            _ApiProviderIndicator(providerType: activeProviderType)
          else
            _OfflineHint(onOpenModels: onOpenModels),
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
          const MoodGauge(),
          const SizedBox(width: 12),
          const Spacer(),
          MouseRegion(
            cursor: SystemMouseCursors.click,
            child: GestureDetector(
              onTap: onOpenModels,
              child: Row(
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
