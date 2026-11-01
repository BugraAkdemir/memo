import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../models/twin.dart';
import 'autonomy_panel.dart';
import '../providers/chat_provider.dart';

/// Owner-managed identity links and read-only evidence. This panel does not
/// expose the owner's bot/self-chat history.
class TwinDialog extends ConsumerStatefulWidget {
  const TwinDialog({super.key, this.initialTab = 0, this.initialContactId});
  final int initialTab;
  final String? initialContactId;
  @override
  ConsumerState<TwinDialog> createState() => _TwinDialogState();
}

class _TwinDialogState extends ConsumerState<TwinDialog> {
  TwinState? _state;
  bool _initialContactApplied = false;
  bool _busy = false;
  String? _error;
  String? _selected;
  String? _project;
  String _policy = 'default';
  final _name = TextEditingController();
  final _address = TextEditingController();
  final _timezone = TextEditingController();
  final _start = TextEditingController();
  final _end = TextEditingController();
  String _channel = 'mail';
  List<TwinAlias> _aliases = [];
  List<dynamic>? _timeline;
  Map<String, dynamic>? _git;
  Map<String, dynamic>? _availability;
  bool _inspect = false;
  DateTime _day = DateTime.now();

  @override
  void initState() {
    super.initState();
    Future.microtask(_load);
  }

  @override
  void dispose() {
    for (final c in [_name, _address, _timezone, _start, _end]) {
      c.dispose();
    }
    super.dispose();
  }

  String _describe(Object e) {
    if (e is DioException && e.response?.data is Map) {
      final code = (e.response!.data as Map)['error'];
      if (const {
        'twin_invalid',
        'twin_alias_conflict',
        'twin_not_found',
        'twin_denied',
        'twin_unavailable',
        'twin_failed',
      }.contains(code)) {
        return L10n.t(code as String);
      }
    }
    return L10n.t('twin_failed');
  }

