import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../providers/chat_provider.dart';
import '../../providers/mail_provider.dart';
import '../../providers/telegram_provider.dart';
import '../../providers/whatsapp_provider.dart';
import '../briefing_content_editor.dart';
import 'mail_model_step.dart';
import 'setup_widgets.dart';

/// Optional step, right after the channels are connected: the daily briefing.
/// It is the same editor as the Settings panel, so what is chosen here is what
/// Settings shows afterwards. Everything lives in the twin preferences, which
/// this step reads and writes as a whole so nothing else in them is lost.
class SetupBriefingStep extends ConsumerStatefulWidget {
  final ThemeColors color;

  /// Told whenever the morning briefing is switched on or off, so the wizard's
  /// summary stays right.
  final ValueChanged<bool> onChanged;

  const SetupBriefingStep({
    super.key,
    required this.color,
    required this.onChanged,
  });

  @override
  ConsumerState<SetupBriefingStep> createState() => _SetupBriefingStepState();
}

class _SetupBriefingStepState extends ConsumerState<SetupBriefingStep> {
  Map<String, dynamic>? _prefs;
  bool _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final state = await ref.read(apiClientProvider).getTwinState();
      if (mounted) setState(() => _prefs = Map.of(state.preferences));
    } catch (_) {
      if (mounted) setState(() => _error = L10n.t('twin_failed'));
    }
  }

  Map<String, dynamic> get _briefing => _prefs?['briefing'] is Map
      ? Map<String, dynamic>.from(_prefs!['briefing'] as Map)
      : <String, dynamic>{};

  Future<void> _save(Map<String, dynamic> next) async {
    final prefs = _prefs;
    if (prefs == null || _saving) return;
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      final all = {...prefs, ...next};
      await ref.read(apiClientProvider).callTwin('preferences', all);
      if (!mounted) return;
      setState(() => _prefs = all);
      widget.onChanged((all['briefing'] as Map?)?['morning'] == true);
    } catch (_) {
      if (mounted) setState(() => _error = L10n.t('mail_reply_save_failed'));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  void _saveBriefing(Map<String, dynamic> briefing) =>
      _save({'briefing': briefing});

  @override
  Widget build(BuildContext context) {
    final c = widget.color;
    final briefing = _briefing;
    final on = briefing['morning'] == true;
    final time = '${briefing['morning_time'] ?? ''}'.isNotEmpty
        ? '${briefing['morning_time']}'
        : '09:00';
    final ready = _prefs != null && !_saving;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SetupPreferenceToggle(
          icon: Icons.wb_twilight_outlined,
          title: L10n.t('setup_briefing_morning_title'),
          desc: L10n.t('setup_briefing_morning_desc'),
          value: on,
          color: c,
          onChanged: ready
              ? (v) => _saveBriefing({
                  ...briefing,
                  'morning': v,
                  'morning_time': time,
                })
              : null,
        ),
        if (on) ...[
          const SizedBox(height: 12),
          SetupCard(
            color: c,
            child: DropdownButtonFormField<String>(
              key: ValueKey('setup_briefing_time_$time'),
              initialValue: time,
              isExpanded: true,
              decoration: InputDecoration(
                labelText: L10n.t('briefing_morning_time'),
              ),
              items: [
                for (final t in briefingTimeOptions(time))
                  DropdownMenuItem(value: t, child: Text(t)),
              ],
              onChanged: ready
                  ? (t) {
                      if (t != null) {
                        _saveBriefing({...briefing, 'morning_time': t});
                      }
                    }
                  : null,
            ),
          ),
          const SizedBox(height: 12),
          setupTimezoneCard(
            color: c,
            current: '${_prefs?['timezone'] ?? 'UTC'}',
            enabled: ready,
            onChanged: (zone) => _save({'timezone': zone}),
          ),
          const SizedBox(height: 12),
          SetupCard(
            color: c,
            // The list tiles paint on the nearest Material, which the card's
            // coloured box would otherwise hide.
            child: Material(
              type: MaterialType.transparency,
              child: BriefingContentEditor(
                settings: briefing,
                availability: BriefingChannelAvailability(
                  telegram:
                      ref
                          .watch(telegramStatusProvider)
                          .valueOrNull
                          ?.ownerLinked ??
                      false,
                  whatsapp:
                      ref.watch(whatsAppStatusProvider).valueOrNull?.loggedIn ??
                      false,
                  mailAccounts:
                      ref.watch(mailAccountsProvider).valueOrNull ?? const [],
                ),
                busy: !ready,
                onChanged: _saveBriefing,
              ),
            ),
          ),
        ],
        if (_error != null)
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Text(_error!, style: const TextStyle(color: MemoTheme.red)),
          ),
      ],
    );
  }
}
