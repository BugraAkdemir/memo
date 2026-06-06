import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../core/theme.dart';
import '../../providers/agent_provider.dart';

class AgentModeToggle extends ConsumerWidget {
  const AgentModeToggle({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isEnabled = ref.watch(agentEnabledProvider);
    final colors = MemoTheme.of(context);

    return GestureDetector(
      onTap: () {
        ref.read(agentEnabledProvider.notifier).setEnabled(!isEnabled);
      },
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 200),
        decoration: BoxDecoration(
          color: isEnabled
              ? MemoTheme.accent.withValues(alpha: 0.12)
              : colors.bgPanel,
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          border: Border.all(
            color: isEnabled
                ? MemoTheme.accent.withValues(alpha: 0.4)
                : colors.borderSoft,
          ),
        ),
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              isEnabled ? Icons.psychology : Icons.smart_toy,
              size: 17,
              color: isEnabled ? MemoTheme.accent : colors.textDim,
            ),
            const SizedBox(width: 6),
            Text(
              'Agent',
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w600,
                color: isEnabled ? MemoTheme.accent : colors.textDim,
              ),
            ),
            const SizedBox(width: 8),
            AnimatedContainer(
              duration: const Duration(milliseconds: 200),
              width: 30,
              height: 18,
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(10),
                color: isEnabled ? MemoTheme.green : colors.textDim.withValues(alpha: 0.25),
              ),
              child: AnimatedAlign(
                duration: const Duration(milliseconds: 200),
                alignment: isEnabled ? Alignment.centerRight : Alignment.centerLeft,
                child: Container(
                  width: 14,
                  height: 14,
                  margin: const EdgeInsets.all(2),
                  decoration: BoxDecoration(
                    color: colors.textInverse,
                    shape: BoxShape.circle,
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withValues(alpha: 0.15),
                        blurRadius: 2,
                        offset: const Offset(0, 1),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
