import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../core/l10n.dart';
import '../../core/friendly_error.dart';
import '../../providers/chat_provider.dart';
import '../../providers/provider_provider.dart';
import '../../models/provider_config.dart';
import '../../models/provider_models.dart';

class DecisionModelSection extends ConsumerStatefulWidget {
  const DecisionModelSection({super.key});
  @override
  ConsumerState<DecisionModelSection> createState() =>
      _DecisionModelSectionState();
}

class _DecisionModelSectionState extends ConsumerState<DecisionModelSection> {
  final _provider = TextEditingController();
  final _model = TextEditingController();
  bool _loading = true,
      _saving = false,
      _decision = false,
      _verification = false;
  String? _error;
  int _threshold = 85, _maxRisk = 20;
  final Map<String, List<String>> _catalogs = {};
  bool _fetching = false;
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
      if (_provider.text.isNotEmpty) _refreshModels();
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
    if ((_decision || _verification) &&
        !_choices().containsKey(
          encodeModelSelection(_provider.text, _model.text),
        )) {
      setState(() => _error = L10n.t('decision_select_required'));
      return;
    }
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

  Future<void> _refreshModels() async {
    final name = _provider.text;
    if (name.isEmpty || _fetching) return;
    setState(() => _fetching = true);
    try {
      final list = await ref.read(apiClientProvider).listProviderModels(name);
      if (mounted) {
        setState(() => _catalogs[name] = list.models.map((m) => m.id).toList());
      }
    } catch (_) {
      /* Saved choices are still available. */
    } finally {
      if (mounted) setState(() => _fetching = false);
    }
  }

  Map<String, String> _choices() {
    final providers =
        ref.read(providerListProvider).valueOrNull ?? <ProviderConfig>[];
    return {
      for (final p in providers.where(
        (p) => p.enabled && !ProviderDefaults.isCLIType(p.type),
      ))
        for (final id in <String>{
          ...p.configuredModels.map((m) => m.id),
          ...?_catalogs[p.name],
          if (p.name == _provider.text && _model.text.isNotEmpty) _model.text,
        })
          encodeModelSelection(p.name, id): '${p.name} · $id',
    };
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const Center(child: CircularProgressIndicator());
    ref.watch(providerListProvider);
    final choices = _choices();
    final selected = encodeModelSelection(_provider.text, _model.text);
    final value = choices.containsKey(selected) ? selected : '';
    return Card(
      margin: const EdgeInsets.only(top: 24),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(Icons.fact_check_outlined, size: 20),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
              L10n.t('decision_tab_title'),
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                IconButton(
                  tooltip: L10n.t('refresh'),
                  onPressed: _fetching ? null : _refreshModels,
                  icon: _fetching
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.refresh, size: 18),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Text(L10n.t('verification_overview'),
              style: Theme.of(context).textTheme.bodySmall,
            ),
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
            ...[
              DropdownButtonFormField<String>(
                key: ValueKey('decision_model_$value'),
                initialValue: value,
                isExpanded: true,
                decoration: InputDecoration(
                  labelText: L10n.t('decision_model_label'),
                  border: const OutlineInputBorder(),
                ),
                items: [
                  DropdownMenuItem(
                    value: '',
                    child: Text(L10n.t('decision_select_model')),
                  ),
                  for (final entry in choices.entries)
                    DropdownMenuItem(
                      value: entry.key,
                      child: Text(entry.value, overflow: TextOverflow.ellipsis),
                    ),
                ],
                onChanged: _saving
                    ? null
                    : (v) => setState(() {
                        final choice = decodeModelSelection(v ?? '');
                        _provider.text = choice?.provider ?? '';
                        _model.text = choice?.model ?? '';
                        _refreshModels();
                      }),
              ),
              const SizedBox(height: 8),
              Text(L10n.t('decision_registered_hint')),
              if (choices.isEmpty) Text(L10n.t('decision_no_models')),
            ],
            if (_verification)
              ExpansionTile(
                tilePadding: EdgeInsets.zero,
                title: Text(L10n.t('verification_limits')),
                children: [
              const SizedBox(height: 8),
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
              ),
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
      ),
    );
  }
}
