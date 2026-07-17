import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../core/api_client.dart';
import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/connection_provider.dart' hide ConnectionState;
import '../providers/locale_provider.dart';

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return DefaultTabController(
      length: 4,
      child: Scaffold(
        backgroundColor: MemoTheme.bg,
        appBar: AppBar(
          title: Text(L10n.t('settings_title')),
          bottom: TabBar(
            isScrollable: true,
            indicatorColor: MemoTheme.accent,
            indicatorWeight: 2,
            indicatorSize: TabBarIndicatorSize.label,
            labelColor: MemoTheme.text,
            unselectedLabelColor: MemoTheme.textFaint,
            labelStyle: MemoTheme.body(14, w: FontWeight.w600),
            unselectedLabelStyle: MemoTheme.body(14),
            dividerColor: MemoTheme.border,
            tabs: [
              Tab(text: L10n.t('tab_connection')),
              Tab(text: L10n.t('tab_providers')),
              Tab(text: L10n.t('tab_models')),
              Tab(text: L10n.t('tab_learning')),
            ],
          ),
        ),
        body: const TabBarView(
          children: [_GeneralTab(), _ProvidersTab(), _ModelsTab(), _LearningTab()],
        ),
      ),
    );
  }
}

// ── Connection ───────────────────────────────────────────────────────
class _GeneralTab extends ConsumerStatefulWidget {
  const _GeneralTab();

  @override
  ConsumerState<_GeneralTab> createState() => _GeneralTabState();
}

class _GeneralTabState extends ConsumerState<_GeneralTab> {
  late final TextEditingController _urlCtrl;
  late final TextEditingController _tokenCtrl;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    _urlCtrl = TextEditingController();
    _tokenCtrl = TextEditingController();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final state = ref.read(connectionStateProvider);
      _urlCtrl.text = state.baseUrl;
      _tokenCtrl.text = state.token;
    });
  }

  @override
  void dispose() {
    _urlCtrl.dispose();
    _tokenCtrl.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final url = _urlCtrl.text.trim();
    if (url.isEmpty) return;
    setState(() => _saving = true);
    try {
      final normalized = url.replaceAll(RegExp(r'/+$'), '');
      final api = ref.read(apiClientProvider);
      api.updateBaseUrl(normalized);
      api.setToken(_tokenCtrl.text.trim());
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString('backend_url', normalized);
      await prefs.setString('backend_token', _tokenCtrl.text.trim());
      ref.read(connectionStateProvider.notifier).connect(
            normalized,
            token: _tokenCtrl.text.trim(),
            remote: Uri.parse(normalized).scheme == 'https',
          );
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(L10n.t('saved_reconnected'))));
      }
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(L10n.t('error_with', {'e': '$e'}))));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        _label(L10n.t('desktop_address')),
        const SizedBox(height: 8),
        TextField(
          controller: _urlCtrl,
          keyboardType: TextInputType.url,
          autocorrect: false,
          style: MemoTheme.mono(14, color: MemoTheme.text),
          decoration: const InputDecoration(prefixIcon: Icon(Icons.lan_outlined, size: 20)),
        ),
        const SizedBox(height: 18),
        _label(L10n.t('access_token')),
        const SizedBox(height: 8),
        TextField(
          controller: _tokenCtrl,
          autocorrect: false,
          style: MemoTheme.mono(14, color: MemoTheme.text),
          decoration: InputDecoration(
            prefixIcon: const Icon(Icons.key_outlined, size: 20),
            hintText: L10n.t('token_hint_remote'),
          ),
        ),
        const SizedBox(height: 24),
        SizedBox(
          height: 50,
          child: FilledButton(
            onPressed: _saving ? null : _save,
            style: FilledButton.styleFrom(
              backgroundColor: MemoTheme.accent,
              foregroundColor: MemoTheme.onAccent,
              disabledBackgroundColor: MemoTheme.surfaceHi,
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(13)),
            ),
            child: _saving
                ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.onAccent))
                : Text(L10n.t('save_reconnect'), style: MemoTheme.body(15, w: FontWeight.w700, color: MemoTheme.onAccent)),
          ),
        ),
        const SizedBox(height: 28),
        _label(L10n.t('language')),
        const SizedBox(height: 8),
        _LanguagePicker(),
      ],
    );
  }

  Widget _label(String s) => Text(s, style: MemoTheme.mono(11, color: MemoTheme.textFaint, ls: 1.2));
}

