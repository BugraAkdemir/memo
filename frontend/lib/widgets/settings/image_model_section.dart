import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/image_config.dart';
import '../../models/provider_config.dart';
import '../../models/provider_models.dart';
import '../../providers/chat_provider.dart';
import '../../providers/image_config_provider.dart';
import '../../providers/provider_provider.dart';

/// Settings card for picture generation: the switch for automatic routing and the
/// default image model API-key providers use. A Subscriptions account always draws
/// with its own account's image model, so it needs nothing here.
class ImageModelSection extends ConsumerStatefulWidget {
  const ImageModelSection({super.key});

  @override
  ConsumerState<ImageModelSection> createState() => _ImageModelSectionState();
}

class _ImageModelSectionState extends ConsumerState<ImageModelSection> {
  final _modelController = TextEditingController();
  TextEditingController? _boundField;
  String _provider = '';
  String _seededFrom = '';
  List<String> _suggestions = const [];
  bool _loadingModels = false;
  bool _saving = false;
  String? _message;
  bool _messageIsError = false;

  @override
  void initState() {
    super.initState();
    // The settings may already be loaded (cached) when this opens; otherwise the
    // listener in build() delivers them. Never seeded from build() itself: that
    // calls setState.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final cfg = ref.read(imageConfigProvider).valueOrNull;
      if (cfg != null) _seed(cfg.defaultProvider, cfg.defaultModel);
    });
  }

  @override
  void dispose() {
    _modelController.dispose();
    super.dispose();
  }

  /// Pulls the saved values into the form once per distinct saved value — not on
  /// every rebuild, which would overwrite what the person is typing.
  void _seed(String provider, String model) {
    final key = '$provider\u0000$model';
    if (_seededFrom == key) return;
    _seededFrom = key;
    setState(() {
      _provider = provider;
      _modelController.text = model;
    });
    if (provider.isNotEmpty) _loadModels(provider);
  }

  Future<void> _loadModels(String provider) async {
    setState(() => _loadingModels = true);
    List<String> ids = const [];
    try {
      final list = await ref.read(apiClientProvider).listProviderModels(provider);
      ids = list.models.map((ProviderModel m) => m.id).toList();
    } catch (_) {
      // No list is fine: the field still takes a typed model name.
    }
    if (!mounted || _provider != provider) return;
    setState(() {
      _suggestions = ids;
      _loadingModels = false;
    });
  }

  Future<void> _saveDefault() async {
    final model = _modelController.text.trim();
    setState(() {
      _saving = true;
      _message = null;
    });
    final err = await ref.read(imageConfigProvider.notifier).save(
          defaultProvider: _provider.isEmpty ? '' : _provider,
          defaultModel: _provider.isEmpty ? '' : model,
        );
    if (!mounted) return;
    setState(() {
      _saving = false;
      _messageIsError = err != null;
      _message = err ?? L10n.t('image_saved');
    });
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final cfgAsync = ref.watch(imageConfigProvider);
    final cfg = cfgAsync.valueOrNull;
    final providers = (ref.watch(providerListProvider).valueOrNull ?? const <ProviderConfig>[])
        .where((p) => p.enabled)
        .toList();
    ref.listen<AsyncValue<ImageConfig>>(imageConfigProvider, (_, next) {
      final c = next.valueOrNull;
      if (c != null) _seed(c.defaultProvider, c.defaultModel);
    });

    // A saved provider that is no longer configured must not break the dropdown.
    final names = providers.map((p) => p.name).toList();
    final dropdownValue = names.contains(_provider) ? _provider : '';

    return Card(
      margin: const EdgeInsets.only(top: 24),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              L10n.t('image_section_title'),
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                    color: theme.textMain,
                  ),
            ),
            const SizedBox(height: 6),
            Text(
              L10n.t('image_section_desc'),
              style: TextStyle(color: theme.textDim, fontSize: 13),
            ),
            const SizedBox(height: 12),
            SwitchListTile(
              key: const Key('image_auto_route_switch'),
              contentPadding: EdgeInsets.zero,
              value: cfg?.autoRoute ?? true,
              onChanged: cfg == null
                  ? null
                  : (v) async {
                      final err = await ref
                          .read(imageConfigProvider.notifier)
                          .save(autoRoute: v);
                      if (!mounted || err == null) return;
                      setState(() {
                        _messageIsError = true;
                        _message = err;
                      });
                    },
              title: Text(L10n.t('image_auto_route')),
              subtitle: Text(
                L10n.t('image_auto_route_hint'),
                style: TextStyle(color: theme.textDim, fontSize: 12),
              ),
            ),
            const Divider(),
            Text(
              L10n.t('image_default_title'),
              style: const TextStyle(fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: 4),
            Text(
              L10n.t('image_default_hint'),
              style: TextStyle(color: theme.textDim, fontSize: 12),
            ),
            const SizedBox(height: 12),
            DropdownButtonFormField<String>(
              // initialValue is read once; keyed so a value seeded from the saved
              // settings (arriving after the first build) is actually shown.
              key: ValueKey('image_default_provider_$dropdownValue'),
              initialValue: dropdownValue,
              isExpanded: true,
              decoration: InputDecoration(
                labelText: L10n.t('image_default_provider'),
                border: const OutlineInputBorder(),
                isDense: true,
              ),
              items: [
                DropdownMenuItem(value: '', child: Text(L10n.t('image_default_none'))),
                for (final n in names) DropdownMenuItem(value: n, child: Text(n)),
              ],
              onChanged: (v) {
                final next = v ?? '';
                setState(() {
                  _provider = next;
                  _suggestions = const [];
                  _modelController.clear();
                  _message = null;
                });
                if (next.isNotEmpty) _loadModels(next);
              },
            ),
            const SizedBox(height: 12),
            Autocomplete<String>(
              key: ValueKey('image_model_$_provider'),
              optionsBuilder: (v) {
                final q = v.text.trim().toLowerCase();
                return _suggestions.where((m) => q.isEmpty || m.toLowerCase().contains(q));
              },
              initialValue: TextEditingValue(text: _modelController.text),
              onSelected: (v) => _modelController.text = v,
              fieldViewBuilder: (context, controller, focus, onSubmit) {
                // Keep the typed value in our own controller, so Save reads it
                // whether it was picked from the list or typed.
                if (!identical(_boundField, controller)) {
                  _boundField = controller;
                  controller.addListener(() => _modelController.text = controller.text);
                }
                return TextField(
                  key: const Key('image_default_model'),
                  controller: controller,
                  focusNode: focus,
                  enabled: _provider.isNotEmpty,
                  decoration: InputDecoration(
                    labelText: L10n.t('image_default_model'),
                    hintText: L10n.t('image_default_model_hint'),
                    border: const OutlineInputBorder(),
                    isDense: true,
                    suffixIcon: _loadingModels
                        ? const Padding(
                            padding: EdgeInsets.all(10),
                            child: SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            ),
                          )
                        : null,
                  ),
                );
              },
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                FilledButton(
                  key: const Key('image_save'),
                  onPressed: _saving ? null : _saveDefault,
                  child: Text(L10n.t('image_save')),
                ),
                const SizedBox(width: 12),
                if (_message != null)
                  Expanded(
                    child: Text(
                      _message!,
                      style: TextStyle(
                        fontSize: 12,
                        color: _messageIsError ? Theme.of(context).colorScheme.error : theme.textDim,
                      ),
                    ),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
