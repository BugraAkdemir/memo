import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/friendly_error.dart';
import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/curated_models.dart';
import '../models/gpu_info.dart';
import '../models/local_model.dart';
import '../models/persona_presets.dart';
import '../providers/chat_provider.dart';
import '../providers/learning_provider.dart';
import '../providers/models_provider.dart';
import '../providers/provider_provider.dart';
import '../providers/settings_provider.dart';
import 'persona_picker.dart';
import 'provider_config_dialog.dart';

class SetupWizardOverlay extends ConsumerWidget {
  const SetupWizardOverlay({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isSetupComplete = ref.watch(setupCompleteProvider);

    if (isSetupComplete) {
      return SizedBox.shrink();
    }

    return _SetupWizardScreen();
  }
}

class _SetupWizardScreen extends ConsumerStatefulWidget {
  const _SetupWizardScreen();

  @override
  ConsumerState<_SetupWizardScreen> createState() => _SetupWizardScreenState();
}

class _SetupWizardScreenState extends ConsumerState<_SetupWizardScreen> {
  final _nameController = TextEditingController();
  final _customPromptController = TextEditingController();

  String _selectedTheme = 'light';
  String _selectedPrompt = personaPresets.first.key;
  bool _customPrompt = false;

  bool _checking = false;
  bool _backendOk = false;
  bool _modelsOk = false;
  int _localModelCount = 0;
  bool _saving = false;

  bool _providerConfigured = false;
  String? _activeProviderName;
  bool _hasEmbeddingModel = false;

  bool _modelDownloadStarted = false;
  bool _modelDownloadDone = false;
  String? _modelDownloadError;

  bool _memoryModelDownloading = false;
  bool _memoryModelDownloadDone = false;
  String? _memoryModelDownloadError;

  ThemeColors get _colors {
    if (_selectedTheme == 'dark') return MemoTheme.dark;
    if (_selectedTheme == 'light') return MemoTheme.light;
    return MemoTheme.of(context);
  }

  Brightness get _brightness {
    if (_selectedTheme == 'dark') return Brightness.dark;
    if (_selectedTheme == 'light') return Brightness.light;
    return Theme.of(context).brightness;
  }

  String _promptFor(String key) {
    for (final p in personaPresets) {
      if (p.key == key) return p.prompt;
    }
    return personaPresets.first.prompt;
  }

  String get _finalPrompt {
    if (_customPrompt) {
      final text = _customPromptController.text.trim();
      return composePersonaPrompt(
        _nameController.text,
        text.isEmpty ? _promptFor(personaPresets.first.key) : text,
      );
    }
    return composePersonaPrompt(_nameController.text, _promptFor(_selectedPrompt));
  }

  @override
  void initState() {
    super.initState();
    _checkDiagnostics();
  }

  @override
  void dispose() {
    _nameController.dispose();
    _customPromptController.dispose();
    super.dispose();
  }

  Future<void> _checkDiagnostics() async {
    setState(() => _checking = true);
    try {
      final isAlive = await ref.read(apiClientProvider).isAlive();
      _backendOk = isAlive;
      if (isAlive) {
        final models = await ref.read(apiClientProvider).listLocalModels();
        _modelsOk = models.isNotEmpty;
        _localModelCount = models.length;
        _hasEmbeddingModel = models.any((m) => m.isEmbedding);
        final activeProvider = await ref.read(apiClientProvider).getActiveProvider();
        _providerConfigured = activeProvider.isNotEmpty;
        _activeProviderName = activeProvider.isEmpty ? null : activeProvider;
      }
    } catch (_) {
      _backendOk = false;
      _modelsOk = false;
    }
    if (mounted) setState(() => _checking = false);
  }

  /// Lets the user skip the local-model download entirely and connect an API
  /// provider instead — the wizard used to only ever offer the local-model
  /// path, forcing a multi-GB download even for someone who just wants to
  /// plug in an OpenAI/Gemini/Claude key. Reuses the same dialog Settings →
  /// Providers uses to add one, then activates whichever provider it created
  /// (diffed against the pre-dialog list, since the dialog only returns a
  /// bool) so the wizard's own "ready" state reflects it immediately.
  Future<void> _connectProvider() async {
    Set<String> before = {};
    try {
      before = (await ref.read(providerListProvider.future)).map((p) => p.name).toSet();
    } catch (_) {
      // Fall through — the diff below then treats every provider as "new",
      // which just means we (re-)activate whichever one comes back first.
    }
    if (!mounted) return;

    final added = await showDialog<bool>(
      context: context,
      builder: (_) => const ProviderConfigDialog(),
    );
    if (added != true || !mounted) return;

    ref.invalidate(providerListProvider);
    try {
      final after = await ref.read(providerListProvider.future);
      final fresh = after.where((p) => !before.contains(p.name));
      if (fresh.isNotEmpty) {
        final name = fresh.first.name;
        await ref.read(apiClientProvider).setActiveProvider(name);
        ref.read(activeProviderTypeProvider.notifier).setActive(name);
        if (mounted) {
          setState(() {
            _providerConfigured = true;
            _activeProviderName = name;
          });
        }
      }
    } catch (_) {
      // Provider was saved either way (updateProvider already succeeded
      // inside the dialog) — activation just didn't confirm; the user can
      // still pick it from the chat top bar's model switcher afterwards.
    }
  }

