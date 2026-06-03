import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/models_provider.dart';

/// Small badge that displays GPU status (Detected / CPU Only)
class GPUBadge extends ConsumerWidget {
   GPUBadge({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final gpuAsync = ref.watch(gpuInfoProvider);

    return gpuAsync.when(
      loading: () =>  _Badge(
        icon: Icons.memory,
        text: '...',
        color: MemoTheme.of(context).textDim,
      ),
      error: (_, __) => _Badge(
        icon: Icons.memory,
        text: L10n.t('error'),
        color: MemoTheme.of(context).textDim,
      ),
      data: (gpu) {
        if (gpu.hasGpu) {
          return _Badge(
            icon: Icons.memory,
            text: '${L10n.t('gpu_detected')} (${gpu.name})',
            color: MemoTheme.accent,
          );
        } else {
          return _Badge(
            icon: Icons.memory,
            text: L10n.t('cpu_mode'),
            color: MemoTheme.of(context).textDim,
          );
        }
      },
    );
  }
}

class _Badge extends StatelessWidget {
  final IconData icon;
  final String text;
  final Color color;

   _Badge({
    required this.icon,
    required this.text,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding:  EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: color.withValues(alpha: 0.2)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 14, color: color),
           SizedBox(width: 6),
          Text(
            text,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: color,
            ),
          ),
        ],
      ),
    );
  }
}
