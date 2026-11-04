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
    if (prefs == null || _savingDrafts) return;
    setState(() {
      _savingDrafts = true;
      _error = null;
    });
    final next = Map<String, dynamic>.of(prefs);
    final autonomy = Map<String, dynamic>.of(
      (prefs['autonomy'] as Map?)?.cast<String, dynamic>() ?? const {},
    );
    autonomy['enabled'] = on;
    autonomy['mail'] = on;
    // Always the safe mode: a draft is prepared, never sent on its own.
    autonomy['mode'] = 'draft_only';
    next['autonomy'] = autonomy;
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
        if (_error != null)
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Text(_error!, style: const TextStyle(color: MemoTheme.red)),
          ),
      ],
    );
  }
}