class _LanguagePicker extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final locale = ref.watch(localeProvider);
    return Row(
      children: [
        Expanded(
          child: _langChip(
            label: L10n.t('language_tr'),
            selected: locale == MemoLocale.tr,
            onTap: () => ref.read(localeProvider.notifier).setLocale(MemoLocale.tr),
          ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: _langChip(
            label: L10n.t('language_en'),
            selected: locale == MemoLocale.en,
            onTap: () => ref.read(localeProvider.notifier).setLocale(MemoLocale.en),
          ),
        ),
      ],
    );
  }

  Widget _langChip({required String label, required bool selected, required VoidCallback onTap}) {
    return GestureDetector(
      onTap: () {
        HapticFeedback.selectionClick();
        onTap();
      },
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 14),
        decoration: BoxDecoration(
          color: selected ? MemoTheme.accent.withValues(alpha: 0.16) : MemoTheme.surface,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: selected ? MemoTheme.accent : MemoTheme.border),
        ),
        alignment: Alignment.center,
        child: Text(
          label,
          style: MemoTheme.body(
            14.5,
            w: FontWeight.w600,
            color: selected ? MemoTheme.accentBright : MemoTheme.textDim,
          ),
        ),
      ),
    );
  }
}

// ── Providers ────────────────────────────────────────────────────────
class _ProvidersTab extends ConsumerStatefulWidget {
  const _ProvidersTab();

  @override
  ConsumerState<_ProvidersTab> createState() => _ProvidersTabState();
}

class _ProvidersTabState extends ConsumerState<_ProvidersTab> {
  late Future<List<ProviderConfig>> _future;

  @override
  void initState() {
    super.initState();
    _future = ref.read(apiClientProvider).getProviders();
  }

  void _reload() => setState(() => _future = ref.read(apiClientProvider).getProviders());

  Future<void> _toggle(ProviderConfig p) async {
    try {
      await ref.read(apiClientProvider).updateProvider(p.copyWith(enabled: !p.enabled));
      _reload();
    } catch (_) {}
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<List<ProviderConfig>>(
      future: _future,
      builder: (context, snap) {
        if (snap.connectionState == ConnectionState.waiting) {
          return const Center(child: CircularProgressIndicator(color: MemoTheme.accent));
        }
        if (snap.hasError) return _errorView('${snap.error}');
        final providers = snap.data ?? [];
        if (providers.isEmpty) {
          return _emptyView(L10n.t('no_providers'), L10n.t('no_providers_hint'));
        }
        return ListView.builder(
          padding: const EdgeInsets.all(16),
          itemCount: providers.length,
          itemBuilder: (context, i) {
            final p = providers[i];
            return _card(
              child: Row(
                children: [
                  _iconTile(Icons.cloud_outlined),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(p.name, style: MemoTheme.body(15, w: FontWeight.w600)),
                        const SizedBox(height: 3),
                        Text(p.model.isEmpty ? p.type : p.model, style: MemoTheme.mono(11.5, color: MemoTheme.textFaint)),
                      ],
                    ),
                  ),
                  _togglePill(p.enabled, () => _toggle(p)),
                ],
              ),
            );
          },
        );
      },
    );
  }

  Widget _togglePill(bool on, VoidCallback onTap) {
    return GestureDetector(
      onTap: () {
        HapticFeedback.selectionClick();
        onTap();
      },
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 6),
        decoration: BoxDecoration(
          color: on ? MemoTheme.sage.withValues(alpha: 0.16) : MemoTheme.surfaceHi,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: on ? MemoTheme.sage.withValues(alpha: 0.4) : MemoTheme.border),
        ),
        child: Text(on ? L10n.t('on') : L10n.t('off'),
            style: MemoTheme.mono(11, w: FontWeight.w600, color: on ? MemoTheme.sage : MemoTheme.textFaint)),
      ),
    );
  }
}

