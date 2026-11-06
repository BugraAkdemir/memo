import 'package:flutter/material.dart';

import '../core/l10n.dart';
import '../models/mail.dart';
import 'mail_reply_model_picker.dart';

/// Built-in morning sections, in the order the briefing shows them. The ids are
/// the backend's (`twin.BriefingSectionIDs`).
const briefingSectionIds = [
  'meetings',
  'overnight',
  'drafts',
  'waiting',
  'promises',
  'code',
];

/// Places besides the app the briefing can be sent to (`twin.BriefingChannelIDs`).
const briefingChannelIds = ['telegram', 'whatsapp', 'mail'];

const briefingMaxCustom = 5;

/// Morning times to pick from: the whole day in half-hours, plus the current
/// value if it is something else.
List<String> briefingTimeOptions(String current) {
  final times = {
    for (var h = 0; h < 24; h++) ...[
      '${h.toString().padLeft(2, '0')}:00',
      '${h.toString().padLeft(2, '0')}:30',
    ],
    current,
  }.toList()..sort();
  return times;
}

/// What can receive the briefing right now. A channel the owner has not set up
/// cannot be newly switched on, so a briefing is never promised to nowhere.
class BriefingChannelAvailability {
  final bool telegram;
  final bool whatsapp;
  final List<MailAccount> mailAccounts;

  const BriefingChannelAvailability({
    this.telegram = false,
    this.whatsapp = false,
    this.mailAccounts = const [],
  });

  bool ready(String channel) => switch (channel) {
    'telegram' => telegram,
    'whatsapp' => whatsapp,
    'mail' => mailAccounts.isNotEmpty,
    _ => false,
  };
}

/// What the morning briefing holds, where it is sent and which model writes it.
/// The same editor serves the setup wizard and the Settings panel: it edits a
/// copy of the briefing settings and hands the whole merged map to [onChanged],
/// which the host saves. Every change is its own save, so there is no draft.
class BriefingContentEditor extends StatefulWidget {
  final Map<String, dynamic> settings;
  final BriefingChannelAvailability availability;
  final bool busy;
  final ValueChanged<Map<String, dynamic>> onChanged;

  const BriefingContentEditor({
    super.key,
    required this.settings,
    required this.availability,
    required this.onChanged,
    this.busy = false,
  });

  @override
  State<BriefingContentEditor> createState() => _BriefingContentEditorState();
}

class _BriefingContentEditorState extends State<BriefingContentEditor> {
  final _title = TextEditingController();
  final _prompt = TextEditingController();

  @override
  void dispose() {
    _title.dispose();
    _prompt.dispose();
    super.dispose();
  }

  Map<String, dynamic> get _s => widget.settings;

  /// A briefing saved before sections existed has none listed: all are on.
  List<String> get _sections => _s['sections'] is List
      ? [for (final v in _s['sections'] as List) '$v']
      : [...briefingSectionIds];

  List<String> get _channels => _s['channels'] is List
      ? [for (final v in _s['channels'] as List) '$v']
      : <String>[];

  List<Map<String, String>> get _custom => [
    if (_s['custom'] is List)
      for (final c in _s['custom'] as List)
        if (c is Map)
          {'title': '${c['title'] ?? ''}', 'prompt': '${c['prompt'] ?? ''}'},
  ];

  Map<String, String> get _model {
    final m = _s['model'];
    return {
      'provider': m is Map ? '${m['provider'] ?? ''}' : '',
      'model': m is Map ? '${m['model'] ?? ''}' : '',
    };
  }

  void _set(String key, Object value) => widget.onChanged({..._s, key: value});

  void _toggle(String key, List<String> current, String id, bool on) {
    final next = [...current]..remove(id);
    if (on) next.add(id);
    _set(key, next);
  }

