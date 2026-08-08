import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/theme.dart';
import '../core/l10n.dart';
import '../providers/chat_provider.dart';
import 'app_shell.dart';
import '../core/friendly_error.dart';

/// A calendar event as returned by /api/calendar/events.
class _Event {
  final String id;
  final String title;
  final DateTime startTime;
  final String description;
  final String source;
  final String contactName;
  // True when the backend sent a start_time that failed to parse (startTime
  // falls back to DateTime.now() in that case, same as when it's missing
  // outright — but a parse *failure* means the event's real time is unknown,
  // not just unset, so the UI flags it instead of quietly showing "now" as
  // if that were the actual scheduled time).
  final bool hasInvalidDate;

  _Event({
    required this.id,
    required this.title,
    required this.startTime,
    required this.description,
    required this.source,
    required this.contactName,
    this.hasInvalidDate = false,
  });

  factory _Event.fromJson(Map<String, dynamic> j) {
    final rawStart = j['start_time'] as String?;
    return _Event(
      id: j['id'] as String? ?? '',
      title: j['title'] as String? ?? '',
      startTime: _parseDateTime(rawStart),
      description: j['description'] as String? ?? '',
      source: j['source'] as String? ?? 'manual',
      contactName: j['contact_name'] as String? ?? '',
      hasInvalidDate: rawStart != null && rawStart.isNotEmpty && DateTime.tryParse(rawStart) == null,
    );
  }

  static DateTime _parseDateTime(String? value) {
    if (value == null || value.isEmpty) return DateTime.now();
    try {
      return DateTime.parse(value).toLocal();
    } catch (_) {
      return DateTime.now();
    }
  }
}

const _monthKeys = [
  'month_january', 'month_february', 'month_march', 'month_april',
  'month_may', 'month_june', 'month_july', 'month_august',
  'month_september', 'month_october', 'month_november', 'month_december',
];
const _dayKeys = ['day_short_mon', 'day_short_tue', 'day_short_wed', 'day_short_thu', 'day_short_fri', 'day_short_sat', 'day_short_sun'];

String _two(int n) => n.toString().padLeft(2, '0');

class CalendarScreen extends ConsumerStatefulWidget {
  const CalendarScreen({super.key});

  @override
  ConsumerState<CalendarScreen> createState() => _CalendarScreenState();
}