// ── Models ───────────────────────────────────────────────────────────
class _ModelsTab extends ConsumerStatefulWidget {
  const _ModelsTab();

  @override
  ConsumerState<_ModelsTab> createState() => _ModelsTabState();
}

class _ModelsTabState extends ConsumerState<_ModelsTab> {
  late Future<List<LocalModel>> _future;
  bool _stopping = false;

  @override
  void initState() {
    super.initState();
    _future = ref.read(apiClientProvider).listLocalModels();
  }

  Future<void> _stop() async {
    setState(() => _stopping = true);
    try {
      await ref.read(apiClientProvider).stopModel();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(L10n.t('model_stopped'))));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(L10n.t('error_with', {'e': '$e'}))));
    } finally {
      if (mounted) setState(() => _stopping = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<List<LocalModel>>(
      future: _future,
      builder: (context, snap) {
        if (snap.connectionState == ConnectionState.waiting) {
          return const Center(child: CircularProgressIndicator(color: MemoTheme.accent));
        }
        if (snap.hasError) return _errorView('${snap.error}');
        final models = snap.data ?? [];
        return Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 14, 16, 6),
              child: SizedBox(
                width: double.infinity,
                child: OutlinedButton.icon(
                  onPressed: _stopping ? null : _stop,
                  icon: _stopping
                      ? const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.error))
                      : const Icon(Icons.stop_circle_outlined, size: 18),
                  label: Text(L10n.t('stop_running_model')),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: MemoTheme.error,
                    side: BorderSide(color: MemoTheme.error.withValues(alpha: 0.4)),
                    padding: const EdgeInsets.symmetric(vertical: 13),
                    textStyle: MemoTheme.body(14, w: FontWeight.w600),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
                  ),
                ),
              ),
            ),
            Expanded(
              child: models.isEmpty
                  ? _emptyView(L10n.t('no_local_models'), L10n.t('no_local_models_hint'))
                  : ListView.builder(
                      padding: const EdgeInsets.all(16),
                      itemCount: models.length,
                      itemBuilder: (context, i) => _ModelCard(m: models[i]),
                    ),
            ),
          ],
        );
      },
    );
  }
}

class _ModelCard extends ConsumerStatefulWidget {
  final LocalModel m;
  const _ModelCard({required this.m});

  @override
  ConsumerState<_ModelCard> createState() => _ModelCardState();
}

class _ModelCardState extends ConsumerState<_ModelCard> {
  bool _starting = false;

  Future<void> _start() async {
    setState(() => _starting = true);
    try {
      final api = ref.read(apiClientProvider);
      if (widget.m.isEmbedding) {
        await api.startEmbeddingModel(path: widget.m.path);
      } else {
        await api.startModel(path: widget.m.path);
      }
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(L10n.t('model_started', {'name': widget.m.filename}))));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(L10n.t('error_with', {'e': '$e'}))));
    } finally {
      if (mounted) setState(() => _starting = false);
    }
  }

  String _size(int b) {
    if (b < 1024 * 1024) {
      return L10n.t('size_kb', {'n': (b / 1024).toStringAsFixed(0)});
    }
    if (b < 1024 * 1024 * 1024) {
      return L10n.t('size_mb', {'n': (b / (1024 * 1024)).toStringAsFixed(0)});
    }
    return L10n.t('size_gb', {'n': (b / (1024 * 1024 * 1024)).toStringAsFixed(1)});
  }

  @override
  Widget build(BuildContext context) {
    return _card(
      child: Row(
        children: [
          _iconTile(widget.m.isEmbedding ? Icons.auto_awesome_outlined : Icons.developer_board_rounded),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(widget.m.filename,
                    maxLines: 1, overflow: TextOverflow.ellipsis, style: MemoTheme.body(14.5, w: FontWeight.w600)),
                const SizedBox(height: 3),
                Text('${_size(widget.m.sizeBytes)}${widget.m.isEmbedding ? L10n.t('embedding_suffix') : ""}',
                    style: MemoTheme.mono(11.5, color: MemoTheme.textFaint)),
              ],
            ),
          ),
          const SizedBox(width: 10),
          SizedBox(
            height: 38,
            child: FilledButton(
              onPressed: _starting ? null : _start,
              style: FilledButton.styleFrom(
                backgroundColor: MemoTheme.accent.withValues(alpha: 0.16),
                foregroundColor: MemoTheme.accentBright,
                padding: const EdgeInsets.symmetric(horizontal: 18),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
              ),
              child: _starting
                  ? const SizedBox(width: 15, height: 15, child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.accent))
                  : Text(L10n.t('start'), style: MemoTheme.body(13.5, w: FontWeight.w600, color: MemoTheme.accentBright)),
            ),
          ),
        ],
      ),
    );
  }
}