  /// An API provider covers chat, but memory/RAG only ever runs against the
  /// local embedding model — connecting a provider alone silently leaves
  /// memory permanently off with nothing telling the user why. Offered right
  /// next to the provider button so it's a one-click follow-up instead of a
  /// surprise discovered later.
  Future<void> _downloadMemoryModel() async {
    setState(() {
      _memoryModelDownloading = true;
      _memoryModelDownloadDone = false;
      _memoryModelDownloadError = null;
    });
    try {
      await _downloadCurated(recommendedMemoryModel);
      if (!mounted) return;
      await _waitForDownloadsIdle([recommendedMemoryModel]);
      if (mounted) {
        setState(() {
          _memoryModelDownloading = false;
          _memoryModelDownloadDone = true;
          _hasEmbeddingModel = true;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _memoryModelDownloading = false;
          _memoryModelDownloadError = FriendlyError.describe(e);
        });
      }
    }
  }

  Future<void> _downloadRecommendedModels(GPUInfo gpu) async {
    setState(() {
      _modelDownloadStarted = true;
      _modelDownloadDone = false;
      _modelDownloadError = null;
    });
    try {
      // The backend downloads distinct repo+file pairs concurrently, so both
      // start right away instead of waiting for one to finish first.
      await Future.wait([
        _downloadCurated(recommendedChatModel(gpu)),
        _downloadCurated(recommendedMemoryModel),
      ]);
      if (!mounted) return;
      await _waitForDownloadsIdle([recommendedChatModel(gpu), recommendedMemoryModel]);
      if (mounted) setState(() => _modelDownloadDone = true);
    } catch (e) {
      if (mounted) setState(() => _modelDownloadError = FriendlyError.describe(e));
    }
  }

  Future<void> _downloadCurated(CuratedModel model) async {
    final api = ref.read(apiClientProvider);
    final files = await api.getModelFiles(model.repoId);
    final file = pickRecommendedFile(files);
    if (file == null) {
      throw Exception('${model.name}: no downloadable file found');
    }
    await api.downloadModel(model.repoId, file.filename, expectedSize: file.size);
  }

  /// Polls until none of [models]' downloads are still active. A download no
  /// longer present in the backend's list has already finished successfully
  /// (the backend prunes completed entries after a few seconds).
  Future<void> _waitForDownloadsIdle(List<CuratedModel> models) async {
    final api = ref.read(apiClientProvider);
    final repoIds = models.map((m) => m.repoId).toSet();
    while (true) {
      final progress = await api.getDownloadProgress();
      final relevant = progress.where((p) => repoIds.contains(p.repoId)).toList();
      for (final p in relevant) {
        if (p.active && p.error != null && p.error!.isNotEmpty) {
          throw Exception(p.error);
        }
      }
      if (relevant.every((p) => !p.active)) return;
      await Future.delayed(const Duration(seconds: 1));
    }
  }

