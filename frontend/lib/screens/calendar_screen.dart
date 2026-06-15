import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/theme.dart';
import '../providers/chat_provider.dart';

/// A calendar event as returned by /api/calendar/events.
class _Event {
  final String id;
  final String title;
  final DateTime startTime;
  final String description;
  final String source;
  final String contactName;

  _Event({
    required this.id,
    required this.title,
    required this.startTime,
    required this.description,
    required this.source,
    required this.contactName,
  });

  factory _Event.fromJson(Map<String, dynamic> j) => _Event(
        id: j['id'] as String? ?? '',
        title: j['title'] as String? ?? '',
        startTime: DateTime.parse(j['start_time'] as String).toLocal(),
        description: j['description'] as String? ?? '',
        source: j['source'] as String? ?? 'manual',
        contactName: j['contact_name'] as String? ?? '',
      );
}

const _monthNames = [
  'Ocak', 'Şubat', 'Mart', 'Nisan', 'Mayıs', 'Haziran',
  'Temmuz', 'Ağustos', 'Eylül', 'Ekim', 'Kasım', 'Aralık',
];
const _dayNames = ['Pzt', 'Sal', 'Çar', 'Per', 'Cum', 'Cmt', 'Paz'];

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
    // Defer to after first frame: _load() calls setState, which is not allowed
    // synchronously inside initState.
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
    // Auto-refresh so events added from chat/WhatsApp appear without a restart.
    _refreshTimer = Timer.periodic(
        const Duration(seconds: 20), (_) => _load(silent: true));
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    super.dispose();
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
        _error = '$e';
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
    try {
      await ref.read(apiClientProvider).deleteCalendarEvent(id);
      await _load();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Silinemedi: $e')));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
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
                    ? Center(child: Text('Yüklenemedi: $_error',
                        style: TextStyle(color: c.textDim)))
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
          Text('Takvim',
              style: TextStyle(
                  fontSize: 17, fontWeight: FontWeight.w600, color: c.textMain)),
          const Spacer(),
          IconButton(
            tooltip: 'Önceki ay',
            icon: Icon(Icons.chevron_left, color: c.textDim),
            onPressed: () {
              setState(() {
                _focused = DateTime(_focused.year, _focused.month - 1);
                _selected = _focused;
              });
              _load();
            },
          ),
          Text('${_monthNames[_focused.month - 1]} ${_focused.year}',
              style: TextStyle(
                  fontSize: 15, fontWeight: FontWeight.w500, color: c.textMain)),
          IconButton(
            tooltip: 'Sonraki ay',
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
            tooltip: 'Yenile',
            icon: Icon(Icons.refresh, color: c.textDim),
            onPressed: () => _load(),
          ),
          const SizedBox(width: 8),
          FilledButton.icon(
            style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
            icon: const Icon(Icons.add, size: 18),
            label: const Text('Etkinlik'),
            onPressed: () => _showAddDialog(c),
          ),
        ],
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
          children: _dayNames
              .map((d) => Expanded(
                    child: Center(
                      child: Text(d,
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
    final label = '${_selected.day} ${_monthNames[_selected.month - 1]}';

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
                  child: Text('Bu gün için etkinlik yok',
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
                Text(e.title,
                    style: TextStyle(
                        color: c.textMain, fontWeight: FontWeight.w500, fontSize: 14)),
                if (e.contactName.isNotEmpty)
                  Text(e.contactName,
                      style: TextStyle(color: c.textDim, fontSize: 12)),
              ],
            ),
          ),
          IconButton(
            tooltip: 'Sil',
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
          title: Text('Yeni Etkinlik', style: TextStyle(color: c.textMain)),
          content: SizedBox(
            width: 360,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                  controller: titleCtrl,
                  style: TextStyle(color: c.textMain),
                  decoration: const InputDecoration(labelText: 'Başlık'),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: descCtrl,
                  style: TextStyle(color: c.textMain),
                  decoration: const InputDecoration(labelText: 'Açıklama (opsiyonel)'),
                ),
                const SizedBox(height: 14),
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        '${picked.day} ${_monthNames[picked.month - 1]} ${picked.year}  ${_two(picked.hour)}:${_two(picked.minute)}',
                        style: TextStyle(color: c.textMain, fontSize: 13),
                      ),
                    ),
                    TextButton(
                      child: const Text('Tarih/Saat'),
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
                child: const Text('İptal')),
            FilledButton(
              style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
              onPressed: () async {
                if (titleCtrl.text.trim().isEmpty) return;
                Navigator.pop(ctx);
                try {
                  await ref.read(apiClientProvider).addCalendarEvent(
                      titleCtrl.text.trim(), picked, descCtrl.text.trim());
                  await _load();
                } catch (e) {
                  if (mounted) {
                    ScaffoldMessenger.of(context)
                        .showSnackBar(SnackBar(content: Text('Eklenemedi: $e')));
                  }
                }
              },
              child: const Text('Ekle'),
            ),
          ],
        ),
      ),
    );
  }
}