// ── Shared bits ──────────────────────────────────────────────────────
Widget _card({required Widget child}) {
  return Container(
    margin: const EdgeInsets.only(bottom: 10),
    padding: const EdgeInsets.all(14),
    decoration: BoxDecoration(
      color: MemoTheme.surface,
      borderRadius: BorderRadius.circular(14),
      border: Border.all(color: MemoTheme.border),
    ),
    child: child,
  );
}

Widget _iconTile(IconData icon) {
  return Container(
    width: 42,
    height: 42,
    decoration: BoxDecoration(
      color: MemoTheme.accent.withValues(alpha: 0.12),
      borderRadius: BorderRadius.circular(11),
    ),
    child: Icon(icon, color: MemoTheme.accent, size: 21),
  );
}

Widget _errorView(String msg) {
  return Center(
    child: Padding(
      padding: const EdgeInsets.all(32),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.cloud_off_rounded, size: 34, color: MemoTheme.textFaint),
          const SizedBox(height: 12),
          Text(L10n.t('couldnt_load'), style: MemoTheme.body(15, w: FontWeight.w600)),
          const SizedBox(height: 6),
          Text(msg, textAlign: TextAlign.center, style: MemoTheme.body(13, color: MemoTheme.textFaint), maxLines: 4, overflow: TextOverflow.ellipsis),
        ],
      ),
    ),
  );
}

Widget _emptyView(String title, String sub) {
  return Center(
    child: Padding(
      padding: const EdgeInsets.all(32),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(title, style: MemoTheme.body(15, w: FontWeight.w600, color: MemoTheme.textDim)),
          const SizedBox(height: 6),
          Text(sub, textAlign: TextAlign.center, style: MemoTheme.body(13, color: MemoTheme.textFaint)),
        ],
      ),
    ),
  );
}

// ── Learning ─────────────────────────────────────────────────────────
class _LearningTab extends ConsumerStatefulWidget {
  const _LearningTab();

  @override
  ConsumerState<_LearningTab> createState() => _LearningTabState();
}

class _LearningTabState extends ConsumerState<_LearningTab> {
  bool _loading = true;
  bool _saving = false;
  String? _error;

  bool _singleModel = false;
  final _modelCtrl = TextEditingController();
  int _reminderLead = 30;
  bool _guessTime = true;