  Future<void> _saveSetup() async {
    setState(() => _saving = true);
    try {
      await ref.read(systemPromptProvider.notifier).save(_finalPrompt);
      await ref.read(themeModeProvider.notifier).setMode(_selectedTheme);
      await ref.read(setupCompleteProvider.notifier).completeSetup();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${L10n.t('error')}: ${FriendlyError.describe(e)}')),
        );
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Theme(
      data: Theme.of(context).copyWith(
        brightness: _brightness,
        colorScheme: ColorScheme.fromSeed(
          seedColor: MemoTheme.accent,
          brightness: _brightness,
        ),
      ),
      child: Builder(
        builder: (context) {
          final c = _colors;
          final textTheme = Theme.of(context).textTheme;

          return Container(
            color: c.bgApp,
            child: Center(
              child: SingleChildScrollView(
                padding: EdgeInsets.symmetric(vertical: 40),
                child: Container(
                  width: 640,
                  margin: EdgeInsets.symmetric(horizontal: 24),
                  decoration: BoxDecoration(
                    color: c.bgPanel,
                    borderRadius: BorderRadius.circular(24),
                    border: Border.all(color: c.borderSoft),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withValues(alpha: _brightness == Brightness.dark ? 0.4 : 0.1),
                        blurRadius: 40,
                        offset: Offset(0, 8),
                      ),
                    ],
                  ),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      // ─── Hero ───────────────────────────────
                      Container(
                        width: double.infinity,
                        padding: EdgeInsets.fromLTRB(40, 40, 40, 28),
                        decoration: BoxDecoration(
                          border: Border(bottom: BorderSide(color: c.borderSoft)),
                        ),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Container(
                              width: 52,
                              height: 52,
                              decoration: BoxDecoration(
                                shape: BoxShape.circle,
                                gradient: LinearGradient(
                                  begin: Alignment.topLeft,
                                  end: Alignment.bottomRight,
                                  colors: [MemoTheme.accent, MemoTheme.accentLight],
                                ),
                                boxShadow: [
                                  BoxShadow(
                                    color: MemoTheme.accent.withValues(alpha: 0.35),
                                    blurRadius: 20,
                                    offset: Offset(0, 6),
                                  ),
                                ],
                              ),
                              child: Center(
                                child: Text(
                                  'M',
                                  style: TextStyle(
                                    fontFamily: textTheme.headlineSmall?.fontFamily,
                                    fontSize: 24,
                                    fontWeight: FontWeight.w700,
                                    color: c.textInverse,
                                  ),
                                ),
                              ),
                            ),
                            SizedBox(height: 20),
                            Text(
                              L10n.t('setup_eyebrow').toUpperCase(),
                              style: TextStyle(
                                fontSize: 11,
                                fontWeight: FontWeight.w700,
                                letterSpacing: 1.4,
                                color: MemoTheme.accent,
                              ),
                            ),
                            SizedBox(height: 8),
                            Text(
                              L10n.t('setup_wizard_title'),
                              style: textTheme.headlineSmall?.copyWith(
                                fontSize: 26,
                                height: 1.2,
                                fontWeight: FontWeight.w700,
                                color: c.textMain,
                              ),
                            ),
                            SizedBox(height: 10),
                            Text(
                              L10n.t('setup_subtitle'),
                              style: TextStyle(
                                fontSize: 14,
                                color: c.textDim,
                                height: 1.5,
                              ),
                            ),
                          ],
                        ),
                      ),

                      Padding(
                        padding: EdgeInsets.fromLTRB(40, 32, 40, 36),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            // ─── Step 1 — Language & Theme ────
                            _TimelineStep(
                              number: '1',
                              title: L10n.t('setup_step_language_theme'),
                              color: c,
                              child: Row(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Expanded(
                                    child: _Card(
                                      color: c,
                                      child: Column(
                                        crossAxisAlignment: CrossAxisAlignment.start,
                                        children: [
                                          _Label(text: L10n.t('language'), color: c),
                                          SizedBox(height: 10),
                                          _Pill(
                                            label: 'Türkçe',
                                            selected: L10n.locale == MemoLocale.tr,
                                            accent: MemoTheme.accent,
                                            color: c,
                                            onTap: () => setState(() {
                                              ref.read(localeProvider.notifier).setLocale(MemoLocale.tr);
                                            }),
                                          ),
                                          SizedBox(height: 6),
                                          _Pill(
                                            label: 'English',
                                            selected: L10n.locale == MemoLocale.en,
                                            accent: MemoTheme.accent,
                                            color: c,
                                            onTap: () => setState(() {
                                              ref.read(localeProvider.notifier).setLocale(MemoLocale.en);
                                            }),
                                          ),
                                        ],
                                      ),
                                    ),
                                  ),
                                  SizedBox(width: 12),
                                  Expanded(
                                    child: _Card(
                                      color: c,
                                      child: Column(
                                        crossAxisAlignment: CrossAxisAlignment.start,
                                        children: [
                                          _Label(text: L10n.t('theme'), color: c),
                                          SizedBox(height: 10),
                                          _Pill(
                                            label: L10n.t('theme_dark'),
                                            selected: _selectedTheme == 'dark',
                                            accent: MemoTheme.accent,
                                            color: c,
                                            onTap: () => setState(() => _selectedTheme = 'dark'),
                                          ),
                                          SizedBox(height: 6),
                                          _Pill(
                                            label: L10n.t('theme_light'),
                                            selected: _selectedTheme == 'light',
                                            accent: MemoTheme.accent,
                                            color: c,
                                            onTap: () => setState(() => _selectedTheme = 'light'),
                                          ),
                                          SizedBox(height: 6),
                                          _Pill(
                                            label: L10n.t('theme_system'),
                                            selected: _selectedTheme == 'system',
                                            accent: MemoTheme.accent,
                                            color: c,
                                            onTap: () => setState(() => _selectedTheme = 'system'),
                                          ),
                                        ],
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            ),

                            // ─── Step 2 — Persona ──────────────
                            _TimelineStep(
                              number: '2',
                              title: L10n.t('setup_step_persona'),
                              subtitle: L10n.t('setup_step_persona_desc'),
                              color: c,
                              child: PersonaPicker(
                                selectedKey: _customPrompt ? 'custom' : _selectedPrompt,
                                nameController: _nameController,
                                customController: _customPromptController,
                                onSelect: (key) => setState(() {
                                  if (key == 'custom') {
                                    _customPrompt = true;
                                  } else {
                                    _customPrompt = false;
                                    _selectedPrompt = key;
                                  }
                                }),
                                onNameChanged: (_) {},
                                onCustomTextChanged: (_) {},
                              ),
                            ),

                            // ─── Step 3 — Model Recommendation ─
                            _TimelineStep(
                              number: '3',
                              title: L10n.t('setup_step_model'),
                              color: c,
                              child: Consumer(
                                builder: (context, ref, _) {
                                  final gpuAsync = ref.watch(gpuInfoProvider);
                                  final gpu = gpuAsync.valueOrNull ?? const GPUInfo();
                                  final downloads =
                                      ref.watch(downloadProgressProvider).valueOrNull ?? [];
                                  final repoIds = {
                                    recommendedChatModel(gpu).repoId,
                                    recommendedMemoryModel.repoId,
                                  };
                                  final ourDownloads = downloads
                                      .where((p) => repoIds.contains(p.repoId))
                                      .toList();
                                  return _ModelRecommendationCard(
                                    color: c,
                                    checking: _checking,
                                    gpu: gpu,
                                    downloads: ourDownloads,
                                    alreadyHasModels: _modelsOk,
                                    localModelCount: _localModelCount,
                                    downloadStarted: _modelDownloadStarted,
                                    downloadDone: _modelDownloadDone,
                                    downloadError: _modelDownloadError,
                                    onDownload: _downloadRecommendedModels,
                                    providerConfigured: _providerConfigured,
                                    activeProviderName: _activeProviderName,
                                    onConnectProvider: _connectProvider,
                                    hasEmbeddingModel: _hasEmbeddingModel,
                                    memoryModelDownloading: _memoryModelDownloading,
                                    memoryModelDownloadDone: _memoryModelDownloadDone,
                                    memoryModelDownloadError: _memoryModelDownloadError,
                                    onDownloadMemoryModel: _downloadMemoryModel,
                                  );
                                },
                              ),
                            ),

                            // ─── Step 4 — Starting Preferences ─
                            _TimelineStep(
                              number: '4',
                              title: L10n.t('setup_step_preferences'),
                              subtitle: L10n.t('setup_step_preferences_desc'),
                              color: c,
                              child: Consumer(
                                builder: (context, ref, _) {
                                  final learning = ref.watch(learningSettingsProvider);
                                  final learningData = learning.valueOrNull;
                                  final proactiveEnabled = learningData?['enabled'] as bool? ?? false;
                                  final proactiveLevel = learningData?['level'] as String? ?? 'subtle';

                                  final minimal = ref.watch(minimalModeProvider);
                                  final minimalEnabled = minimal.valueOrNull ?? false;

                                  return Column(
                                    children: [
                                      _PreferenceToggle(
                                        icon: Icons.psychology_outlined,
                                        title: L10n.t('learning_proactive_title'),
                                        desc: L10n.t('setup_pref_proactive_desc'),
                                        value: proactiveEnabled,
                                        color: c,
                                        onChanged: learning.isLoading
                                            ? null
                                            : (v) => ref
                                                .read(learningSettingsProvider.notifier)
                                                .update(v, proactiveLevel),
                                      ),
                                      SizedBox(height: 10),
                                      _PreferenceToggle(
                                        icon: Icons.speed_outlined,
                                        title: L10n.t('minimal_mode_section'),
                                        desc: L10n.t('setup_pref_minimal_desc'),
                                        value: minimalEnabled,
                                        color: c,
                                        onChanged: minimal.isLoading
                                            ? null
                                            : (_) => ref.read(minimalModeProvider.notifier).toggle(),
                                      ),
                                    ],
                                  );
                                },
                              ),
                            ),

                            // ─── Step 5 — System Check ─────────
                            _TimelineStep(
                              number: '5',
                              title: L10n.t('setup_step_check'),
                              color: c,
                              isLast: true,
                              child: _Card(
                                color: c,
                                child: Column(
                                  children: [
                                    Row(
                                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                                      children: [
                                        Text(
                                          L10n.t('system_check'),
                                          style: TextStyle(
                                            fontSize: 13,
                                            fontWeight: FontWeight.w500,
                                            color: c.textSecondary,
                                          ),
                                        ),
                                        Tooltip(
                                          message: L10n.t('setup_check_refresh_tooltip'),
                                          waitDuration: const Duration(milliseconds: 300),
                                          child: InkWell(
                                            onTap: _checking ? null : _checkDiagnostics,
                                            child: Padding(
                                              padding: EdgeInsets.all(4),
                                              child: _checking
                                                  ? SizedBox(
                                                      width: 14,
                                                      height: 14,
                                                      child: CircularProgressIndicator(
                                                        strokeWidth: 2,
                                                        color: c.textDim,
                                                      ),
                                                    )
                                                  : Icon(
                                                      Icons.refresh,
                                                      size: 16,
                                                      color: c.textDim,
                                                    ),
                                            ),
                                          ),
                                        ),
                                      ],
                                    ),
                                    SizedBox(height: 14),
                                    _DiagRow(
                                      title: L10n.t('backend_connection'),
                                      ok: _backendOk,
                                      color: c,
                                      tooltip: L10n.t('setup_check_backend_tooltip'),
                                    ),
                                    SizedBox(height: 8),
                                    _DiagRow(
                                      title: L10n.t('local_models_status'),
                                      ok: _modelsOk,
                                      color: c,
                                      tooltip: L10n.t('setup_check_models_tooltip'),
                                    ),
                                    SizedBox(height: 8),
                                    _DiagRow(
                                      title: L10n.t('setup_check_ready'),
                                      ok: _modelsOk || _providerConfigured,
                                      color: c,
                                    ),
                                    if (!_backendOk || !(_modelsOk || _providerConfigured)) ...[
                                      SizedBox(height: 12),
                                      Container(
                                        padding: EdgeInsets.all(10),
                                        decoration: BoxDecoration(
                                          color: MemoTheme.warningOrange.withValues(alpha: 0.08),
                                          borderRadius: BorderRadius.circular(8),
                                        ),
                                        child: Text(
                                          L10n.t('system_check_info'),
                                          style: TextStyle(
                                            fontSize: 11,
                                            color: MemoTheme.warningOrange,
                                            height: 1.3,
                                          ),
                                        ),
                                      ),
                                    ],
                                  ],
                                ),
                              ),
                            ),

                            SizedBox(height: 4),

                            // ─── Save Button ──────────────────
                            SizedBox(
                              height: 50,
                              child: ElevatedButton(
                                onPressed: _saving ? null : _saveSetup,
                                style: ElevatedButton.styleFrom(
                                  backgroundColor: MemoTheme.accent,
                                  foregroundColor: c.textInverse,
                                  shape: RoundedRectangleBorder(
                                    borderRadius: BorderRadius.circular(14),
                                  ),
                                  elevation: 0,
                                ),
                                child: _saving
                                    ? SizedBox(
                                        width: 22,
                                        height: 22,
                                        child: CircularProgressIndicator(
                                          strokeWidth: 2.5,
                                          color: c.textInverse,
                                        ),
                                      )
                                    : Text(
                                        L10n.t('start_button'),
                                        style: TextStyle(
                                          fontWeight: FontWeight.bold,
                                          fontSize: 16,
                                        ),
                                      ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}

// ─── Helpers ─────────────────────────────────────────────────────

/// One step in the setup checklist: a numbered circle connected by a thin
/// bronze thread to the next step's circle — literalizes "a handful of
/// things, one thread, then you're in" rather than implying a gated,
/// paginated flow (every step is already visible/editable at once, so a
/// fake "locked until previous step done" stepper would be dishonest UI).
class _TimelineStep extends StatelessWidget {
  final String number;
  final String title;
  final String? subtitle;
  final Widget child;
  final ThemeColors color;
  final bool isLast;

  const _TimelineStep({
    required this.number,
    required this.title,
    this.subtitle,
    required this.child,
    required this.color,
    this.isLast = false,
  });

  @override
  Widget build(BuildContext context) {
    return IntrinsicHeight(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Column(
            children: [
              Container(
                width: 28,
                height: 28,
                decoration: BoxDecoration(
                  color: MemoTheme.accent.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(14),
                ),
                child: Center(
                  child: Text(
                    number,
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.bold,
                      color: MemoTheme.accent,
                    ),
                  ),
                ),
              ),
              if (!isLast)
                Expanded(
                  child: Container(
                    width: 2,
                    margin: EdgeInsets.symmetric(vertical: 6),
                    color: MemoTheme.accent.withValues(alpha: 0.18),
                  ),
                ),
            ],
          ),
          SizedBox(width: 14),
          Expanded(
            child: Padding(
              padding: EdgeInsets.only(bottom: isLast ? 0 : 28),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    title,
                    style: TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w600,
                      color: color.textMain,
                      height: 1.1,
                    ),
                  ),
                  if (subtitle != null) ...[
                    SizedBox(height: 4),
                    Text(
                      subtitle!,
                      style: TextStyle(fontSize: 12.5, color: color.textDim, height: 1.4),
                    ),
                  ],
                  SizedBox(height: 14),
                  child,
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// A small "?" affordance that explains a term or control on hover — for
/// someone who's never run a local AI model before, "chat model" vs "memory
/// model" or "API provider" aren't self-explanatory just from a label.
class _InfoTip extends StatelessWidget {
  final String message;
  final ThemeColors color;

  const _InfoTip({required this.message, required this.color});

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: message,
      waitDuration: const Duration(milliseconds: 300),
      child: Icon(Icons.help_outline_rounded, size: 14, color: color.textDim),
    );
  }
}

class _Card extends StatelessWidget {
  final Widget child;
  final ThemeColors color;

  const _Card({required this.child, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: color.bgApp,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: color.borderSoft),
      ),
      child: child,
    );
  }
}

class _Label extends StatelessWidget {
  final String text;
  final ThemeColors color;

  const _Label({required this.text, required this.color});

  @override
  Widget build(BuildContext context) {
    return Text(
      text,
      style: TextStyle(
        fontWeight: FontWeight.w600,
        fontSize: 13,
        color: color.textMain,
      ),
    );
  }
}

class _Pill extends StatelessWidget {
  final String label;
  final bool selected;
  final Color accent;
  final ThemeColors color;
  final VoidCallback onTap;

  const _Pill({
    required this.label,
    required this.selected,
    required this.accent,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: AnimatedContainer(
        duration: Duration(milliseconds: 150),
        padding: EdgeInsets.symmetric(horizontal: 16, vertical: 9),
        decoration: BoxDecoration(
          color: selected
              ? accent.withValues(alpha: 0.12)
              : color.bgElement,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(
            color: selected ? accent : color.borderSoft,
            width: selected ? 1.5 : 1,
          ),
        ),
        child: Text(
          label,
          style: TextStyle(
            fontSize: 12,
            fontWeight: selected ? FontWeight.w600 : FontWeight.normal,
            color: selected ? accent : color.textSecondary,
          ),
        ),
      ),
    );
  }
}

/// One background preference the user can decide on right away instead of
/// discovering it later — title, plain-language explanation, and a switch
/// that applies immediately (same provider Settings itself uses, so nothing
/// is lost if the wizard is closed before the final "Start" tap).
class _PreferenceToggle extends StatelessWidget {
  final IconData icon;
  final String title;
  final String desc;
  final bool value;
  final ThemeColors color;
  final ValueChanged<bool>? onChanged;

  const _PreferenceToggle({
    required this.icon,
    required this.title,
    required this.desc,
    required this.value,
    required this.color,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: color.bgApp,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: color.borderSoft),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 32,
            height: 32,
            decoration: BoxDecoration(
              color: MemoTheme.accent.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Icon(icon, size: 16, color: MemoTheme.accent),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: color.textMain),
                ),
                const SizedBox(height: 4),
                Text(
                  desc,
                  style: TextStyle(fontSize: 11.5, height: 1.4, color: color.textDim),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Switch(
            value: value,
            onChanged: onChanged,
            activeThumbColor: MemoTheme.accent,
          ),
        ],
      ),
    );
  }
}

class _ModelRecommendationCard extends StatelessWidget {
  final ThemeColors color;
  final bool checking;
  final GPUInfo gpu;
  final List<DownloadProgress> downloads;
  final bool alreadyHasModels;
  final int localModelCount;
  final bool downloadStarted;
  final bool downloadDone;
  final String? downloadError;
  final void Function(GPUInfo gpu) onDownload;
  final bool providerConfigured;
  final String? activeProviderName;
  final VoidCallback onConnectProvider;
  final bool hasEmbeddingModel;
  final bool memoryModelDownloading;
  final bool memoryModelDownloadDone;
  final String? memoryModelDownloadError;
  final VoidCallback onDownloadMemoryModel;

  const _ModelRecommendationCard({
    required this.color,
    required this.checking,
    required this.gpu,
    required this.downloads,
    required this.alreadyHasModels,
    required this.localModelCount,
    required this.downloadStarted,
    required this.downloadDone,
    required this.downloadError,
    required this.onDownload,
    required this.providerConfigured,
    required this.activeProviderName,
    required this.onConnectProvider,
    required this.hasEmbeddingModel,
    required this.memoryModelDownloading,
    required this.memoryModelDownloadDone,
    required this.memoryModelDownloadError,
    required this.onDownloadMemoryModel,
  });

  @override
  Widget build(BuildContext context) {
    return _Card(
      color: color,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _content(),
          if (!checking) ...[
            SizedBox(height: 14),
            _providerSection(),
          ],
        ],
      ),
    );
  }

  /// Alternative to the local-model download above: connect an API provider
  /// instead, so setup doesn't force a multi-GB download on someone who
  /// already has an OpenAI/Gemini/Claude key. Shown regardless of the local
  /// model state above — a provider is useful even alongside local models.
  Widget _providerSection() {
    if (providerConfigured) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.check_circle_rounded, color: MemoTheme.green, size: 18),
              SizedBox(width: 8),
              Expanded(
                child: Text(
                  L10n.t('setup_provider_connected', {'name': activeProviderName ?? ''}),
                  style: TextStyle(fontSize: 12, color: color.textMain),
                ),
              ),
            ],
          ),
          if (!hasEmbeddingModel) ...[
            SizedBox(height: 10),
            _memoryModelWarning(),
          ],
        ],
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(child: Divider(color: color.borderSoft)),
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 10),
              child: Text(
                L10n.t('setup_or'),
                style: TextStyle(fontSize: 11, color: color.textDim),
              ),
            ),
            Expanded(child: Divider(color: color.borderSoft)),
          ],
        ),
        SizedBox(height: 10),
        Row(
          children: [
            Expanded(
              child: SizedBox(
                height: 42,
                child: OutlinedButton.icon(
                  onPressed: onConnectProvider,
                  icon: Icon(Icons.cloud_outlined, size: 16, color: color.textMain),
                  label: Text(
                    L10n.t('setup_provider_connect_button'),
                    style: TextStyle(
                      color: color.textMain,
                      fontWeight: FontWeight.w600,
                      fontSize: 13,
                    ),
                  ),
                  style: OutlinedButton.styleFrom(
                    side: BorderSide(color: color.borderSoft),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
                  ),
                ),
              ),
            ),
            SizedBox(width: 8),
            _InfoTip(message: L10n.t('setup_provider_connect_tooltip'), color: color),
          ],
        ),
      ],
    );
  }

  /// An API provider only ever covers chat — memory/RAG always runs against
  /// the local embedding model regardless of which chat provider is active,
  /// so connecting a provider alone leaves memory silently, permanently off
  /// with nothing telling the user why. Surfaced right here instead, with a
  /// one-click download for the same curated memory model the local-model
  /// path above already recommends.
  Widget _memoryModelWarning() {
    return Container(
      padding: EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: MemoTheme.warningOrange.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(Icons.info_outline, size: 16, color: MemoTheme.warningOrange),
              SizedBox(width: 8),
              Expanded(
                child: Text(
                  L10n.t('setup_memory_model_missing'),
                  style: TextStyle(fontSize: 11, color: color.textMain, height: 1.4),
                ),
              ),
            ],
          ),
          SizedBox(height: 10),
          if (memoryModelDownloadError != null) ...[
            Text(
              memoryModelDownloadError!,
              style: TextStyle(fontSize: 11, color: MemoTheme.red),
            ),
            SizedBox(height: 8),
          ],
          if (memoryModelDownloadDone)
            Row(
              children: [
                Icon(Icons.check_circle_rounded, color: MemoTheme.green, size: 16),
                SizedBox(width: 6),
                Text(
                  L10n.t('setup_memory_model_ready'),
                  style: TextStyle(fontSize: 12, color: color.textMain),
                ),
              ],
            )
          else if (memoryModelDownloading)
            Row(
              children: [
                SizedBox(
                  width: 14,
                  height: 14,
                  child: CircularProgressIndicator(strokeWidth: 2, color: MemoTheme.warningOrange),
                ),
                SizedBox(width: 8),
                Text(
                  L10n.t('setup_memory_model_downloading'),
                  style: TextStyle(fontSize: 12, color: color.textDim),
                ),
              ],
            )
          else
            OutlinedButton.icon(
              onPressed: onDownloadMemoryModel,
              icon: Icon(Icons.download_outlined, size: 15, color: MemoTheme.warningOrange),
              label: Text(
                L10n.t('setup_memory_model_download_button'),
                style: TextStyle(
                  color: MemoTheme.warningOrange,
                  fontWeight: FontWeight.w600,
                  fontSize: 12,
                ),
              ),
              style: OutlinedButton.styleFrom(
                side: BorderSide(color: MemoTheme.warningOrange.withValues(alpha: 0.4)),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                padding: EdgeInsets.symmetric(horizontal: 10, vertical: 6),
              ),
            ),
        ],
      ),
    );
  }

  Widget _content() {
    if (checking) {
      return Row(
        children: [
          SizedBox(
            width: 16,
            height: 16,
            child: CircularProgressIndicator(strokeWidth: 2, color: color.textDim),
          ),
          SizedBox(width: 10),
          Text(
            L10n.t('setup_model_checking'),
            style: TextStyle(fontSize: 13, color: color.textDim),
          ),
        ],
      );
    }

    if (alreadyHasModels) {
      return Row(
        children: [
          Icon(Icons.check_circle_rounded, color: MemoTheme.green, size: 20),
          SizedBox(width: 10),
          Expanded(
            child: Text(
              L10n.t('setup_model_already', {'count': '$localModelCount'}),
              style: TextStyle(fontSize: 13, color: color.textMain),
            ),
          ),
        ],
      );
    }

    final chatModel = recommendedChatModel(gpu);
    final memModel = recommendedMemoryModel;
    final gpuLine = gpu.hasGpu
        ? '${gpu.name} — ${gpu.vramFormatted} VRAM'
        : L10n.t('setup_model_gpu_none');
    final ramLine = gpu.ramTotalMb > 0 ? gpu.ramFormatted : L10n.t('setup_model_ram_unknown');

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          L10n.t('setup_model_hello'),
          style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: color.textMain),
        ),
        SizedBox(height: 10),
        _SpecRow(label: L10n.t('setup_model_ram'), value: ramLine, color: color),
        SizedBox(height: 4),
        _SpecRow(
          label: L10n.t('setup_model_gpu'),
          value: gpuLine,
          color: color,
        ),
        SizedBox(height: 14),
        Container(
          padding: EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: MemoTheme.accent.withValues(alpha: 0.06),
            borderRadius: BorderRadius.circular(10),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _SpecRow(
                label: L10n.t('setup_model_chat_label'),
                value: '${chatModel.name} (${chatModel.sizeFormatted})',
                color: color,
                bold: true,
                tooltip: L10n.t('setup_model_chat_tooltip'),
              ),
              SizedBox(height: 4),
              _SpecRow(
                label: L10n.t('setup_model_memory_label'),
                value: memModel.name,
                color: color,
                bold: true,
                tooltip: L10n.t('setup_model_memory_tooltip'),
              ),
            ],
          ),
        ),
        SizedBox(height: 8),
        Text(
          L10n.t('setup_model_local_note'),
          style: TextStyle(fontSize: 11, color: color.textDim, height: 1.4),
        ),
        SizedBox(height: 14),
        if (downloadError != null) ...[
          Text(
            downloadError!,
            style: TextStyle(fontSize: 12, color: MemoTheme.red),
          ),
          SizedBox(height: 8),
        ],
        if (!downloadStarted)
          SizedBox(
            width: double.infinity,
            height: 42,
            child: OutlinedButton(
              onPressed: () => onDownload(gpu),
              style: OutlinedButton.styleFrom(
                side: BorderSide(color: MemoTheme.accent),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
              ),
              child: Text(
                L10n.t('setup_model_download_button'),
                style: TextStyle(
                  color: MemoTheme.accent,
                  fontWeight: FontWeight.w600,
                  fontSize: 13,
                ),
              ),
            ),
          )
        else if (downloadDone)
          Row(
            children: [
              Icon(Icons.check_circle_rounded, color: MemoTheme.green, size: 18),
              SizedBox(width: 8),
              Text(
                L10n.t('setup_model_download_done'),
                style: TextStyle(fontSize: 12, color: color.textMain),
              ),
            ],
          )
        else ...[
          for (final d in downloads) ...[
            _DownloadProgressRow(progress: d, color: color),
            SizedBox(height: 8),
          ],
          Text(
            L10n.t('setup_model_download_bg_note'),
            style: TextStyle(fontSize: 11, color: color.textDim, height: 1.4),
          ),
        ],
      ],
    );
  }
}

