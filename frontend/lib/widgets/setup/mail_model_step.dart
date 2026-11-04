import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../providers/chat_provider.dart';
import '../mail_reply_model_picker.dart';
import 'setup_widgets.dart';

/// Required once a mail account is connected: pick the model that writes reply
/// drafts. Below it, one safe switch: prepare reply drafts for incoming mail.
/// Drafts only: nothing is ever sent without the owner approving it, because
/// every contact starts on the draft-only policy.
class SetupMailModelStep extends ConsumerStatefulWidget {
  final ThemeColors color;
  final bool modelSet;
  final ValueChanged<bool> onModelChanged;

  const SetupMailModelStep({
    super.key,
    required this.color,
    required this.modelSet,
    required this.onModelChanged,
  });

  @override
  ConsumerState<SetupMailModelStep> createState() => _SetupMailModelStepState();
}

/// Common zones to pick from, and the zone to suggest for a given UTC offset in
/// minutes (the device only exposes an offset, not an IANA name).
const setupTimezones = [
  'UTC',
  'Europe/London',
  'Europe/Berlin',
  'Europe/Athens',
  'Europe/Istanbul',
  'Asia/Dubai',
  'Asia/Kolkata',
  'Asia/Tokyo',
  'America/New_York',
  'America/Chicago',
  'America/Los_Angeles',
];

const _zoneByOffsetMinutes = {
  0: 'UTC',
  60: 'Europe/Berlin',
  120: 'Europe/Athens',
  180: 'Europe/Istanbul',
  240: 'Asia/Dubai',
  330: 'Asia/Kolkata',
  540: 'Asia/Tokyo',
  -300: 'America/New_York',
  -360: 'America/Chicago',
  -480: 'America/Los_Angeles',
};

/// The zone that matches this device's current UTC offset, if we know one.
String? suggestedTimezone([Duration? offset]) =>
    _zoneByOffsetMinutes[(offset ?? DateTime.now().timeZoneOffset).inMinutes];

class _SetupMailModelStepState extends ConsumerState<SetupMailModelStep> {
  Map<String, dynamic>? _prefs;
  bool _savingDrafts = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadPrefs();
  }

  Future<void> _loadPrefs() async {
    try {
      final state = await ref.read(apiClientProvider).getTwinState();
      if (mounted) setState(() => _prefs = Map.of(state.preferences));
    } catch (_) {
      if (mounted) setState(() => _error = L10n.t('twin_failed'));
    }
  }

  bool get _draftsOn {
    final a = _prefs?['autonomy'];
    return a is Map && a['enabled'] == true && a['mail'] == true;
  }

  Future<void> _setDrafts(bool on) async {
    final prefs = _prefs;
    if (prefs == null) return;
    final next = Map<String, dynamic>.of(prefs);
    final autonomy = Map<String, dynamic>.of(
      (prefs['autonomy'] as Map?)?.cast<String, dynamic>() ?? const {},
    );
    autonomy['enabled'] = on;
    autonomy['mail'] = on;
    // Always the safe mode: a draft is prepared, never sent on its own.
    autonomy['mode'] = 'draft_only';
    next['autonomy'] = autonomy;
    await _savePrefs(next);
  }

  Future<void> _setTimezone(String zone) async {
    final prefs = _prefs;
    if (prefs == null) return;
    await _savePrefs({...prefs, 'timezone': zone});
  }

  Future<void> _savePrefs(Map<String, dynamic> next) async {
    if (_savingDrafts) return;
    setState(() {
      _savingDrafts = true;
      _error = null;
    });
    try {
      await ref.read(apiClientProvider).callTwin('preferences', next);
      if (mounted) setState(() => _prefs = next);
    } catch (_) {
      if (mounted) setState(() => _error = L10n.t('mail_reply_save_failed'));
    } finally {
      if (mounted) setState(() => _savingDrafts = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = widget.color;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SetupCard(
          color: c,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                L10n.t('setup_mail_model_help'),
                style: TextStyle(color: c.textDim, fontSize: 12, height: 1.5),
              ),
              const SizedBox(height: 12),
              if (widget.modelSet)
                Row(
                  key: const Key('setup_mail_model_ready'),
                  children: [
                    const Icon(
                      Icons.check_circle_outline,
                      size: 18,
                      color: MemoTheme.green,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        L10n.t('setup_mail_model_ready'),
                        style: const TextStyle(
                          color: MemoTheme.green,
                          fontSize: 13,
                        ),
                      ),
                    ),
                  ],
                )
              else
                Text(
                  L10n.t('setup_mail_model_required'),
                  key: const Key('setup_mail_model_required'),
                  style: const TextStyle(
                    fontSize: 12,
                    color: MemoTheme.warningOrange,
                    height: 1.3,
                  ),
                ),
              const SizedBox(height: 12),
              MailReplyModelPicker(
                showSavedSnackBar: false,
                onSaved: (provider, model) => widget.onModelChanged(
                  provider.isNotEmpty && model.isNotEmpty,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        SetupPreferenceToggle(
          icon: Icons.drafts_outlined,
          title: L10n.t('setup_mail_drafts_title'),
          desc: L10n.t('setup_mail_drafts_desc'),
          value: _draftsOn,
          color: c,
          onChanged: _prefs == null || _savingDrafts ? null : _setDrafts,
        ),
        const SizedBox(height: 12),
        _timezoneCard(c),
        if (_error != null)
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Text(_error!, style: const TextStyle(color: MemoTheme.red)),
          ),
      ],
    );
  }

  Widget _timezoneCard(ThemeColors c) {
    final current = '${_prefs?['timezone'] ?? 'UTC'}';
    final zones = {...setupTimezones, current}.toList();
    final suggestion = suggestedTimezone();
    return SetupCard(
      color: c,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            L10n.t('setup_timezone_title'),
            style: TextStyle(color: c.textMain, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 4),
          Text(
            L10n.t('setup_timezone_desc'),
            style: TextStyle(color: c.textDim, fontSize: 12, height: 1.5),
          ),
          const SizedBox(height: 10),
          DropdownButtonFormField<String>(
            key: ValueKey('setup_timezone_$current'),
            initialValue: current,
            isExpanded: true,
            items: [
              for (final z in zones) DropdownMenuItem(value: z, child: Text(z)),
            ],
            onChanged: _prefs == null || _savingDrafts
                ? null
                : (v) {
                    if (v != null && v != current) _setTimezone(v);
                  },
          ),
          if (suggestion != null && suggestion != current) ...[
            const SizedBox(height: 8),
            TextButton.icon(
              key: const Key('setup_timezone_suggest'),
              onPressed: _prefs == null || _savingDrafts
                  ? null
                  : () => _setTimezone(suggestion),
              icon: const Icon(Icons.schedule, size: 16),
              label: Text(
                L10n.t('setup_timezone_suggest', {'zone': suggestion}),
              ),
            ),
          ],
        ],
      ),
    );
  }
}