class _CalendarScreenState extends ConsumerState<CalendarScreen> {
  DateTime _focused = DateTime.now();
  DateTime _selected = DateTime.now();
  List<_Event> _events = [];
  bool _loading = true;
  String? _error;
  Timer? _refreshTimer;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _load();
      // Start timer only if calendar tab is already active, then keep it
      // in sync with tab switches via ref.listen below.
      if (ref.read(activeTabProvider) == 4) _startRefreshTimer();
    });
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    super.dispose();
  }

  void _startRefreshTimer() {
    _refreshTimer?.cancel();
    _refreshTimer =
        Timer.periodic(const Duration(seconds: 20), (_) => _load(silent: true));
  }

  Future<void> _load({bool silent = false}) async {
    if (!silent) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final api = ref.read(apiClientProvider);
      final from = DateTime(_focused.year, _focused.month - 1, 1);
      final to = DateTime(_focused.year, _focused.month + 2, 0);
      final raw = await api.getCalendarEvents(from: from, to: to);
      if (!mounted) return;
      setState(() {
        _events = raw.map(_Event.fromJson).toList();
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = FriendlyError.describeGeneric(e);
        _loading = false;
      });
    }
  }

  List<_Event> _eventsForDay(DateTime day) {
    final list = _events
        .where((e) =>
            e.startTime.year == day.year &&
            e.startTime.month == day.month &&
            e.startTime.day == day.day)
        .toList()
      ..sort((a, b) => a.startTime.compareTo(b.startTime));
    return list;
  }

  bool _hasEvents(DateTime day) => _eventsForDay(day).isNotEmpty;

  Future<void> _delete(String id) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('delete')),
        content: Text(L10n.t('calendar_delete_confirm')),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: Text(L10n.t('cancel'))),
          TextButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: Text(L10n.t('delete'),
                  style: const TextStyle(color: MemoTheme.red))),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(apiClientProvider).deleteCalendarEvent(id);
      await _load();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(L10n.t('calendar_delete_error', {'e': e.toString()}))));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.listen<int>(activeTabProvider, (prev, next) {
      if (next == 4) {
        _startRefreshTimer();
      } else {
        _refreshTimer?.cancel();
        _refreshTimer = null;
      }
    });
    final c = MemoTheme.of(context);

    return Scaffold(
      backgroundColor: c.bgApp,
      body: Column(
        children: [
          _header(c),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _error != null
                    ? Center(child: Text(L10n.t('calendar_load_error', {'e': _error!}),
                        style: TextStyle(color: c.textDim)))
                    : _events.isEmpty
                        ? _buildEmptyState(c)
                        : Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          // Calendar grid (left)
                          Expanded(
                            flex: 3,
                            child: SingleChildScrollView(
                              padding: const EdgeInsets.all(20),
                              child: _grid(c),
                            ),
                          ),
                          VerticalDivider(width: 1, color: c.borderSoft),
                          // Day events (right)
                          Expanded(
                            flex: 2,
                            child: _dayPanel(c),
                          ),
                        ],
                      ),
          ),
        ],
      ),
    );
  }

  Widget _header(ThemeColors c) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
      decoration: BoxDecoration(
        color: c.bgPanel,
        border: Border(bottom: BorderSide(color: c.borderSoft)),
      ),
      child: Row(
        children: [
          Icon(Icons.calendar_month, color: MemoTheme.accent, size: 22),
          const SizedBox(width: 10),
          Text(L10n.t('calendar_title'),
              style: TextStyle(
                  fontSize: 17, fontWeight: FontWeight.w600, color: c.textMain)),
          const Spacer(),
          IconButton(
            tooltip: L10n.t('calendar_prev_month'),
            icon: Icon(Icons.chevron_left, color: c.textDim),
            onPressed: () {
              setState(() {
                _focused = DateTime(_focused.year, _focused.month - 1);
                _selected = _focused;
              });
              _load();
            },
          ),
          Text('${L10n.t(_monthKeys[_focused.month - 1])} ${_focused.year}',
              style: TextStyle(
                  fontSize: 15, fontWeight: FontWeight.w500, color: c.textMain)),
          IconButton(
            tooltip: L10n.t('calendar_next_month'),
            icon: Icon(Icons.chevron_right, color: c.textDim),
            onPressed: () {
              setState(() {
                _focused = DateTime(_focused.year, _focused.month + 1);
                _selected = _focused;
              });
              _load();
            },
          ),
          IconButton(
            tooltip: L10n.t('calendar_refresh'),
            icon: Icon(Icons.refresh, color: c.textDim),
            onPressed: () => _load(),
          ),
          const SizedBox(width: 8),
          FilledButton.icon(
            style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
            icon: const Icon(Icons.add, size: 18),
            label: Text(L10n.t('calendar_add_event')),
            onPressed: () => _showAddDialog(c),
          ),
        ],
      ),
    );
  }

  Widget _buildEmptyState(ThemeColors c) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(40),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 64,
              height: 64,
              decoration: BoxDecoration(
                color: MemoTheme.accent.withValues(alpha: 0.12),
                borderRadius: BorderRadius.circular(16),
              ),
              child: const Icon(Icons.calendar_month, size: 32, color: MemoTheme.accent),
            ),
            const SizedBox(height: 20),
            Text(
              L10n.t('calendar_empty_title'),
              style: TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.w700,
                color: c.textMain,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              L10n.t('calendar_empty_desc'),
              style: TextStyle(fontSize: 13, color: c.textDim),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 24),
            FilledButton.icon(
              style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
              onPressed: () => _showAddDialog(c),
              icon: const Icon(Icons.add, size: 18),
              label: Text(L10n.t('calendar_add_event_btn')),
            ),
          ],
        ),
      ),
    );
  }

  Widget _grid(ThemeColors c) {
    final firstDay = DateTime(_focused.year, _focused.month, 1);
    final daysInMonth = DateTime(_focused.year, _focused.month + 1, 0).day;
    final startOffset = (firstDay.weekday - 1) % 7; // Monday-based

    return Column(
      children: [
        Row(
          children: _dayKeys
              .map((d) => Expanded(
                    child: Center(
                      child: Text(L10n.t(d),
                          style: TextStyle(
                              color: c.textDim,
                              fontSize: 12,
                              fontWeight: FontWeight.w500)),
                    ),
                  ))
              .toList(),
        ),
        const SizedBox(height: 8),
        GridView.builder(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: 7,
            childAspectRatio: 1.1,
          ),
          itemCount: startOffset + daysInMonth,
          itemBuilder: (_, idx) {
            if (idx < startOffset) return const SizedBox.shrink();
            final day = DateTime(_focused.year, _focused.month, idx - startOffset + 1);
            final isSelected = day.year == _selected.year &&
                day.month == _selected.month &&
                day.day == _selected.day;
            final now = DateTime.now();
            final isToday =
                day.year == now.year && day.month == now.month && day.day == now.day;

            return GestureDetector(
              onTap: () => setState(() => _selected = day),
              child: Container(
                margin: const EdgeInsets.all(3),
                decoration: BoxDecoration(
                  color: isSelected
                      ? MemoTheme.accent
                      : isToday
                          ? MemoTheme.accentMuted
                          : Colors.transparent,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  border: Border.all(
                      color: isSelected ? MemoTheme.accent : c.borderSoft),
                ),
                child: Stack(
                  alignment: Alignment.center,
                  children: [
                    Text('${day.day}',
                        style: TextStyle(
                          color: isSelected ? Colors.white : c.textMain,
                          fontWeight: isToday ? FontWeight.bold : FontWeight.normal,
                          fontSize: 13,
                        )),
                    if (_hasEvents(day) && !isSelected)
                      Positioned(
                        bottom: 6,
                        child: Container(
                          width: 5,
                          height: 5,
                          decoration: BoxDecoration(
                            color: MemoTheme.accent,
                            shape: BoxShape.circle,
                          ),
                        ),
                      ),
                  ],
                ),
              ),
            );
          },
        ),
      ],
    );
  }

  Widget _dayPanel(ThemeColors c) {
    final events = _eventsForDay(_selected);
    final label = '${_selected.day} ${L10n.t(_monthKeys[_selected.month - 1])}';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 18, 20, 8),
          child: Text(label,
              style: TextStyle(
                  fontSize: 15, fontWeight: FontWeight.w600, color: c.textMain)),
        ),
        Expanded(
          child: events.isEmpty
              ? Center(
                  child: Text(L10n.t('calendar_no_events_day'),
                      style: TextStyle(color: c.textDim, fontSize: 13)))
              : ListView.builder(
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  itemCount: events.length,
                  itemBuilder: (_, i) => _eventTile(c, events[i]),
                ),
        ),
      ],
    );
  }

  Widget _eventTile(ThemeColors c, _Event e) {
    final time = '${_two(e.startTime.hour)}:${_two(e.startTime.minute)}';
    final icon = switch (e.source) {
      'whatsapp' => Icons.message_outlined,
      'chat' => Icons.smart_toy_outlined,
      _ => Icons.event_outlined,
    };

    return Container(
      margin: const EdgeInsets.symmetric(vertical: 5),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: c.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: c.borderSoft),
      ),
      child: Row(
        children: [
          Icon(icon, size: 18, color: MemoTheme.accent),
          const SizedBox(width: 10),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(time,
                  style: TextStyle(
                      color: c.textDim, fontSize: 12, fontWeight: FontWeight.w600)),
            ],
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Flexible(
                      child: Text(e.title,
                          style: TextStyle(
                              color: c.textMain, fontWeight: FontWeight.w500, fontSize: 14)),
                    ),
                    if (e.hasInvalidDate) ...[
                      const SizedBox(width: 6),
                      Tooltip(
                        message: L10n.t('received_an_invalid_date_from_the_server_the_time_'),
                        child: Icon(Icons.warning_amber_rounded,
                            size: 14, color: MemoTheme.warmBrown),
                      ),
                    ],
                  ],
                ),
                if (e.contactName.isNotEmpty)
                  Text(e.contactName,
                      style: TextStyle(color: c.textDim, fontSize: 12)),
              ],
            ),
          ),
          IconButton(
            tooltip: L10n.t('calendar_delete_event'),
            icon: Icon(Icons.delete_outline, size: 18, color: c.textDim),
            onPressed: () => _delete(e.id),
          ),
        ],
      ),
    );
  }

  void _showAddDialog(ThemeColors c) {
    final titleCtrl = TextEditingController();
    final descCtrl = TextEditingController();
    DateTime picked = DateTime(
        _selected.year, _selected.month, _selected.day,
        DateTime.now().hour + 1, 0);

    showDialog(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setS) => AlertDialog(
          backgroundColor: c.bgPanel,
          title: Text(L10n.t('calendar_new_event_title'), style: TextStyle(color: c.textMain)),
          content: SizedBox(
            width: 360,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                  controller: titleCtrl,
                  style: TextStyle(color: c.textMain),
                  decoration: InputDecoration(labelText: L10n.t('calendar_field_title')),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: descCtrl,
                  style: TextStyle(color: c.textMain),
                  decoration: InputDecoration(labelText: L10n.t('calendar_field_desc_optional')),
                ),
                const SizedBox(height: 14),
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        '${picked.day} ${L10n.t(_monthKeys[picked.month - 1])} ${picked.year}  ${_two(picked.hour)}:${_two(picked.minute)}',
                        style: TextStyle(color: c.textMain, fontSize: 13),
                      ),
                    ),
                    TextButton(
                      child: Text(L10n.t('calendar_pick_datetime')),
                      onPressed: () async {
                        final d = await showDatePicker(
                          context: ctx,
                          initialDate: picked,
                          firstDate: DateTime.now().subtract(const Duration(days: 1)),
                          lastDate: DateTime.now().add(const Duration(days: 365)),
                        );
                        if (d == null) return;
                        final t = await showTimePicker(
                          context: ctx,
                          initialTime: TimeOfDay.fromDateTime(picked),
                        );
                        if (t == null) return;
                        setS(() => picked =
                            DateTime(d.year, d.month, d.day, t.hour, t.minute));
                      },
                    ),
                  ],
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
                onPressed: () => Navigator.pop(ctx),
                child: Text(L10n.t('cancel'))),
            FilledButton(
              style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
              onPressed: () async {
                if (titleCtrl.text.trim().isEmpty) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(L10n.t('please_enter_title'))),
                  );
                  return;
                }
                Navigator.pop(ctx);
                try {
                  await ref.read(apiClientProvider).addCalendarEvent(
                      titleCtrl.text.trim(), picked, descCtrl.text.trim());
                  await _load();
                } catch (e) {
                  if (mounted) {
                    ScaffoldMessenger.of(context)
                        .showSnackBar(SnackBar(content: Text(L10n.t('calendar_add_error', {'e': e.toString()}))));
                  }
                }
              },
              child: Text(L10n.t('calendar_add')),
            ),
          ],
        ),
      ),
    ).then((_) {
      titleCtrl.dispose();
      descCtrl.dispose();
    });
  }
}
