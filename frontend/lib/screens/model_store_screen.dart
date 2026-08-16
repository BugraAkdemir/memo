import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/gpu_info.dart';
import '../providers/models_provider.dart';

import 'model_store/discover_tab.dart';
import 'model_store/my_models_tab.dart';

// ─── Root screen ─────────────────────────────────────────────────

class ModelStoreScreen extends ConsumerStatefulWidget {
  const ModelStoreScreen({super.key});

  @override
  ConsumerState<ModelStoreScreen> createState() => _ModelStoreScreenState();
}

class _ModelStoreScreenState extends ConsumerState<ModelStoreScreen> {
  int _tab = 0;

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Container(
      color: c.bgApp,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _Header(tab: _tab, onTab: (t) => setState(() => _tab = t)),
          const DownloadBanner(),
          Expanded(
            child: IndexedStack(
              index: _tab,
              children: [DiscoverTab(isActive: _tab == 0), const MyModelsTab()],
            ),
          ),
        ],
      ),
    );
  }
}

// ─── Header ──────────────────────────────────────────────────────

class _Header extends ConsumerWidget {
  final int tab;
  final ValueChanged<int> onTab;
  const _Header({required this.tab, required this.onTab});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(24, 16, 24, 0),
      decoration: BoxDecoration(
        border: Border(bottom: BorderSide(color: c.borderSoft)),
      ),
      child: Row(
        children: [
          // Title + tabs scroll rather than overflow: their combined
          // natural width (localized labels included) exceeds a phone-width
          // pane, which threw a 41px RenderFlex overflow at 375px. Expanded
          // still pushes the hardware chip to the right edge exactly as the
          // previous Spacer did.
          Expanded(
            child: SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    L10n.t('nav_models'),
                    style: Theme.of(context).textTheme.titleLarge?.copyWith(
                          fontWeight: FontWeight.w700,
                          color: c.textMain,
                        ),
                  ),
                  const SizedBox(width: 24),
                  _TabItem(
                    label: L10n.t('tab_discover'),
                    active: tab == 0,
                    onTap: () => onTab(0),
                  ),
                  const SizedBox(width: 4),
                  _TabItem(
                    label: L10n.t('tab_my_models'),
                    active: tab == 1,
                    onTap: () => onTab(1),
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(width: 12),
          // Bounded so the chip's own Flexible text can ellipsize: as a
          // non-flex Row child it would otherwise be laid out against
          // unbounded width and never shorten.
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 200),
            child: const _HardwareChip(),
          ),
        ],
      ),
    );
  }
}

class _TabItem extends StatelessWidget {
  final String label;
  final bool active;
  final VoidCallback onTap;
  const _TabItem({required this.label, required this.active, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          decoration: BoxDecoration(
            border: Border(
              bottom: BorderSide(
                color: active ? MemoTheme.accent : Colors.transparent,
                width: 2,
              ),
            ),
          ),
          child: Text(
            label,
            style: TextStyle(
              fontSize: 13,
              fontWeight: active ? FontWeight.w600 : FontWeight.w500,
              color: active ? MemoTheme.accent : c.textMuted,
            ),
          ),
        ),
      ),
    );
  }
}

class _HardwareChip extends ConsumerWidget {
  const _HardwareChip();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final gpu = ref.watch(gpuInfoProvider).valueOrNull ?? const GPUInfo();
    final parts = <String>[];
    if (gpu.ramTotalMb > 0) parts.add('${gpu.ramFormatted} RAM');
    if (gpu.hasGpu) {
      final name = _shortGpuName(gpu.name);
      parts.add(gpu.vramMb > 0 ? '$name ${gpu.vramFormatted}' : name);
    } else {
      parts.add('CPU');
    }
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: c.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: c.borderSoft),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(gpu.hasGpu ? Icons.memory : Icons.developer_board,
              size: 13, color: MemoTheme.accent),
          const SizedBox(width: 6),
          Flexible(
            child: Text(
              parts.join(' · '),
              maxLines: 1,
              softWrap: false,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(fontSize: 11, color: c.textMuted),
            ),
          ),
        ],
      ),
    );
  }
}

/// Trims verbose lspci-style GPU descriptions to a readable short name.
/// e.g. "VGA compatible controller: Advanced Micro Devices, Inc. [AMD/ATI]
/// Cezanne [Radeon Vega Series / Radeon Vega Mobile Gfx]" → "Radeon Vega
/// Series / Radeon Vega Mobile Gfx". Clean names (NVIDIA/Windows) pass through.
String _shortGpuName(String name) {
  final bracket = RegExp(r'\[([^\[\]]+)\]\s*$').firstMatch(name);
  if (bracket != null) return bracket.group(1)!.trim();
  return name.replaceFirst(RegExp(r'^.*controller:\s*'), '').trim();
}