  void _addCustom() {
    final title = _title.text.trim();
    final prompt = _prompt.text.trim();
    if (title.isEmpty ||
        prompt.isEmpty ||
        _custom.length >= briefingMaxCustom) {
      return;
    }
    _set('custom', [
      ..._custom,
      {'title': title, 'prompt': prompt},
    ]);
    _title.clear();
    _prompt.clear();
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final custom = _custom;
    final channels = _channels;
    final accounts = widget.availability.mailAccounts;
    final account = _s['mail_account'] is String
        ? _s['mail_account'] as String
        : '';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _heading(L10n.t('briefing_sections_title')),
        for (final id in briefingSectionIds)
          CheckboxListTile(
            key: ValueKey('briefing-section-$id'),
            contentPadding: EdgeInsets.zero,
            dense: true,
            controlAffinity: ListTileControlAffinity.leading,
            title: Text(L10n.t('briefing_section_$id')),
            value: _sections.contains(id),
            onChanged: widget.busy
                ? null
                : (v) => _toggle('sections', _sections, id, v == true),
          ),
        const SizedBox(height: 12),
        _heading(L10n.t('briefing_custom_title')),
        Text(L10n.t('briefing_custom_help'), style: theme.textTheme.bodySmall),
        for (var i = 0; i < custom.length; i++)
          ListTile(
            key: ValueKey('briefing-custom-$i'),
            contentPadding: EdgeInsets.zero,
            dense: true,
            title: Text(custom[i]['title']!),
            subtitle: Text(
              custom[i]['prompt']!,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            trailing: IconButton(
              key: ValueKey('briefing-custom-remove-$i'),
              tooltip: L10n.t('briefing_custom_remove'),
              icon: const Icon(Icons.close),
              onPressed: widget.busy
                  ? null
                  : () => _set('custom', [...custom]..removeAt(i)),
            ),
          ),
        if (custom.length < briefingMaxCustom) ...[
          const SizedBox(height: 8),
          TextField(
            key: const ValueKey('briefing-custom-title'),
            controller: _title,
            maxLength: 60,
            decoration: InputDecoration(
              labelText: L10n.t('briefing_custom_item_title'),
              counterText: '',
            ),
            onChanged: (_) => setState(() {}),
          ),
          const SizedBox(height: 8),
          TextField(
            key: const ValueKey('briefing-custom-prompt'),
            controller: _prompt,
            maxLength: 500,
            minLines: 1,
            maxLines: 4,
            decoration: InputDecoration(
              labelText: L10n.t('briefing_custom_item_prompt'),
              counterText: '',
            ),
            onChanged: (_) => setState(() {}),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.centerLeft,
            child: FilledButton.tonal(
              key: const ValueKey('briefing-custom-add'),
              onPressed:
                  widget.busy ||
                      _title.text.trim().isEmpty ||
                      _prompt.text.trim().isEmpty
                  ? null
                  : _addCustom,
              child: Text(L10n.t('briefing_custom_add')),
            ),
          ),
        ] else
          Padding(
            padding: const EdgeInsets.only(top: 4),
            child: Text(
              L10n.t('briefing_custom_limit'),
              style: theme.textTheme.bodySmall,
            ),
          ),
        const SizedBox(height: 16),
        _heading(L10n.t('briefing_channels_title')),
        Text(
          L10n.t('briefing_channels_help'),
          style: theme.textTheme.bodySmall,
        ),
        for (final id in briefingChannelIds)
          CheckboxListTile(
            key: ValueKey('briefing-channel-$id'),
            contentPadding: EdgeInsets.zero,
            dense: true,
            controlAffinity: ListTileControlAffinity.leading,
            title: Text(L10n.t('briefing_channel_$id')),
            subtitle: Text(
              widget.availability.ready(id)
                  ? L10n.t('briefing_channel_ready')
                  : L10n.t('briefing_channel_${id}_missing'),
            ),
            value: channels.contains(id),
            // An unavailable channel can be switched off but not newly on.
            onChanged:
                widget.busy ||
                    (!widget.availability.ready(id) && !channels.contains(id))
                ? null
                : (v) => _toggle('channels', channels, id, v == true),
          ),
        if (channels.contains('mail') && accounts.length > 1)
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: DropdownButtonFormField<String>(
              key: ValueKey('briefing-mail-account-$account'),
              initialValue: accounts.any((a) => a.id == account)
                  ? account
                  : accounts.first.id,
              isExpanded: true,
              decoration: InputDecoration(
                labelText: L10n.t('briefing_mail_account'),
              ),
              items: [
                for (final a in accounts)
                  DropdownMenuItem(value: a.id, child: Text(a.email)),
              ],
              onChanged: widget.busy
                  ? null
                  : (v) {
                      if (v != null) _set('mail_account', v);
                    },
            ),
          ),
        const SizedBox(height: 16),
        _heading(L10n.t('briefing_model_title')),
        Text(L10n.t('briefing_model_help'), style: theme.textTheme.bodySmall),
        const SizedBox(height: 8),
        MailReplyModelPicker(
          showSavedSnackBar: false,
          providerLabelKey: 'briefing_model_provider',
          modelLabelKey: 'briefing_model_model',
          saveKey: const Key('briefing_model_save'),
          loadSelection: () async =>
              (provider: _model['provider']!, model: _model['model']!),
          saveSelection: (provider, model) async =>
              _set('model', {'provider': provider, 'model': model}),
        ),
      ],
    );
  }

  Widget _heading(String text) => Padding(
    padding: const EdgeInsets.only(bottom: 4),
    child: Text(text, style: Theme.of(context).textTheme.titleSmall),
  );
}
