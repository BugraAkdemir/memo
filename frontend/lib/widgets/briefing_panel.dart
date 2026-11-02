import 'package:flutter/material.dart';
import '../core/l10n.dart';

/// Morning briefing and pre-meeting note: two opt-in switches with their own
/// time choices, and the newest briefing cards. Cards are built locally with no
/// model call; headings arrive as codes and are translated here.
class BriefingPanel extends StatelessWidget {
  static const _leads = [5, 10, 15, 30, 60];

  final Map<String, dynamic> settings;
  final String timezone;
  final List<Map<String, dynamic>> briefings;
  final bool busy;
  final ValueChanged<Map<String, dynamic>> onSettings;
  final ValueChanged<String> onRead;
  const BriefingPanel({
    super.key,
    required this.settings,
    this.timezone = '',
    required this.briefings,
    required this.busy,
    required this.onSettings,
    required this.onRead,
  });

  String get _time {
    final t = settings['morning_time'];
    return t is String && RegExp(r'^\d\d:\d\d$').hasMatch(t) ? t : '09:00';
  }

  int get _lead {
    final m = settings['pre_meeting_minutes'];
    return m is int && m > 0 ? m : 15;
  }

  void _set(String key, Object value) => onSettings({
    'morning': settings['morning'] == true,
    'morning_time': _time,
    'pre_meeting': settings['pre_meeting'] == true,
    'pre_meeting_minutes': _lead,
    key: value,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    final times = {
      for (var h = 5; h <= 12; h++) ...[
        '${h.toString().padLeft(2, '0')}:00',
        '${h.toString().padLeft(2, '0')}:30',
      ],
      _time,
    }.toList()..sort();
    final leads = {..._leads, _lead}.toList()..sort();
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
                Icon(Icons.wb_twilight_outlined, color: colors.primary),
                const SizedBox(width: 10),
                Text(
                  L10n.t('briefing_title'),
                  style: theme.textTheme.titleMedium,
                ),
              ],
            ),
            const SizedBox(height: 6),
            Text(L10n.t('briefing_help'), style: theme.textTheme.bodySmall),
            SwitchListTile(
              key: const ValueKey('briefing-morning'),
              contentPadding: EdgeInsets.zero,
              title: Text(L10n.t('briefing_morning_toggle')),
              value: settings['morning'] == true,
              onChanged: busy ? null : (v) => _set('morning', v),
            ),
            DropdownButtonFormField<String>(
              key: const ValueKey('briefing-time'),
              initialValue: _time,
              isExpanded: true,
              decoration: InputDecoration(
                labelText: L10n.t('briefing_morning_time'),
              ),
              items: [
                for (final t in times)
                  DropdownMenuItem(value: t, child: Text(t)),
              ],
              onChanged: busy
                  ? null
                  : (t) {
                      if (t != null) _set('morning_time', t);
                    },
            ),
            if (timezone.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 4, bottom: 4),
                child: Text(
                  L10n.t('briefing_timezone', {'tz': timezone}),
                  style: theme.textTheme.bodySmall,
                ),
              ),
            SwitchListTile(
              key: const ValueKey('briefing-meeting'),
              contentPadding: EdgeInsets.zero,
              title: Text(L10n.t('briefing_meeting_toggle')),
              value: settings['pre_meeting'] == true,
              onChanged: busy ? null : (v) => _set('pre_meeting', v),
            ),
            DropdownButtonFormField<int>(
              key: const ValueKey('briefing-minutes'),
              initialValue: _lead,
              isExpanded: true,
              decoration: InputDecoration(
                labelText: L10n.t('briefing_meeting_minutes'),
              ),
              items: [
                for (final m in leads)
                  DropdownMenuItem(
                    value: m,
                    child: Text(L10n.t('briefing_minutes_value', {'m': '$m'})),
                  ),
              ],
              onChanged: busy
                  ? null
                  : (m) {
                      if (m != null) _set('pre_meeting_minutes', m);
                    },
            ),
            const SizedBox(height: 16),
            if (briefings.isEmpty)
              Text(L10n.t('briefing_empty'), style: theme.textTheme.bodySmall)
            else
              for (final b in briefings) _card(context, b),
          ],
        ),
      ),
    );
  }

  Widget _card(BuildContext context, Map<String, dynamic> b) {
    final theme = Theme.of(context);
    final unread = b['read'] != true;
    final title = b['kind'] == 'meeting'
        ? L10n.t('briefing_meeting_title', {'title': '${b['title'] ?? ''}'})
        : L10n.t('briefing_morning_title', {'date': '${b['title'] ?? ''}'});
    final sections = (b['sections'] as List? ?? const [])
        .whereType<Map>()
        .map((s) => Map<String, dynamic>.from(s))
        .toList();
    return ExpansionTile(
      key: PageStorageKey('briefing-${b['key']}'),
      tilePadding: EdgeInsets.zero,
      childrenPadding: const EdgeInsets.only(left: 4, bottom: 8),
      expandedCrossAxisAlignment: CrossAxisAlignment.start,
      leading: Icon(
        unread ? Icons.circle : Icons.circle_outlined,
        size: 10,
        color: unread ? theme.colorScheme.primary : null,
      ),
      title: Text(
        title,
        style: unread ? theme.textTheme.titleSmall : theme.textTheme.bodyMedium,
      ),
      onExpansionChanged: (open) {
        if (open && unread) onRead('${b['key']}');
      },
      children: [
        for (final s in sections) ...[
          Text(
            L10n.t('briefing_h_${s['heading']}'),
            style: theme.textTheme.labelLarge,
          ),
          const SizedBox(height: 4),
          if (s['counts'] is Map && (s['counts'] as Map).isNotEmpty)
            Text(
              (s['counts'] as Map).entries
                  .map((e) => '${L10n.t('auto_state_${e.key}')}: ${e.value}')
                  .join(' · '),
            ),
          for (final item in (s['items'] as List? ?? const []))
            Padding(
              padding: const EdgeInsets.only(bottom: 2),
              child: Text('$item'),
            ),
          const SizedBox(height: 8),
        ],
      ],
    );
  }
}
