import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../models/provider_config.dart';
import '../providers/chat_provider.dart';
import '../providers/provider_provider.dart';

/// The mail reply-writer selector: pick a provider, pick one of its models and
/// save. Shared by the Settings tab and the setup wizard so both read and write
/// the same twin reply model (`/api/twin/reply_model`). The reply writer is
/// chosen independently of the scalar decision model.
class MailReplyModelPicker extends ConsumerStatefulWidget {
  /// Called after a successful save with the saved provider and model (empty
  /// strings when the selection was cleared).
  final void Function(String provider, String model)? onSaved;

  /// Extra buttons appended after Refresh and Save.
  final List<Widget> extraActions;

  /// Tells the owner what was saved with a snackbar. The wizard shows its own
  /// status instead.
  final bool showSavedSnackBar;

  /// Where the selection lives. By default it is the twin reply model; the
  /// briefing model reuses this picker with its own load and save.
  final Future<({String provider, String model})> Function()? loadSelection;
  final Future<void> Function(String provider, String model)? saveSelection;

  /// Label keys and the save button's key, so a second picker on the same screen
  /// does not read "Reply model".
  final String providerLabelKey;
  final String modelLabelKey;
  final Key saveKey;

  const MailReplyModelPicker({
    super.key,
    this.onSaved,
    this.extraActions = const [],
    this.showSavedSnackBar = true,
    this.loadSelection,
    this.saveSelection,
    this.providerLabelKey = 'mail_reply_provider',
    this.modelLabelKey = 'mail_reply_model',
    this.saveKey = const Key('reply_model_save'),
  });

  @override
  ConsumerState<MailReplyModelPicker> createState() =>
      _MailReplyModelPickerState();
}

class _MailReplyModelPickerState extends ConsumerState<MailReplyModelPicker> {
  bool _loading = true, _saving = false, _fetching = false;
  String _provider = '', _model = '';
  String? _error;
  List<String> _catalog = [];
  int _request = 0;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final load = widget.loadSelection;
      final chosen = load != null
          ? await load()
          : await ref.read(apiClientProvider).getTwinState().then((state) {
              final m = state.replyModel;
              return (
                provider: m['provider'] is String
                    ? m['provider'] as String
                    : '',
                model: m['model'] is String ? m['model'] as String : '',
              );
            });
      if (!mounted) return;
      setState(() {
        _provider = chosen.provider;
        _model = chosen.model;
        _loading = false;
      });
      if (_provider.isNotEmpty) await _refreshModels();
    } catch (_) {
      if (mounted) {
        setState(() {
          _loading = false;
          _error = L10n.t('twin_failed');
        });
      }
    }
  }

  Future<void> _refreshModels() async {
    final name = _provider;
    final request = ++_request;
    if (name.isEmpty) return;
    setState(() {
      _fetching = true;
      _error = null;
    });
    try {
      final result = await ref.read(apiClientProvider).listProviderModels(name);
      if (mounted && request == _request) {
        setState(() {
          _catalog = result.models.map((m) => m.id).toList();
          if (result.error.isNotEmpty) {
            _error = L10n.t('mail_reply_catalog_failed');
          }
        });
      }
    } catch (_) {
      if (mounted && request == _request) {
        setState(() => _error = L10n.t('mail_reply_catalog_failed'));
      }
    } finally {
      if (mounted && request == _request) setState(() => _fetching = false);
    }
  }

  Future<void> _save() async {
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      final save = widget.saveSelection;
      if (save != null) {
        await save(_provider, _model);
      } else {
        await ref.read(apiClientProvider).callTwin('reply_model', {
          'provider': _provider,
          'model': _model,
        });
      }
      if (mounted) {
        if (widget.showSavedSnackBar) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text(L10n.t('mail_reply_saved'))));
        }
        widget.onSaved?.call(_provider, _model);
      }
    } catch (_) {
      if (mounted) setState(() => _error = L10n.t('mail_reply_save_failed'));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final providers =
        (ref.watch(providerListProvider).valueOrNull ?? <ProviderConfig>[])
            .where(
              (p) =>
                  p.enabled &&
                  !ProviderDefaults.isCLIType(p.type) &&
                  !p.type.startsWith('web-'),
            )
            .toList();
    final selected = providers.where((p) => p.name == _provider).firstOrNull;
    final models = <String>{
      ..._catalog.where((m) => m.isNotEmpty),
      ...?selected?.configuredModels
          .map((m) => m.id)
          .where((m) => m.isNotEmpty),
      if (_model.isNotEmpty && selected != null) _model,
    }.toList();
    if (_loading) return const Center(child: CircularProgressIndicator());
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        DropdownButtonFormField<String>(
          key: ValueKey('reply_provider_$_provider'),
          initialValue: selected == null ? '' : _provider,
          isExpanded: true,
          decoration: InputDecoration(
            labelText: L10n.t(widget.providerLabelKey),
          ),
          items: [
            DropdownMenuItem(value: '', child: Text(L10n.t('mail_reply_none'))),
            for (final p in providers)
              DropdownMenuItem(value: p.name, child: Text(p.name)),
          ],
          onChanged: _saving
              ? null
              : (v) {
                  setState(() {
                    _request++;
                    _fetching = false;
                    _provider = v ?? '';
                    _model = '';
                    _catalog = [];
                  });
                  if (_provider.isNotEmpty) _refreshModels();
                },
        ),
        const SizedBox(height: 12),
        DropdownButtonFormField<String>(
          key: ValueKey('reply_model_${_provider}_$_model'),
          initialValue: models.contains(_model) ? _model : '',
          isExpanded: true,
          decoration: InputDecoration(labelText: L10n.t(widget.modelLabelKey)),
          items: [
            DropdownMenuItem(value: '', child: Text(L10n.t('mail_reply_none'))),
            for (final m in models)
              DropdownMenuItem(
                value: m,
                child: Text(m, overflow: TextOverflow.ellipsis),
              ),
          ],
          onChanged: _saving || selected == null
              ? null
              : (v) => setState(() => _model = v ?? ''),
        ),
        if (_fetching) const LinearProgressIndicator(),
        if (_error != null)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 12),
            child: Text(
              _error!,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ),
        const SizedBox(height: 16),
        Wrap(
          spacing: 12,
          runSpacing: 8,
          children: [
            OutlinedButton(
              onPressed: _saving || _fetching || selected == null
                  ? null
                  : _refreshModels,
              child: Text(L10n.t('refresh')),
            ),
            FilledButton(
              key: widget.saveKey,
              onPressed:
                  _saving ||
                      (_provider.isNotEmpty &&
                          (_model.isEmpty || selected == null))
                  ? null
                  : _save,
              child: Text(L10n.t('save')),
            ),
            ...widget.extraActions,
          ],
        ),
      ],
    );
  }
}