class _SpecRow extends StatelessWidget {
  final String label;
  final String value;
  final ThemeColors color;
  final bool bold;
  final String? tooltip;

  const _SpecRow({
    required this.label,
    required this.value,
    required this.color,
    this.bold = false,
    this.tooltip,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 100,
          child: Row(
            children: [
              Flexible(
                child: Text(label, style: TextStyle(fontSize: 12, color: color.textDim)),
              ),
              if (tooltip != null) ...[
                SizedBox(width: 4),
                _InfoTip(message: tooltip!, color: color),
              ],
            ],
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: TextStyle(
              fontSize: 12,
              fontWeight: bold ? FontWeight.w600 : FontWeight.normal,
              color: color.textMain,
            ),
          ),
        ),
      ],
    );
  }
}

class _DownloadProgressRow extends StatelessWidget {
  final DownloadProgress progress;
  final ThemeColors color;

  const _DownloadProgressRow({
    required this.progress,
    required this.color,
  });

  /// Parses the backend's already-formatted speed string (e.g. "3.2 MB/s",
  /// from Go's `formatSpeed`) back into bytes/sec, then estimates time
  /// remaining from `totalBytes - downloaded`. A non-technical user staring
  /// at a raw percentage with no sense of whether this takes 30 seconds or
  /// 30 minutes is the #1 reason downloads get abandoned mid-way.
  String? _etaLabel() {
    final p = progress;
    if (p.speed.isEmpty || p.totalBytes <= 0) return null;
    final match = RegExp(r'([\d.]+)\s*(B|KB|MB|GB)/s').firstMatch(p.speed);
    if (match == null) return null;
    final value = double.tryParse(match.group(1)!);
    if (value == null || value <= 0) return null;
    final unit = match.group(2);
    final bytesPerSec = switch (unit) {
      'GB' => value * 1024 * 1024 * 1024,
      'MB' => value * 1024 * 1024,
      'KB' => value * 1024,
      _ => value,
    };
    final remainingBytes = p.totalBytes - p.downloaded;
    if (remainingBytes <= 0 || bytesPerSec <= 0) return null;
    final seconds = remainingBytes / bytesPerSec;
    if (seconds < 60) {
      return L10n.t('setup_eta_seconds', {'s': '${seconds.round()}'});
    }
    final minutes = (seconds / 60).round();
    return L10n.t('setup_eta_minutes', {'m': '$minutes'});
  }

