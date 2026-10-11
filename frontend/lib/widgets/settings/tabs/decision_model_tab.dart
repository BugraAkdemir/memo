import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/l10n.dart';
import '../../../core/friendly_error.dart';
import '../../../providers/chat_provider.dart';

class DecisionModelTab extends ConsumerStatefulWidget {
  const DecisionModelTab({super.key});
  @override
  ConsumerState<DecisionModelTab> createState() => _DecisionModelTabState();
}

class _DecisionModelTabState extends ConsumerState<DecisionModelTab> {
  final _provider = TextEditingController();
  final _model = TextEditingController();
  bool _loading = true,
      _saving = false,
      _decision = false,
      _verification = false;
  String? _error;
  int _threshold = 85, _maxRisk = 20;
  bool _legacySingle = false;
  String _legacyModel = '';
  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _provider.dispose();
    _model.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final data = await ref.read(apiClientProvider).getLearningSettings();
      if (!mounted) return;
      setState(() {
        _decision = data['decision_enabled'] == true;
        _verification = data['verification_enabled'] == true;
        _provider.text = data['decision_provider'] is String
            ? data['decision_provider'] as String
            : '';
        _model.text = data['decision_model'] is String
            ? data['decision_model'] as String
            : '';
        final threshold = data['verification_threshold'];
        final risk = data['verification_max_risk'];
        _threshold = threshold is int && threshold > 0 && threshold <= 100
            ? threshold
            : 85;
        _maxRisk = risk is int && risk > 0 && risk <= 100 ? risk : 20;
        _legacySingle = data['single_model_enabled'] == true;
        _legacyModel = data['model_id'] is String
            ? data['model_id'] as String
            : '';
        _loading = false;
      });
    } catch (e) {
      if (mounted) {
        setState(() {
          _loading = false;
          _error = FriendlyError.describeGeneric(e);
        });
      }
    }
  }

  Future<void> _save() async {
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await ref
          .read(apiClientProvider)
          .updateLearningSettings(
            _legacySingle,
            _legacyModel,
            decisionEnabled: _decision,
            decisionProvider: _provider.text.trim(),
            decisionModel: _model.text.trim(),
            verificationEnabled: _verification,
            verificationThreshold: _threshold,
            verificationMaxRisk: _maxRisk,
          );
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('learning_settings_saved'))),
        );
      }
    } catch (e) {
      if (mounted) setState(() => _error = FriendlyError.describeGeneric(e));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const Center(child: CircularProgressIndicator());
    return Material(
      color: Colors.transparent,
      child: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          Text(
            L10n.t('decision_tab_title'),
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 12),
          Text(L10n.t('verification_overview')),
          const SizedBox(height: 12),
          SwitchListTile(
            contentPadding: EdgeInsets.zero,
            title: Text(L10n.t('decision_model_title')),
            subtitle: Text(L10n.t('decision_model_desc')),
            value: _decision,
            onChanged: _saving
                ? null
                : (value) => setState(() => _decision = value),
          ),
          SwitchListTile(
            contentPadding: EdgeInsets.zero,
            title: Text(L10n.t('verification_enabled_title')),
            subtitle: Text(L10n.t('verification_enabled_desc')),
            value: _verification,
            onChanged: _saving
                ? null
                : (value) => setState(() => _verification = value),
          ),
          if (_decision || _verification) ...[
            TextField(
              controller: _provider,
              enabled: !_saving,
              decoration: InputDecoration(
                labelText: L10n.t('decision_provider_label'),
                helperText: L10n.t('decision_provider_hint'),
                helperMaxLines: 3,
              ),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _model,
              enabled: !_saving,
              decoration: InputDecoration(
                labelText: L10n.t('decision_model_label'),
              ),
            ),
          ],
          if (_verification) ...[
            const SizedBox(height: 20),
            Text(L10n.t('verification_threshold', {'n': '$_threshold'})),
            Slider(
              value: _threshold.toDouble(),
              min: 1,
              max: 100,
              divisions: 99,
              label: '$_threshold%',
              onChanged: _saving
                  ? null
                  : (value) => setState(() => _threshold = value.round()),
            ),
            Text(L10n.t('verification_max_risk', {'n': '$_maxRisk'})),
            Slider(
              value: _maxRisk.toDouble(),
              min: 1,
              max: 100,
              divisions: 99,
              label: '$_maxRisk%',
              onChanged: _saving
                  ? null
                  : (value) => setState(() => _maxRisk = value.round()),
            ),
            Text(L10n.t('verification_fallback_desc')),
          ],
          if (_error != null)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 12),
              child: Text(L10n.t('learning_error', {'e': _error!})),
            ),
          const SizedBox(height: 16),
          Align(
            alignment: Alignment.centerRight,
            child: TextButton(
              onPressed: _saving ? null : _save,
              child: _saving
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(L10n.t('save')),
            ),
          ),
        ],
      ),
    );
  }
}