  static const _leadOptions = [10, 15, 30, 60, 120];

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _modelCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final client = ref.read(apiClientProvider);
      final learning = await client.getLearningSettings();
      final calendar = await client.getCalendarSettings();
      if (!mounted) return;
      setState(() {
        _singleModel = learning['single_model_enabled'] as bool? ?? false;
        _modelCtrl.text = learning['model_id'] as String? ?? '';
        final lead = calendar['reminder_lead_minutes'] as int? ?? 30;
        _reminderLead = _leadOptions.contains(lead) ? lead : 30;
        _guessTime = !(calendar['disable_time_guess'] as bool? ?? false);
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = '$e';
        _loading = false;
      });
    }
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    try {
      final client = ref.read(apiClientProvider);
      await client.updateLearningSettings(
        singleModelEnabled: _singleModel,
        modelId: _modelCtrl.text.trim(),
      );
      await client.updateCalendarSettings(
        reminderLeadMinutes: _reminderLead,
        disableTimeGuess: !_guessTime,
      );
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(L10n.t('learning_saved'))));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(L10n.t('error_with', {'e': '$e'}))));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator(color: MemoTheme.accent));
    }
    if (_error != null) return _errorView(_error!);

    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        Text(L10n.t('single_model_mode'),
            style: MemoTheme.mono(11, color: MemoTheme.textFaint, ls: 1.2)),
        const SizedBox(height: 8),
        _card(
          child: Column(
            children: [
              Row(
                children: [
                  _iconTile(Icons.psychology_outlined),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(L10n.t('use_one_model'),
                            style: MemoTheme.body(15, w: FontWeight.w600)),
                        const SizedBox(height: 3),
                        Text(L10n.t('use_one_model_hint'),
                            style: MemoTheme.mono(11.5, color: MemoTheme.textFaint)),
                      ],
                    ),
                  ),
                  Switch(
                    value: _singleModel,
                    activeColor: MemoTheme.accent,
                    onChanged: (v) => setState(() => _singleModel = v),
                  ),
                ],
              ),
              if (_singleModel) ...[
                const SizedBox(height: 12),
                TextField(
                  controller: _modelCtrl,
                  autocorrect: false,
                  style: MemoTheme.mono(14, color: MemoTheme.text),
                  decoration: InputDecoration(
                    prefixIcon: const Icon(Icons.smart_toy_outlined, size: 20),
                    hintText: L10n.t('model_id_hint'),
                  ),
                ),
              ],
            ],
          ),
        ),
        const SizedBox(height: 18),
        Text(L10n.t('calendar_reminder_section'),
            style: MemoTheme.mono(11, color: MemoTheme.textFaint, ls: 1.2)),
        const SizedBox(height: 8),
        _card(
          child: Row(
            children: [
              _iconTile(Icons.notifications_outlined),
              const SizedBox(width: 14),
              Expanded(
                child: Text(L10n.t('notify_before_event'),
                    style: MemoTheme.body(15, w: FontWeight.w600)),
              ),
              DropdownButton<int>(
                value: _reminderLead,
                dropdownColor: MemoTheme.surface,
                underline: const SizedBox.shrink(),
                style: MemoTheme.body(14, color: MemoTheme.text),
                items: _leadOptions
                    .map((m) => DropdownMenuItem(
                          value: m,
                          child: Text(m < 60
                              ? L10n.t('minutes_short', {'n': '$m'})
                              : L10n.t('hours_short', {'n': '${m ~/ 60}'})),
                        ))
                    .toList(),
                onChanged: (v) => setState(() => _reminderLead = v ?? 30),
              ),
            ],
          ),
        ),
        const SizedBox(height: 18),
        Text(L10n.t('uncertain_times'),
            style: MemoTheme.mono(11, color: MemoTheme.textFaint, ls: 1.2)),
        const SizedBox(height: 8),
        _card(
          child: Row(
            children: [
              _iconTile(Icons.schedule_outlined),
              const SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(L10n.t('guess_time'),
                        style: MemoTheme.body(15, w: FontWeight.w600)),
                    const SizedBox(height: 3),
                    Text(L10n.t('guess_time_hint'),
                        style: MemoTheme.mono(11.5, color: MemoTheme.textFaint)),
                  ],
                ),
              ),
              Switch(
                value: _guessTime,
                activeColor: MemoTheme.accent,
                onChanged: (v) => setState(() => _guessTime = v),
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),
        SizedBox(
          height: 50,
          child: FilledButton(
            onPressed: _saving ? null : _save,
            style: FilledButton.styleFrom(
              backgroundColor: MemoTheme.accent,
              foregroundColor: MemoTheme.onAccent,
              disabledBackgroundColor: MemoTheme.surfaceHi,
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(13)),
            ),
            child: _saving
                ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.onAccent))
                : Text(L10n.t('save'), style: MemoTheme.body(15, w: FontWeight.w700, color: MemoTheme.onAccent)),
          ),
        ),
      ],
    );
  }
}
