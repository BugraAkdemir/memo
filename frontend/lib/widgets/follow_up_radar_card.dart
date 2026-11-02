import 'package:flutter/material.dart';
import '../core/l10n.dart';

/// Follow-up radar: turns on reminder drafts for questions that stay
/// unanswered, and lists what the owner is still waiting on. The radar only
/// prepares a draft for review; it never sends anything by itself.
class FollowUpRadarCard extends StatelessWidget {
  static const _choices = [24, 48, 72, 120, 168];

  final Map<String, dynamic> radar;
  final List<Map<String, dynamic>> waiting;
  final bool busy;
  final ValueChanged<Map<String, dynamic>> onChanged;
  const FollowUpRadarCard({
    super.key,
    required this.radar,
    required this.waiting,
    required this.busy,
    required this.onChanged,
  });

  bool get _enabled => radar['enabled'] == true;

  int get _hours {
    final h = radar['follow_up_hours'];
    return h is int && h > 0 ? h : 48;
  }

  static String _when(String? iso) {
    final t = DateTime.tryParse(iso ?? '')?.toLocal();
    if (t == null) return '';
    String two(int n) => n.toString().padLeft(2, '0');
    return '${two(t.day)}.${two(t.month)} ${two(t.hour)}:${two(t.minute)}';
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    final hours = {..._choices, _hours}.toList()..sort();
    return Material(
      color: colors.surfaceContainerHighest.withValues(alpha: .4),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: BorderSide(color: colors.outlineVariant),
      ),
      clipBehavior: Clip.antiAlias,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.radar_outlined, color: colors.primary),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    L10n.t('radar_title'),
                    style: theme.textTheme.titleMedium,
                  ),
                ),
                Switch(
                  value: _enabled,
                  onChanged: busy
                      ? null
                      : (v) => onChanged({...radar, 'enabled': v}),
                ),
              ],
            ),
            const SizedBox(height: 6),
            Text(L10n.t('radar_help'), style: theme.textTheme.bodySmall),
            const SizedBox(height: 12),
            DropdownButtonFormField<int>(
              key: const ValueKey('radar-hours'),
              initialValue: _hours,
              isExpanded: true,
              decoration: InputDecoration(labelText: L10n.t('radar_hours')),
              items: [
                for (final h in hours)
                  DropdownMenuItem(
                    value: h,
                    child: Text(L10n.t('radar_hours_value', {'h': '$h'})),
                  ),
              ],
              onChanged: busy
                  ? null
                  : (h) {
                      if (h != null) {
                        onChanged({...radar, 'follow_up_hours': h});
                      }
                    },
            ),
            const SizedBox(height: 16),
            Text(
              L10n.t('radar_waiting_title', {'n': '${waiting.length}'}),
              style: theme.textTheme.labelLarge,
            ),
            const SizedBox(height: 8),
            if (waiting.isEmpty)
              Text(
                L10n.t('radar_waiting_empty'),
                style: theme.textTheme.bodySmall,
              )
            else
              for (final w in waiting)
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(
                        w['status'] == 'reminded'
                            ? Icons.mark_email_read_outlined
                            : Icons.hourglass_empty,
                        size: 18,
                        color: colors.onSurfaceVariant,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              [
                                '${w['address'] ?? ''}',
                                if ('${w['subject'] ?? ''}'.isNotEmpty)
                                  '${w['subject']}',
                              ].join(' · '),
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                            ),
                            Text(
                              L10n.t(
                                w['status'] == 'reminded'
                                    ? 'radar_status_reminded'
                                    : 'radar_status_open',
                              ),
                              style: theme.textTheme.bodySmall,
                            ),
                            if (_when(w['due_at'] as String?).isNotEmpty)
                              Text(
                                L10n.t('radar_due', {
                                  'when': _when(w['due_at'] as String?),
                                }),
                                style: theme.textTheme.bodySmall,
                              ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
          ],
        ),
      ),
    );
  }
}
