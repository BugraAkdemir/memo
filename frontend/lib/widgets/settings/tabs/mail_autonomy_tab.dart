import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/l10n.dart';
import '../../../models/provider_config.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/provider_provider.dart';
import '../../twin_dialog.dart';

/// The reply writer is selected independently of the scalar decision model.
class MailAutonomyTab extends ConsumerStatefulWidget {
  const MailAutonomyTab({super.key});
  @override
  ConsumerState<MailAutonomyTab> createState() => _MailAutonomyTabState();
}

class _MailAutonomyTabState extends ConsumerState<MailAutonomyTab> {
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
      final state = await ref.read(apiClientProvider).getTwinState();
      if (!mounted) return;
      setState(() {
        _provider = state.replyModel['provider'] is String
            ? state.replyModel['provider'] as String
            : '';
        _model = state.replyModel['model'] is String
            ? state.replyModel['model'] as String
            : '';
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
      await ref.read(apiClientProvider).callTwin('reply_model', {
        'provider': _provider,
        'model': _model,
      });
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(L10n.t('mail_reply_saved'))));
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
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        Text(
          L10n.t('mail_reply_title'),
          style: Theme.of(context).textTheme.titleLarge,
        ),
        const SizedBox(height: 12),
        Text(L10n.t('mail_reply_help')),
        const SizedBox(height: 16),
        DropdownButtonFormField<String>(
          key: ValueKey('reply_provider_$_provider'),
          initialValue: selected == null ? '' : _provider,
          isExpanded: true,
          decoration: InputDecoration(labelText: L10n.t('mail_reply_provider')),
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
          decoration: InputDecoration(labelText: L10n.t('mail_reply_model')),
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
              onPressed:
                  _saving ||
                      (_provider.isNotEmpty &&
                          (_model.isEmpty || selected == null))
                  ? null
                  : _save,
              child: Text(L10n.t('save')),
            ),
            OutlinedButton.icon(
              onPressed: () => showDialog<void>(
                context: context,
                builder: (_) => const TwinDialog(initialTab: 3),
              ),
              icon: const Icon(Icons.auto_awesome_outlined, size: 18),
              label: Text(L10n.t('mail_open_twin_panel')),
            ),
          ],
        ),
      ],
    );
  }
}
