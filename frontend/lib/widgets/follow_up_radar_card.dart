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
  final ValueChanged<String>? onResolve;
  const FollowUpRadarCard({
    super.key,
    required this.radar,
    required this.waiting,
    required this.busy,
    required this.onChanged,
    this.onResolve,
  });

  bool get _enabled => radar['enabled'] == true;
  bool get _track => radar['track_promises'] == true;

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
    final replies = [
      for (final w in waiting)
        if (w['kind'] != 'my_promise') w,
    ];
    final promises = [
      for (final w in waiting)
        if (w['kind'] == 'my_promise') w,
    ];
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
            SwitchListTile(
              key: const ValueKey('radar-track-promises'),
              contentPadding: EdgeInsets.zero,
              title: Text(L10n.t('radar_promises_toggle')),
              subtitle: Text(L10n.t('radar_promises_help')),
              value: _track,
              onChanged: busy
                  ? null
                  : (v) => onChanged({...radar, 'track_promises': v}),
            ),
            const SizedBox(height: 16),
            Text(
              L10n.t('radar_waiting_title', {'n': '${replies.length}'}),
              style: theme.textTheme.labelLarge,
            ),
            const SizedBox(height: 8),
            if (replies.isEmpty)
              Text(
                L10n.t('radar_waiting_empty'),
                style: theme.textTheme.bodySmall,
              )
            else
              for (final w in replies)
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
            if (_track || promises.isNotEmpty) ...[
              const SizedBox(height: 16),
              Text(
                L10n.t('radar_promises_title', {'n': '${promises.length}'}),
                style: theme.textTheme.labelLarge,
              ),
              const SizedBox(height: 8),
              if (promises.isEmpty)
                Text(
                  L10n.t('radar_promises_empty'),
                  style: theme.textTheme.bodySmall,
                )
              else
                for (final w in promises)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Icon(
                          w['status'] == 'reminded'
                              ? Icons.notifications_active_outlined
                              : Icons.handshake_outlined,
                          size: 18,
                          color: w['status'] == 'reminded'
                              ? colors.error
                              : colors.onSurfaceVariant,
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                '${w['summary'] ?? ''} · ${w['address'] ?? ''}',
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                              ),
                              Text(
                                L10n.t(
                                  w['status'] == 'reminded'
                                      ? 'radar_promise_reminded'
                                      : 'radar_promise_open',
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
                        TextButton(
                          onPressed: busy || onResolve == null
                              ? null
                              : () => onResolve!('${w['id']}'),
                          child: Text(L10n.t('radar_promise_done')),
                        ),
                      ],
                    ),
                  ),
            ],
          ],
        ),
      ),
    );
  }
}
