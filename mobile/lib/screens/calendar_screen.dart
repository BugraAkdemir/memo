import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/calendar_event.dart';
import '../providers/calendar_provider.dart';

class CalendarScreen extends ConsumerStatefulWidget {
  const CalendarScreen({super.key});

  @override
  ConsumerState<CalendarScreen> createState() => _CalendarScreenState();
}

class _CalendarScreenState extends ConsumerState<CalendarScreen> {
  late DateTime _focusedDay;
  late DateTime _selectedDay;

  @override
  void initState() {
    super.initState();
    _focusedDay = DateTime.now();
    _selectedDay = DateTime.now();
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(calendarProvider);
    final notifier = ref.read(calendarProvider.notifier);

    return Scaffold(
      backgroundColor: MemoTheme.bg,
      appBar: AppBar(
        backgroundColor: MemoTheme.surface,
        title: Text(
          L10n.t('calendar_title'),
          style: TextStyle(color: MemoTheme.text, fontWeight: FontWeight.w600),
        ),
        actions: [
          IconButton(
            icon: Icon(Icons.add, color: MemoTheme.accent),
            tooltip: L10n.t('add_event'),
            onPressed: () => _showAddDialog(context, notifier),
          ),
          IconButton(
            icon: Icon(Icons.settings_outlined, color: MemoTheme.textDim),
            tooltip: L10n.t('reminder_settings'),
            onPressed: () => _showSettingsDialog(context, state.settings, notifier),
          ),
        ],
      ),
      body: state.loading
          ? const Center(child: CircularProgressIndicator())
          : Column(
              children: [
                _MonthHeader(
                  focusedDay: _focusedDay,
                  onPrev: () => setState(() {
                    _focusedDay = DateTime(_focusedDay.year, _focusedDay.month - 1);
                    _selectedDay = _focusedDay;
                    notifier.setFocusedDay(_focusedDay);
                  }),
                  onNext: () => setState(() {
                    _focusedDay = DateTime(_focusedDay.year, _focusedDay.month + 1);
                    _selectedDay = _focusedDay;
                    notifier.setFocusedDay(_focusedDay);
                  }),
                ),
                _CalendarGrid(
                  focusedDay: _focusedDay,
                  selectedDay: _selectedDay,
                  hasEvents: (day) => notifier.eventsForDay(day).isNotEmpty,
                  onDayTap: (day) => setState(() => _selectedDay = day),
                ),
                const Divider(height: 1),
                Expanded(
                  child: _DayEventList(
                    day: _selectedDay,
                    events: notifier.eventsForDay(_selectedDay),
                    onDelete: (id) => notifier.deleteEvent(id),
                  ),
                ),
              ],
            ),
    );
  }

