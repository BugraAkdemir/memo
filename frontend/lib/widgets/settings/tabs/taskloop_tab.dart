import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../providers/provider_provider.dart';
import '../../../core/friendly_error.dart';

class TaskLoopTab extends ConsumerWidget {
  const TaskLoopTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final providersAsync = ref.watch(providerListProvider);

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            L10n.t('taskloop_settings'),
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.w600,
              color: c.textMain,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            L10n.t('taskloop_description'),
            style: TextStyle(fontSize: 13, color: c.textSecondary),
          ),
          const SizedBox(height: 24),
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: c.bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(color: c.borderSoft),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(Icons.engineering, size: 20, color: c.textSecondary),
                    const SizedBox(width: 8),
                    Text(
                      L10n.t('taskloop_worker'),
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                        color: c.textMain,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 6),
                Text(
                  L10n.t('taskloop_worker_desc'),
                  style: TextStyle(fontSize: 12, color: c.textDim),
                ),
                const SizedBox(height: 12),
                providersAsync.when(
                  data: (providers) => _buildProviderInfo(c, providers),
                  loading: () =>
                      const SizedBox(height: 20, child: LinearProgressIndicator()),
                  error: (e, _) => Text(
                    '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
                    style: const TextStyle(fontSize: 12, color: MemoTheme.red),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: c.bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(color: c.borderSoft),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(Icons.policy, size: 20, color: c.textSecondary),
                    const SizedBox(width: 8),
                    Text(
                      L10n.t('taskloop_ceo'),
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                        color: c.textMain,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 6),
                Text(
                  L10n.t('taskloop_ceo_desc'),
                  style: TextStyle(fontSize: 12, color: c.textDim),
                ),
                const SizedBox(height: 12),
                providersAsync.when(
                  data: (providers) => _buildCEOInfo(c, providers),
                  loading: () =>
                      const SizedBox(height: 20, child: LinearProgressIndicator()),
                  error: (e, _) => Text(
                    '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
                    style: const TextStyle(fontSize: 12, color: MemoTheme.red),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: c.bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(color: c.borderSoft),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(Icons.info_outline, size: 20, color: c.textSecondary),
                    const SizedBox(width: 8),
                    Text(
                      L10n.t('taskloop_how_it_works'),
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                        color: c.textMain,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 8),
                Text(
                  L10n.t('taskloop_how_it_works_desc'),
                  style: TextStyle(fontSize: 12, color: c.textSecondary, height: 1.5),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildProviderInfo(ThemeColors c, List<dynamic> providers) {
    final enabled = providers.where((p) => p.enabled == true).toList();
    if (enabled.isEmpty) {
      return Text(
        L10n.t('taskloop_no_providers'),
        style: TextStyle(fontSize: 12, color: c.textDim, fontStyle: FontStyle.italic),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          '${L10n.t('taskloop_worker_uses')}: ${enabled.length} ${L10n.t('enabled').toLowerCase()} provider',
          style: TextStyle(fontSize: 12, color: c.textSecondary),
        ),
        const SizedBox(height: 8),
        Wrap(
          spacing: 8,
          runSpacing: 4,
          children: enabled.map((p) {
            final type = p.type as String? ?? '';
            final name = p.name as String? ?? '';
            return Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              decoration: BoxDecoration(
                color: c.bgElement,
                borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                border: Border.all(color: c.borderSoft),
              ),
              child: Text(
                '${providerIcon(type)} ${name.isNotEmpty ? name : type}',
                style: TextStyle(fontSize: 12, color: c.textMain),
              ),
            );
          }).toList(),
        ),
      ],
    );
  }

  Widget _buildCEOInfo(ThemeColors c, List<dynamic> providers) {
    return Text(
      L10n.t('taskloop_ceo_auto'),
      style: TextStyle(fontSize: 12, color: c.textSecondary, height: 1.5),
    );
  }
}