  Future<void> _run(Future<void> Function() action) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await action();
    } catch (e) {
      if (mounted) setState(() => _error = _describe(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _refresh() async {
    final state = await ref.read(apiClientProvider).getTwinState();
    if (!mounted) return;
    setState(() {
      _state = state;
      _timezone.text = state.preferences['timezone'] as String;
      _start.text = state.preferences['work_start'] as String;
      _end.text = state.preferences['work_end'] as String;
      _inspect = state.preferences['allow_project_inspection'] == true;
      if (!state.projects.any((p) => p['id'] == _project)) _project = null;
    });
    if (!_initialContactApplied) {
      _initialContactApplied = true;
      final contact = state.contacts
          .where((contact) => contact.id == widget.initialContactId)
          .firstOrNull;
      if (contact != null) _select(contact);
    }
  }

  Future<void> _load() => _run(_refresh);

  void _select(TwinContact? contact) {
    setState(() {
      _selected = contact?.id;
      _name.text = contact?.name ?? '';
      _policy = contact?.policy ?? 'default';
      _aliases = List.of(contact?.aliases ?? []);
      _address.clear();
      _timeline = null;
      _error = null;
    });
  }

  Future<void> _saveContact() => _run(() async {
    if (_name.text.trim().isEmpty || _aliases.isEmpty) {
      setState(() => _error = L10n.t('twin_invalid'));
      return;
    }
    final result = await ref.read(apiClientProvider).callTwin('contact', {
      'id': _selected ?? '',
      'name': _name.text.trim(),
      'autonomy_policy': _policy,
      'aliases': _aliases.map((a) => a.toJson()).toList(),
    });
    if (!mounted) return;
    _selected = (result as Map)['id'] as String;
    await _refresh();
  });

  Future<void> _merge(TwinContact source) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(L10n.t('twin_merge')),
        content: Text(
          L10n.t('twin_merge_confirm', {
            'source': source.name,
            'target': _name.text,
          }),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: Text(L10n.t('cancel')),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: Text(L10n.t('twin_merge')),
          ),
        ],
      ),
    );
    if (ok != true || !mounted) return;
    await _run(() async {
      await ref.read(apiClientProvider).callTwin('merge', {
        'target_id': _selected,
        'source_id': source.id,
      });
      if (!mounted) return;
      await _refresh();
      if (!mounted) return;
      _select(_state!.contacts.firstWhere((c) => c.id == _selected));
    });
  }

  Widget _button(String key, VoidCallback action, {IconData? icon}) =>
      OutlinedButton.icon(
        onPressed: _busy ? null : action,
        icon: Icon(icon ?? Icons.check, size: 18),
        style: OutlinedButton.styleFrom(minimumSize: const Size(0, 44)),
        label: Text(L10n.t(key)),
      );

  Widget _contacts() {
    final contacts = _state!.contacts;
    final selected = contacts.where((c) => c.id == _selected).firstOrNull;
    return LayoutBuilder(
      builder: (context, constraints) {
        final compact = constraints.maxWidth < 650;
        return Flex(
          direction: compact ? Axis.vertical : Axis.horizontal,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            SizedBox(
              width: compact ? null : 230,
              height: compact ? 130 : null,
              child: Column(
                children: [
                  _button(
                    'twin_new_contact',
                    () => _select(null),
                    icon: Icons.person_add_outlined,
                  ),
                  Expanded(
                    child: contacts.isEmpty
                        ? Center(child: Text(L10n.t('twin_no_contacts')))
                        : ListView(
                            children: [
                              for (final c in contacts)
                                ListTile(
                                  selected: c.id == _selected,
                                  title: Text(
                                    c.name,
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                  subtitle: Text(
                                    L10n.t('twin_policy_${c.policy}'),
                                    style: TextStyle(
                                      color: c.policy == 'never'
                                          ? Theme.of(context).colorScheme.error
                                          : null,
                                    ),
                                  ),
                                  leading: const Icon(
                                    Icons.person_outline,
                                    size: 20,
                                  ),
                                  onTap: _busy ? null : () => _select(c),
                                ),
                            ],
                          ),
                  ),
                ],
              ),
            ),
            if (!compact) const VerticalDivider(),
            Expanded(
              child: ListView(
                key: const ValueKey('twin_contact_editor'),
                padding: const EdgeInsets.all(20),
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          L10n.t(
                            _selected == null
                                ? 'twin_new_contact'
                                : 'twin_contact_details',
                          ),
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                      ),
                      const SizedBox(width: 12),
                      FilledButton.icon(
                        onPressed: _busy ? null : _saveContact,
                        icon: const Icon(Icons.check, size: 18),
                        label: Text(L10n.t('save')),
                        style: FilledButton.styleFrom(
                          minimumSize: const Size(0, 44),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 16),
                  TextField(
                    key: const ValueKey('twin_name'),
                    controller: _name,
                    enabled: !_busy,
                    decoration: InputDecoration(labelText: L10n.t('twin_name')),
                  ),
                  const SizedBox(height: 12),
                  DropdownButtonFormField<String>(
                    isExpanded: true,
                    key: ValueKey(_policy),
                    initialValue: _policy,
                    decoration: InputDecoration(
                      labelText: L10n.t('twin_contact_policy'),
                    ),
                    items: [
                      for (final p in [
                        'default',
                        'draft_only',
                        'full_auto',
                        'never',
                      ])
                        DropdownMenuItem(
                          value: p,
                          child: Text(L10n.t('twin_policy_$p')),
                        ),
                    ],
                    onChanged: _busy
                        ? null
                        : (v) => setState(() => _policy = v!),
                  ),
                  const SizedBox(height: 12),
                  Text(
                    L10n.t('twin_contact_policy_help'),
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                  const SizedBox(height: 24),
                  Text(L10n.t('twin_alias_help')),
                  for (final a in _aliases)
                    ListTile(
                      dense: true,
                      title: Text(
                        '${L10n.t('twin_channel_${a.channel}')}: ${a.address}',
                      ),
                      trailing: IconButton(
                        tooltip: L10n.t('twin_remove_alias'),
                        onPressed: _busy
                            ? null
                            : () => setState(() => _aliases.remove(a)),
                        icon: const Icon(Icons.close),
                      ),
                    ),
                  Row(
                    children: [
                      DropdownButton<String>(
                        value: _channel,
                        items: [
                          for (final ch in ['mail', 'whatsapp', 'telegram'])
                            DropdownMenuItem(
                              value: ch,
                              child: Text(L10n.t('twin_channel_$ch')),
                            ),
                        ],
                        onChanged: _busy
                            ? null
                            : (v) => setState(() => _channel = v!),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: TextField(
                          key: const ValueKey('twin_address'),
                          controller: _address,
                          enabled: !_busy,
                          decoration: InputDecoration(
                            labelText: L10n.t('twin_address'),
                          ),
                        ),
                      ),
                      IconButton(
                        tooltip: L10n.t('twin_add_alias'),
                        onPressed: _busy
                            ? null
                            : () {
                                if (_address.text.trim().isEmpty) return;
                                setState(() {
                                  _aliases.add(
                                    TwinAlias(_channel, _address.text.trim()),
                                  );
                                  _address.clear();
                                });
                              },
                        icon: const Icon(Icons.add),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  Wrap(
                    spacing: 10,
                    children: [
                      if (_selected != null)
                        _button(
                          'twin_timeline',
                          () => _run(() async {
                            final data = await ref
                                .read(apiClientProvider)
                                .callTwin('contact_timeline', {
                                  'contact_id': _selected,
                                });
                            if (mounted) {
                              setState(() => _timeline = data as List);
                            }
                          }),
                          icon: Icons.history,
                        ),
                      if (_selected != null && contacts.length > 1)
                        PopupMenuButton<TwinContact>(
                          enabled: !_busy,
                          tooltip: L10n.t('twin_merge'),
                          onSelected: _merge,
                          itemBuilder: (_) => [
                            for (final c in contacts.where(
                              (c) => c.id != _selected,
                            ))
                              PopupMenuItem(value: c, child: Text(c.name)),
                          ],
                          child: Padding(
                            padding: const EdgeInsets.all(12),
                            child: Text(L10n.t('twin_merge')),
                          ),
                        ),
                    ],
                  ),
                  if (selected != null) ...[
                    const Divider(),
                    Text(
                      L10n.t('twin_style', {
                        'n': '${selected.style['samples']}',
                        'tone': L10n.t('twin_tone_${selected.style['tone']}'),
                        'language': L10n.t(
                          'twin_language_${selected.style['language']}',
                        ),
                      }),
                    ),
                    if (selected.style['sufficient'] != true)
                      Text(L10n.t('twin_style_insufficient')),
                  ],
                  if (_timeline != null) ...[
                    const Divider(),
                    if (_timeline!.isEmpty) Text(L10n.t('twin_no_history')),
                    for (final raw in _timeline!) ...[
                      ListTile(
                        title: SelectableText((raw as Map)['body'] as String),
                        subtitle: Text(
                          '${L10n.t(raw['outgoing'] == true ? 'twin_outgoing' : 'twin_incoming')} · ${L10n.t('twin_channel_${raw['channel']}')} · ${raw['sent_at']}',
                        ),
                      ),
                      const Divider(),
                    ],
                  ],
                ],
              ),
            ),
          ],
        );
      },
    );
  }

  Widget _preferences() => ListView(
    padding: const EdgeInsets.all(24),
    children: [
      Text(
        L10n.t('twin_hours_heading'),
        style: Theme.of(context).textTheme.titleMedium,
      ),
      const SizedBox(height: 8),
      Text(L10n.t('twin_preferences_help')),
      const SizedBox(height: 20),
      TextField(
        controller: _timezone,
        decoration: InputDecoration(labelText: L10n.t('twin_timezone')),
      ),
      const SizedBox(height: 16),
      TextField(
        controller: _start,
        decoration: InputDecoration(labelText: L10n.t('twin_work_start')),
      ),
      const SizedBox(height: 16),
      TextField(
        controller: _end,
        decoration: InputDecoration(labelText: L10n.t('twin_work_end')),
      ),
      const SizedBox(height: 20),
      SwitchListTile(
        contentPadding: EdgeInsets.zero,
        title: Text(L10n.t('twin_project_access')),
        value: _inspect,
        onChanged: _busy ? null : (v) => setState(() => _inspect = v),
      ),
      _button(
        'save',
        () => _run(() async {
          await ref.read(apiClientProvider).callTwin('preferences', {
            'timezone': _timezone.text.trim(),
            'work_start': _start.text.trim(),
            'work_end': _end.text.trim(),
            'allow_project_inspection': _inspect,
            'autonomy': _state!.preferences['autonomy'] ?? {},
          });
          if (!mounted) return;
          setState(() {
            _git = null;
            _availability = null;
          });
          await _refresh();
        }),
      ),
    ],
  );

  Widget _autonomy() => AutonomyPanel(
    replyModel: _state!.replyModel,
    settings: _state!.preferences['autonomy'] is Map
        ? Map<String, dynamic>.from(_state!.preferences['autonomy'] as Map)
        : {},
    actions: _state!.actions,
    busy: _busy,
    onSettings: (value) => _run(() async {
      await ref.read(apiClientProvider).callTwin('preferences', {
        ..._state!.preferences,
        'autonomy': value,
      });
      await _refresh();
    }),
    onCancel: (id) => _run(() async {
      await ref.read(apiClientProvider).callTwin('cancel_action', {'id': id});
      await _refresh();
    }),
  );

  Widget _tools() => ListView(
    padding: const EdgeInsets.all(16),
    children: [
      Text(
        L10n.t('twin_git_heading'),
        style: Theme.of(context).textTheme.titleMedium,
      ),
      const SizedBox(height: 8),
      Text(L10n.t('twin_readonly_help')),
      const SizedBox(height: 20),
      DropdownButtonFormField<String>(
        isExpanded: true,
        key: ValueKey(_project),
        initialValue: _project,
        decoration: InputDecoration(labelText: L10n.t('twin_project')),
        items: [
          for (final p in _state!.projects)
            DropdownMenuItem(
              value: p['id'] as String,
              child: Text(p['name'] as String),
            ),
        ],
        onChanged: _busy
            ? null
            : (v) => setState(() {
                _project = v;
                _git = null;
              }),
      ),
      _button(
        'twin_inspect_git',
        () => _run(() async {
          setState(() => _git = null);
          final data = await ref.read(apiClientProvider).callTwin(
            'git_project_status',
            {'project_id': _project ?? ''},
          );
          if (mounted) {
            setState(() => _git = Map<String, dynamic>.from(data as Map));
          }
        }),
        icon: Icons.account_tree_outlined,
      ),
      if (_state!.projects.isEmpty) Text(L10n.t('twin_no_projects')),
      if (_git != null)
        for (final field in [
          'status',
          'merged_branches',
          'recent_commits',
        ]) ...[
          Text(
            L10n.t('twin_git_$field'),
            style: const TextStyle(fontWeight: FontWeight.bold),
          ),
          SelectableText(
            (_git![field] as String).isEmpty
                ? L10n.t('twin_git_empty')
                : _git![field] as String,
          ),
          const SizedBox(height: 12),
        ],
      const Divider(height: 40),
      Text(
        L10n.t('twin_calendar_heading'),
        style: Theme.of(context).textTheme.titleMedium,
      ),
      const SizedBox(height: 16),
      _button('twin_pick_day', () async {
        final day = await showDatePicker(
          context: context,
          initialDate: _day,
          firstDate: DateTime(2000),
          lastDate: DateTime(2100),
        );
        if (day != null && mounted) {
          setState(() {
            _day = day;
            _availability = null;
          });
        }
      }, icon: Icons.calendar_month),
      Text(MaterialLocalizations.of(context).formatFullDate(_day)),
      Text(L10n.t('twin_device_day')),
      _button(
        'twin_check_availability',
        () => _run(() async {
          setState(() => _availability = null);
          final from = DateTime(_day.year, _day.month, _day.day);
          final to = DateTime(_day.year, _day.month, _day.day + 1);
          final data = await ref
              .read(apiClientProvider)
              .callTwin('check_availability', {
                'from': from.toUtc().toIso8601String(),
                'to': to.toUtc().toIso8601String(),
              });
          if (mounted) {
            setState(
              () => _availability = Map<String, dynamic>.from(data as Map),
            );
          }
        }),
        icon: Icons.schedule,
      ),
      if (_availability != null) ...[
        Text(
          L10n.t('twin_work_summary', {
            'timezone': '${_availability!['timezone']}',
            'start': '${_availability!['work_start']}',
            'end': '${_availability!['work_end']}',
          }),
        ),
        if ((_availability!['busy'] as List).isEmpty)
          Text(L10n.t('twin_no_busy')),
        for (final b in _availability!['busy'] as List)
          Text(
            L10n.t('twin_busy_window', {
              'start': '${b['start']}',
              'end': b['end']?.toString() ?? L10n.t('twin_end_unknown'),
            }),
          ),
      ],
    ],
  );

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return DefaultTabController(
      initialIndex: widget.initialTab,
      length: 4,
      child: Dialog(
        insetPadding: const EdgeInsets.symmetric(horizontal: 20, vertical: 20),
        clipBehavior: Clip.antiAlias,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 1080, maxHeight: 760),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(24, 16, 12, 12),
                child: Row(
                  children: [
                    Icon(
                      Icons.auto_awesome_outlined,
                      color: theme.colorScheme.primary,
                      size: 26,
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            L10n.t('twin_title'),
                            style: theme.textTheme.titleLarge,
                          ),
                          const SizedBox(height: 4),
                          Text(
                            L10n.t('twin_subtitle'),
                            style: theme.textTheme.bodySmall,
                          ),
                        ],
                      ),
                    ),
                    IconButton(
                      tooltip: L10n.t('refresh'),
                      onPressed: _busy ? null : _load,
                      icon: const Icon(Icons.refresh),
                    ),
                    IconButton(
                      tooltip: L10n.t('close'),
                      onPressed: () => Navigator.pop(context),
                      icon: const Icon(Icons.close),
                    ),
                  ],
                ),
              ),
              TabBar(
                isScrollable: true,
                tabAlignment: TabAlignment.start,
                padding: const EdgeInsets.symmetric(horizontal: 12),
                tabs: [
                  for (final k in [
                    'twin_contacts_tab',
                    'twin_preferences_tab',
                    'twin_tools_tab',
                    'auto_tab',
                  ])
                    Tab(text: L10n.t(k)),
                ],
              ),
              SizedBox(
                height: 3,
                child: _busy ? const LinearProgressIndicator() : null,
              ),
              if (_error != null)
                Container(
                  color: theme.colorScheme.errorContainer,
                  padding: const EdgeInsets.symmetric(
                    horizontal: 24,
                    vertical: 12,
                  ),
                  child: Row(
                    children: [
                      Icon(
                        Icons.error_outline,
                        color: theme.colorScheme.onErrorContainer,
                        size: 20,
                      ),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Text(
                          _error!,
                          style: TextStyle(
                            color: theme.colorScheme.onErrorContainer,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              Expanded(
                child: _state == null
                    ? Center(
                        child: _busy
                            ? const CircularProgressIndicator()
                            : _button('retry', _load),
                      )
                    : TabBarView(
                        children: [
                          _contacts(),
                          _preferences(),
                          _tools(),
                          _autonomy(),
                        ],
                      ),
              ),
              const Divider(height: 1),
              Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: 24,
                  vertical: 12,
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      Icons.info_outline,
                      size: 16,
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Text(
                        L10n.t('twin_scope_notice'),
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