  void _showAddDialog(BuildContext context, CalendarNotifier notifier) {
    final titleCtrl = TextEditingController();
    final descCtrl = TextEditingController();
    DateTime pickedTime = DateTime.now().add(const Duration(hours: 1));

    showDialog(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setS) => AlertDialog(
          backgroundColor: MemoTheme.surface,
          title: Text(L10n.t('new_event'),
              style: TextStyle(color: MemoTheme.text)),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: titleCtrl,
                style: TextStyle(color: MemoTheme.text),
                decoration: InputDecoration(
                  labelText: L10n.t('title_label'),
                  labelStyle: TextStyle(color: MemoTheme.textDim),
                ),
              ),
              const SizedBox(height: 8),
              TextField(
                controller: descCtrl,
                style: TextStyle(color: MemoTheme.text),
                decoration: InputDecoration(
                  labelText: L10n.t('description_optional'),
                  labelStyle: TextStyle(color: MemoTheme.textDim),
                ),
              ),
              const SizedBox(height: 12),
              ListTile(
                contentPadding: EdgeInsets.zero,
                title: Text(
                  DateFormat('dd MMM yyyy HH:mm', L10n.dateLocale).format(pickedTime),
                  style: TextStyle(color: MemoTheme.text),
                ),
                trailing: Icon(Icons.schedule, color: MemoTheme.accent),
                onTap: () async {
                  final date = await showDatePicker(
                    context: ctx,
                    initialDate: pickedTime,
                    firstDate: DateTime.now(),
                    lastDate: DateTime.now().add(const Duration(days: 365)),
                  );
                  if (date == null) return;
                  final time = await showTimePicker(
                    context: ctx,
                    initialTime: TimeOfDay.fromDateTime(pickedTime),
                  );
                  if (time == null) return;
                  setS(() {
                    pickedTime = DateTime(
                        date.year, date.month, date.day, time.hour, time.minute);
                  });
                },
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: Text(L10n.t('cancel'),
                  style: TextStyle(color: MemoTheme.textDim)),
            ),
            ElevatedButton(
              style: ElevatedButton.styleFrom(backgroundColor: MemoTheme.accent),
              onPressed: () {
                if (titleCtrl.text.trim().isEmpty) return;
                notifier.addEvent(
                  title: titleCtrl.text.trim(),
                  startTime: pickedTime,
                  description: descCtrl.text.trim(),
                );
                Navigator.pop(ctx);
              },
              child: Text(L10n.t('add')),
            ),
          ],
        ),
      ),
    );
  }

  void _showSettingsDialog(
      BuildContext context, CalendarSettings settings, CalendarNotifier notifier) {
    final options = [10, 15, 30, 60, 120];
    int selected = settings.reminderLeadMinutes;

    showDialog(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setS) => AlertDialog(
          backgroundColor: MemoTheme.surface,
          title: Text(L10n.t('reminder_lead_title'),
              style: TextStyle(color: MemoTheme.text)),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: options.map((min) {
              final label = min < 60
                  ? L10n.t('minutes_before', {'n': '$min'})
                  : L10n.t('hours_before', {'n': '${min ~/ 60}'});
              return RadioListTile<int>(
                title: Text(label,
                    style: TextStyle(color: MemoTheme.text)),
                value: min,
                groupValue: selected,
                activeColor: MemoTheme.accent,
                onChanged: (v) => setS(() => selected = v!),
              );
            }).toList(),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: Text(L10n.t('cancel'),
                  style: TextStyle(color: MemoTheme.textDim)),
            ),
            ElevatedButton(
              style: ElevatedButton.styleFrom(backgroundColor: MemoTheme.accent),
              onPressed: () {
                notifier.updateSettings(reminderLeadMinutes: selected);
                Navigator.pop(ctx);
              },
              child: Text(L10n.t('save')),
            ),
          ],
        ),
      ),
    );
  }
}

// ── Month header ──────────────────────────────────────────────────────────────

class _MonthHeader extends StatelessWidget {
  final DateTime focusedDay;
  final VoidCallback onPrev;
  final VoidCallback onNext;

  const _MonthHeader({
    required this.focusedDay,
    required this.onPrev,
    required this.onNext,
  });

  @override
  Widget build(BuildContext context) {
    final label = DateFormat('MMMM yyyy', L10n.dateLocale).format(focusedDay);
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      child: Row(
        children: [
          IconButton(
              onPressed: onPrev,
              icon: Icon(Icons.chevron_left, color: MemoTheme.textDim)),
          Expanded(
            child: Center(
              child: Text(label,
                  style: TextStyle(
                      color: MemoTheme.text,
                      fontWeight: FontWeight.w600,
                      fontSize: 16)),
            ),
          ),
          IconButton(
              onPressed: onNext,
              icon: Icon(Icons.chevron_right, color: MemoTheme.textDim)),
        ],
      ),
    );
  }
}

// ── Calendar grid ─────────────────────────────────────────────────────────────

class _CalendarGrid extends StatelessWidget {
  final DateTime focusedDay;
  final DateTime selectedDay;
  final bool Function(DateTime) hasEvents;
  final void Function(DateTime) onDayTap;

  const _CalendarGrid({
    required this.focusedDay,
    required this.selectedDay,
    required this.hasEvents,
    required this.onDayTap,
  });