  @override
  Widget build(BuildContext context) {
    final p = progress;
    final pct = p.percent.clamp(0, 100).toDouble();
    final label = p.filename.isNotEmpty ? p.filename : L10n.t('setup_preparing');
    final speedSuffix = p.speed.isNotEmpty ? ' (${p.speed})' : '';
    final eta = _etaLabel();
    final etaSuffix = eta != null ? ' — $eta' : '';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(4),
          child: LinearProgressIndicator(
            value: pct > 0 ? pct / 100 : null,
            minHeight: 6,
            backgroundColor: color.bgElement,
            color: MemoTheme.accent,
          ),
        ),
        SizedBox(height: 6),
        Text(
          '$label — %${pct.toStringAsFixed(0)}$speedSuffix$etaSuffix',
          style: TextStyle(fontSize: 11, color: color.textDim),
        ),
      ],
    );
  }
}

class _DiagRow extends StatelessWidget {
  final String title;
  final bool ok;
  final ThemeColors color;
  final String? tooltip;

  const _DiagRow({required this.title, required this.ok, required this.color, this.tooltip});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(
          ok ? Icons.check_circle_rounded : Icons.error_outline_rounded,
          color: ok ? MemoTheme.green : MemoTheme.red,
          size: 20,
        ),
        SizedBox(width: 10),
        Text(
          title,
          style: TextStyle(
            fontSize: 13,
            color: ok ? color.textMain : MemoTheme.red,
          ),
        ),
        if (tooltip != null) ...[
          SizedBox(width: 6),
          _InfoTip(message: tooltip!, color: color),
        ],
      ],
    );
  }
}