  @override
  Widget build(BuildContext context) {
    final firstDay = DateTime(focusedDay.year, focusedDay.month, 1);
    final daysInMonth =
        DateTime(focusedDay.year, focusedDay.month + 1, 0).day;
    final startWeekday = (firstDay.weekday % 7); // Mon=1→1, Sun=7→0

    final dayNames = [
      L10n.t('day_mon'),
      L10n.t('day_tue'),
      L10n.t('day_wed'),
      L10n.t('day_thu'),
      L10n.t('day_fri'),
      L10n.t('day_sat'),
      L10n.t('day_sun'),
    ];

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 8),
      child: Column(
        children: [
          Row(
            children: dayNames
                .map((d) => Expanded(
                      child: Center(
                        child: Text(d,
                            style: TextStyle(
                                color: MemoTheme.textDim,
                                fontSize: 12,
                                fontWeight: FontWeight.w500)),
                      ),
                    ))
                .toList(),
          ),
          const SizedBox(height: 4),
          GridView.builder(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 7,
              childAspectRatio: 1,
            ),
            itemCount: startWeekday + daysInMonth,
            itemBuilder: (_, idx) {
              if (idx < startWeekday) return const SizedBox.shrink();
              final day = DateTime(
                  focusedDay.year, focusedDay.month, idx - startWeekday + 1);
              final isSelected = day.year == selectedDay.year &&
                  day.month == selectedDay.month &&
                  day.day == selectedDay.day;
              final isToday = day.year == DateTime.now().year &&
                  day.month == DateTime.now().month &&
                  day.day == DateTime.now().day;
              final dotVisible = hasEvents(day);

              return GestureDetector(
                onTap: () => onDayTap(day),
                child: Container(
                  margin: const EdgeInsets.all(2),
                  decoration: BoxDecoration(
                    color: isSelected
                        ? MemoTheme.accent
                        : isToday
                            ? MemoTheme.accent.withOpacity(0.15)
                            : Colors.transparent,
                    shape: BoxShape.circle,
                  ),
                  child: Stack(
                    alignment: Alignment.center,
                    children: [
                      Text(
                        '${day.day}',
                        style: TextStyle(
                          color: isSelected
                              ? Colors.white
                              : MemoTheme.text,
                          fontWeight: isToday
                              ? FontWeight.bold
                              : FontWeight.normal,
                          fontSize: 13,
                        ),
                      ),
                      if (dotVisible && !isSelected)
                        Positioned(
                          bottom: 4,
                          child: Container(
                            width: 4,
                            height: 4,
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
      ),
    );
  }
}

// ── Day event list ────────────────────────────────────────────────────────────

class _DayEventList extends StatelessWidget {
  final DateTime day;
  final List<CalendarEvent> events;
  final void Function(String id) onDelete;

  const _DayEventList({
    required this.day,
    required this.events,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final label = DateFormat('d MMMM', L10n.dateLocale).format(day);

    if (events.isEmpty) {
      return Center(
        child: Text(
          L10n.t('no_events_day', {'label': label}),
          style: TextStyle(color: MemoTheme.textDim),
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
          child: Text(label,
              style: TextStyle(
                  color: MemoTheme.textDim,
                  fontSize: 13,
                  fontWeight: FontWeight.w500)),
        ),
        Expanded(
          child: ListView.builder(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            itemCount: events.length,
            itemBuilder: (_, i) => _EventTile(
              event: events[i],
              onDelete: onDelete,
            ),
          ),
        ),
      ],
    );
  }
}

class _EventTile extends StatelessWidget {
  final CalendarEvent event;
  final void Function(String) onDelete;

  const _EventTile({required this.event, required this.onDelete});

  @override
  Widget build(BuildContext context) {
    final timeStr = DateFormat('HH:mm').format(event.startTime);
    final sourceIcon = switch (event.source) {
      'whatsapp' => Icons.chat_bubble_outline,
      'chat' => Icons.smart_toy_outlined,
      _ => Icons.calendar_today_outlined,
    };

    return Card(
      color: MemoTheme.surface,
      margin: const EdgeInsets.symmetric(vertical: 4),
      child: ListTile(
        leading: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(sourceIcon, size: 16, color: MemoTheme.accent),
            const SizedBox(height: 2),
            Text(timeStr,
                style: TextStyle(
                    color: MemoTheme.textDim,
                    fontSize: 11,
                    fontWeight: FontWeight.w600)),
          ],
        ),
        title: Text(event.title,
            style: TextStyle(
                color: MemoTheme.text, fontWeight: FontWeight.w500)),
        subtitle: event.contactName.isNotEmpty
            ? Text(event.contactName,
                style: TextStyle(color: MemoTheme.textDim, fontSize: 12))
            : null,
        trailing: IconButton(
          icon: Icon(Icons.delete_outline, color: MemoTheme.textDim),
          onPressed: () => onDelete(event.id),
        ),
      ),
    );
  }
}
