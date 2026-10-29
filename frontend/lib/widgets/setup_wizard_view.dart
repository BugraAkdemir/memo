import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart' show debugPrint, kIsWeb;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/friendly_error.dart';
import '../core/l10n.dart';
import '../core/notification_service.dart';
import '../core/platform_capabilities.dart';
import '../core/theme.dart';
import '../models/curated_models.dart';
import '../models/gpu_info.dart';
import '../models/local_model.dart';
import '../models/persona_presets.dart';
import '../providers/auth_gate_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/learning_provider.dart';
import '../providers/models_provider.dart';
import '../providers/provider_provider.dart';
import '../providers/settings_provider.dart';
import '../providers/telemetry_provider.dart';
import 'persona_picker.dart';
import 'provider_config_dialog.dart';
import 'settings/tabs/subscriptions_tab.dart';
import 'telemetry_details.dart';

class SetupWizardOverlay extends ConsumerWidget {
  const SetupWizardOverlay({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isSetupComplete = ref.watch(setupCompleteProvider);

    if (isSetupComplete) {
      return SizedBox.shrink();
    }

    if (isMobilePlatform || kIsWeb) {
      // A phone connects (MobileConnectScreen) and signs in (AuthGateOverlay)
      // before anything else, and the wizard is only for a server that has
      // never been set up. Until the server has actually answered "onboarding
      // complete?", the local preference's `false` is just the default of a
      // fresh install — acting on it is what showed the full wizard against
      // servers that were long since configured.
      final gate = ref.watch(authGateProvider).valueOrNull;
      if ((isMobilePlatform && ref.watch(backendUrlProvider).isEmpty) ||
          gate == null ||
          gate.state != AuthGateState.ok ||
          !ref.watch(onboardingCheckedProvider)) {
        return const SizedBox.shrink();
      }
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
  bool _notifPermissionGranted = false;
  bool _notifPermissionAsking = false;

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
    return composePersonaPrompt(
      _nameController.text,
      _promptFor(_selectedPrompt),
    );
  }

  @override
  void initState() {
    super.initState();
    _checkDiagnostics();
  }

  @override
  void dispose() {
    _pageScroll.dispose();
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
        _modelsOk = models.any((m) => !m.isEmbedding);
        _localModelCount = models.where((m) => !m.isEmbedding).length;
        _hasEmbeddingModel = models.any((m) => m.isEmbedding);
        final activeProvider = await ref
            .read(apiClientProvider)
            .getActiveProvider();
        _providerConfigured = activeProvider.isNotEmpty;
        _activeProviderName = activeProvider.isEmpty ? null : activeProvider;
      }
    } catch (_) {
      _backendOk = false;
      _modelsOk = false;
    }
    if (mounted) setState(() => _checking = false);
  }

  /// Asks the OS for notification permission, from a place where the user
  /// can see what it is for.
  ///
  /// Both platforms return the already-granted answer instead of prompting
  /// again, so tapping this twice is harmless. There is no way to *revoke*
  /// it from inside the app — that lives in system settings — so the toggle
  /// only ever moves one way here.
  Future<void> _askNotificationPermission() async {
    if (_notifPermissionAsking) return;
    setState(() => _notifPermissionAsking = true);
    try {
      final granted = await NotificationService.requestPermission();
      if (mounted) setState(() => _notifPermissionGranted = granted);
    } finally {
      if (mounted) setState(() => _notifPermissionAsking = false);
    }
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
      before = (await ref.read(
        providerListProvider.future,
      )).map((p) => p.name).toSet();
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
      _memoryModelDownloadError = null;
    });
    try {
      await _downloadCurated(recommendedMemoryModel);
      if (!mounted) return;
      await _waitForDownloadsIdle([recommendedMemoryModel]);
      if (mounted) {
        setState(() {
          _memoryModelDownloading = false;
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
      await _waitForDownloadsIdle([
        recommendedChatModel(gpu),
        recommendedMemoryModel,
      ]);
      if (mounted) setState(() => _modelDownloadDone = true);
    } catch (e) {
      if (mounted) {
        setState(() => _modelDownloadError = FriendlyError.describe(e));
      }
    }
  }

  Future<void> _downloadCurated(CuratedModel model) async {
    final api = ref.read(apiClientProvider);
    final files = await api.getModelFiles(model.repoId);
    final file = pickRecommendedFile(files);
    if (file == null) {
      throw Exception('${model.name}: no downloadable file found');
    }
    await api.downloadModel(
      model.repoId,
      file.filename,
      expectedSize: file.size,
    );
  }

  /// Polls until none of [models]' downloads are still active. A download no
  /// longer present in the backend's list has already finished successfully
  /// (the backend prunes completed entries after a few seconds).
  Future<void> _waitForDownloadsIdle(List<CuratedModel> models) async {
    final api = ref.read(apiClientProvider);
    final repoIds = models.map((m) => m.repoId).toSet();
    while (true) {
      final progress = await api.getDownloadProgress();
      final relevant = progress
          .where((p) => repoIds.contains(p.repoId))
          .toList();
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
          SnackBar(
            content: Text('${L10n.t('error')}: ${FriendlyError.describe(e)}'),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  int _page = 0;
  String _connection = 'local';
  bool _connectingSubscription = false;
  String? _subscriptionError;
  final _pageScroll = ScrollController();

  void _goTo(int page) {
    setState(() => _page = page);
    if (_pageScroll.hasClients) _pageScroll.jumpTo(0);
    if (page == 4) _checkDiagnostics();
  }

  @override
  Widget build(BuildContext context) {
    final c = _colors;
    return Theme(
      data: Theme.of(context).copyWith(
        brightness: _brightness,
        extensions: [c],
        colorScheme: ColorScheme.fromSeed(
          seedColor: MemoTheme.accent,
          brightness: _brightness,
          primary: MemoTheme.accent,
          onPrimary: c.textInverse,
          surface: c.bgPanel,
          onSurface: c.textMain,
        ),
      ),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final wide =
              constraints.maxWidth >= 800 && constraints.maxHeight >= 680;
          final narrow = constraints.maxWidth < 560;
          final labels = [
            L10n.t('setup_step_language_theme'),
            L10n.t('setup_step_persona'),
            L10n.t('setup_step_model'),
            L10n.t('setup_step_preferences'),
            L10n.t('setup_step_check'),
          ];
          final pages = <Widget Function()>[
            () => _SetupPage(
              title: L10n.t('setup_step_language_theme'),
              color: c,
              child: Builder(
                builder: (context) {
                  final languageCard = _Card(
                    color: c,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        _Label(text: L10n.t('language'), color: c),
                        SizedBox(height: 10),
                        _Pill(
                          label: L10n.t('language_tr'),
                          selected: L10n.locale == MemoLocale.tr,
                          accent: MemoTheme.accent,
                          color: c,
                          onTap: () => setState(() {
                            ref
                                .read(localeProvider.notifier)
                                .setLocale(MemoLocale.tr);
                          }),
                        ),
                        SizedBox(height: 6),
                        _Pill(
                          label: L10n.t('language_en'),
                          selected: L10n.locale == MemoLocale.en,
                          accent: MemoTheme.accent,
                          color: c,
                          onTap: () => setState(() {
                            ref
                                .read(localeProvider.notifier)
                                .setLocale(MemoLocale.en);
                          }),
                        ),
                      ],
                    ),
                  );
                  final themeCard = _Card(
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
                          onTap: () =>
                              setState(() => _selectedTheme = 'system'),
                        ),
                      ],
                    ),
                  );

                  if (narrow) {
                    return Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [languageCard, SizedBox(height: 12), themeCard],
                    );
                  }
                  return Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(child: languageCard),
                      SizedBox(width: 12),
                      Expanded(child: themeCard),
                    ],
                  );
                },
              ),
            ),
            () => _SetupPage(
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
            () => _SetupPage(
              title: L10n.t('setup_step_model'),
              subtitle: L10n.t('setup_connection_intro'),
              color: c,
              child: _buildConnection(c),
            ),
            () => _SetupPage(
              title: L10n.t('setup_step_preferences'),
              subtitle: L10n.t('setup_step_preferences_desc'),
              color: c,
              child: Consumer(
                builder: (context, ref, _) {
                  final learning = ref.watch(learningSettingsProvider);
                  final learningData = learning.valueOrNull;
                  final proactiveEnabled =
                      learningData?['enabled'] as bool? ?? false;
                  final proactiveLevel =
                      learningData?['level'] as String? ?? 'subtle';

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
                            : (_) => ref
                                  .read(minimalModeProvider.notifier)
                                  .toggle(),
                      ),
                      const SetupTelemetryToggle(),
                      // Mobile only: asking for the OS
                      // notification permission here, rather
                      // than inside NotificationService.init(),
                      // is the whole point — init() at launch
                      // would fire a system prompt before the
                      // user had seen any of Memo's own UI or
                      // any reason for it.
                      if (notificationsSupported) ...[
                        SizedBox(height: 10),
                        _PreferenceToggle(
                          icon: Icons.notifications_active_outlined,
                          title: L10n.t('notif_permission_title'),
                          desc: L10n.t('notif_permission_desc'),
                          value: _notifPermissionGranted,
                          color: c,
                          onChanged: _notifPermissionAsking
                              ? null
                              : (_) => _askNotificationPermission(),
                        ),
                      ],
                    ],
                  );
                },
              ),
            ),
            () => _SetupPage(
              title: L10n.t('setup_step_check'),
              color: c,

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
                      optional: _providerConfigured,
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
                          color: MemoTheme.warningOrange.withValues(
                            alpha: 0.08,
                          ),
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
          ];
          return ColoredBox(
            color: c.bgApp,
            child: SafeArea(
              child: Center(
                child: Padding(
                  padding: EdgeInsets.all(wide ? 24 : 8),
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(
                      maxWidth: 1120,
                      maxHeight: 880,
                    ),
                    child: Material(
                      color: c.bgPanel,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(24),
                        side: BorderSide(color: c.borderSoft),
                      ),
                      clipBehavior: Clip.antiAlias,
                      child: Row(
                        children: [
                          if (wide)
                            SizedBox(
                              width: 240,
                              child: Material(
                                color: c.bgElement,
                                child: Padding(
                                  padding: const EdgeInsets.all(24),
                                  child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Icon(
                                        Icons.auto_awesome_rounded,
                                        color: MemoTheme.accent,
                                        size: 32,
                                      ),
                                      const SizedBox(height: 20),
                                      Text(
                                        L10n.t('setup_eyebrow'),
                                        style: TextStyle(
                                          color: c.textMain,
                                          fontSize: 24,
                                          fontWeight: FontWeight.w700,
                                        ),
                                      ),
                                      const SizedBox(height: 8),
                                      Text(
                                        L10n.t('setup_journey_desc'),
                                        style: TextStyle(
                                          color: c.textDim,
                                          fontSize: 13,
                                          height: 1.5,
                                        ),
                                      ),
                                      const SizedBox(height: 32),
                                      for (var i = 0; i < labels.length; i++)
                                        Padding(
                                          padding: const EdgeInsets.only(
                                            bottom: 8,
                                          ),
                                          child: ListTile(
                                            contentPadding:
                                                const EdgeInsets.symmetric(
                                                  horizontal: 8,
                                                ),
                                            shape: RoundedRectangleBorder(
                                              borderRadius:
                                                  BorderRadius.circular(10),
                                            ),
                                            selected: i == _page,
                                            selectedTileColor: MemoTheme.accent
                                                .withValues(alpha: .12),
                                            leading: Icon(
                                              i < _page
                                                  ? Icons.check_circle_outline
                                                  : Icons.circle_outlined,
                                              color: i == _page
                                                  ? MemoTheme.accent
                                                  : c.textDim,
                                              size: 20,
                                            ),
                                            title: Text(
                                              labels[i],
                                              style: TextStyle(
                                                fontSize: 13,
                                                fontWeight: i == _page
                                                    ? FontWeight.w600
                                                    : FontWeight.w400,
                                                color: i == _page
                                                    ? c.textMain
                                                    : c.textDim,
                                              ),
                                            ),
                                            onTap: _saving
                                                ? null
                                                : () => _goTo(i),
                                          ),
                                        ),
                                      const Spacer(),
                                      Text(
                                        L10n.t('setup_change_later'),
                                        style: TextStyle(
                                          fontSize: 12,
                                          height: 1.5,
                                          color: c.textDim,
                                        ),
                                      ),
                                    ],
                                  ),
                                ),
                              ),
                            ),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.stretch,
                              children: [
                                Padding(
                                  padding: EdgeInsets.fromLTRB(
                                    wide ? 32 : 20,
                                    24,
                                    wide ? 32 : 20,
                                    16,
                                  ),
                                  child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Row(
                                        children: [
                                          Icon(
                                            Icons.auto_awesome_outlined,
                                            color: MemoTheme.accent,
                                            size: 18,
                                          ),
                                          const SizedBox(width: 8),
                                          Expanded(
                                            child: Text(
                                              L10n.t('setup_progress', {
                                                'current': '${_page + 1}',
                                                'total': '${pages.length}',
                                              }),
                                              style: TextStyle(
                                                fontSize: 12,
                                                color: c.textDim,
                                              ),
                                            ),
                                          ),
                                        ],
                                      ),
                                      const SizedBox(height: 12),
                                      ClipRRect(
                                        borderRadius: BorderRadius.circular(4),
                                        child: LinearProgressIndicator(
                                          value: (_page + 1) / pages.length,
                                          minHeight: 3,
                                          color: MemoTheme.accent,
                                          backgroundColor: c.borderSoft,
                                        ),
                                      ),
                                    ],
                                  ),
                                ),
                                Expanded(
                                  child: SingleChildScrollView(
                                    controller: _pageScroll,
                                    padding: EdgeInsets.fromLTRB(
                                      wide ? 32 : 20,
                                      12,
                                      wide ? 32 : 20,
                                      24,
                                    ),
                                    child: KeyedSubtree(
                                      key: ValueKey(_page),
                                      child: pages[_page](),
                                    ),
                                  ),
                                ),
                                Divider(height: 1, color: c.borderSoft),
                                Padding(
                                  padding: EdgeInsets.symmetric(
                                    horizontal: wide ? 32 : 20,
                                    vertical: 16,
                                  ),
                                  child: Row(
                                    children: [
                                      if (_page > 0)
                                        TextButton.icon(
                                          key: const Key('setup_back'),
                                          onPressed: _saving
                                              ? null
                                              : () => _goTo(_page - 1),
                                          icon: const Icon(
                                            Icons.arrow_back,
                                            size: 16,
                                          ),
                                          label: Text(L10n.t('back')),
                                        ),
                                      const Spacer(),
                                      FilledButton.icon(
                                        key: const Key('setup_next'),
                                        onPressed: _saving
                                            ? null
                                            : _page == pages.length - 1
                                            ? _saveSetup
                                            : () => _goTo(_page + 1),
                                        icon: _saving
                                            ? const SizedBox(
                                                width: 16,
                                                height: 16,
                                                child:
                                                    CircularProgressIndicator(
                                                      strokeWidth: 2,
                                                    ),
                                              )
                                            : Icon(
                                                _page == pages.length - 1
                                                    ? Icons.check
                                                    : Icons.arrow_forward,
                                                size: 18,
                                              ),
                                        label: Text(
                                          L10n.t(
                                            _page == pages.length - 1
                                                ? 'start_button'
                                                : 'next',
                                          ),
                                        ),
                                      ),
                                    ],
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
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildConnection(ThemeColors c) {
    final subscriptions = ref.watch(subscriptionsProvider).valueOrNull;
    final options = [
      (
        'local',
        Icons.computer_outlined,
        'setup_local_title',
        'setup_local_desc',
      ),
      ('api', Icons.key_outlined, 'setup_api_title', 'setup_api_desc'),
      (
        'subscriptions',
        Icons.account_circle_outlined,
        'setup_subs_title',
        'setup_subs_desc',
      ),
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        LayoutBuilder(
          builder: (context, constraints) {
            final horizontal = constraints.maxWidth >= 620;
            final width = horizontal
                ? (constraints.maxWidth - 24) / 3
                : constraints.maxWidth;
            return Wrap(
              spacing: 12,
              runSpacing: 12,
              children: [
                for (final option in options)
                  SizedBox(
                    width: width,
                    child: _ConnectionChoice(
                      key: ValueKey('setup_connection_${option.$1}'),
                      color: c,
                      selected: _connection == option.$1,
                      icon: option.$2,
                      title: L10n.t(option.$3),
                      description: L10n.t(option.$4),
                      onTap: () => setState(() => _connection = option.$1),
                    ),
                  ),
              ],
            );
          },
        ),
        const SizedBox(height: 20),
        if (_connection == 'local')
          Consumer(
            builder: (context, ref, _) {
              final gpu =
                  ref.watch(gpuInfoProvider).valueOrNull ?? const GPUInfo();
              final repoIds = {
                recommendedChatModel(gpu).repoId,
                recommendedMemoryModel.repoId,
              };
              final downloads =
                  (ref.watch(downloadProgressProvider).valueOrNull ??
                          <DownloadProgress>[])
                      .where((p) => repoIds.contains(p.repoId))
                      .toList();
              return _ModelRecommendationCard(
                color: c,
                checking: _checking,
                gpu: gpu,
                downloads: downloads,
                alreadyHasModels: _modelsOk,
                localModelCount: _localModelCount,
                downloadStarted: _modelDownloadStarted,
                downloadDone: _modelDownloadDone,
                downloadError: _modelDownloadError,
                onDownload: _downloadRecommendedModels,
              );
            },
          ),
        if (_connection == 'api')
          _Card(
            color: c,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (_providerConfigured)
                  Text(
                    L10n.t('setup_provider_connected', {
                      'name': _activeProviderName ?? '',
                    }),
                    style: TextStyle(color: c.textMain, fontSize: 13),
                  ),
                const SizedBox(height: 12),
                FilledButton.icon(
                  onPressed: _connectProvider,
                  icon: const Icon(Icons.add_link, size: 18),
                  label: Text(L10n.t('setup_provider_connect_button')),
                ),
              ],
            ),
          ),
        if (_connection == 'subscriptions') ...[
          ClipRRect(
            borderRadius: BorderRadius.circular(14),
            child: const SubscriptionsTab(embedded: true),
          ),
          const SizedBox(height: 12),
          Text(
            L10n.t('setup_subs_wait'),
            style: TextStyle(color: c.textDim, fontSize: 12, height: 1.5),
          ),
          const SizedBox(height: 12),
          Align(
            alignment: Alignment.centerRight,
            child: FilledButton.icon(
              key: const Key('setup_use_subscription'),
              onPressed:
                  _connectingSubscription ||
                      subscriptions == null ||
                      subscriptions.accounts.isEmpty ||
                      subscriptions.models.isEmpty
                  ? null
                  : _useSubscription,
              icon: const Icon(Icons.check, size: 18),
              label: Text(L10n.t('setup_subs_use')),
            ),
          ),
        ],
        if (_connection == 'subscriptions' &&
            _activeProviderName == 'Subscriptions')
          Padding(
            padding: const EdgeInsets.only(top: 12),
            child: Text(
              L10n.t('setup_subs_ready'),
              style: const TextStyle(color: MemoTheme.green, fontSize: 13),
            ),
          ),
        if (_connection == 'subscriptions' && _subscriptionError != null)
          Padding(
            padding: const EdgeInsets.only(top: 12),
            child: Text(
              _subscriptionError!,
              style: const TextStyle(color: MemoTheme.red, fontSize: 13),
            ),
          ),
        if (_connection != 'local' && !_hasEmbeddingModel) ...[
          const SizedBox(height: 20),
          _Card(
            color: c,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  L10n.t('setup_memory_model_missing'),
                  style: TextStyle(color: c.textDim, fontSize: 12, height: 1.5),
                ),
                const SizedBox(height: 12),
                if (_memoryModelDownloadError != null)
                  Text(
                    _memoryModelDownloadError!,
                    style: const TextStyle(color: MemoTheme.red),
                  ),
                OutlinedButton.icon(
                  onPressed: _memoryModelDownloading
                      ? null
                      : _downloadMemoryModel,
                  icon: _memoryModelDownloading
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.download_outlined, size: 18),
                  label: Text(
                    L10n.t(
                      _memoryModelDownloading
                          ? 'setup_memory_model_downloading'
                          : 'setup_memory_model_download_button',
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
        if (_connection != 'local' && _hasEmbeddingModel)
          Padding(
            padding: const EdgeInsets.only(top: 16),
            child: Text(
              L10n.t('setup_memory_model_ready'),
              style: const TextStyle(color: MemoTheme.green),
            ),
          ),
      ],
    );
  }

  Future<void> _useSubscription() async {
    if (_connectingSubscription) return;
    setState(() {
      _connectingSubscription = true;
      _subscriptionError = null;
    });
    try {
      await ref.read(apiClientProvider).setActiveProvider('Subscriptions');
      if (!mounted) return;
      ref.read(activeProviderTypeProvider.notifier).setActive('Subscriptions');
      ref.invalidate(providerListProvider);
      await _checkDiagnostics();
    } catch (e) {
      if (mounted) {
        setState(() => _subscriptionError = FriendlyError.describeGeneric(e));
      }
    } finally {
      if (mounted) setState(() => _connectingSubscription = false);
    }
  }
}

// ─── Helpers ─────────────────────────────────────────────────────

class _SetupPage extends StatelessWidget {
  final String title;
  final String? subtitle;
  final Widget child;
  final ThemeColors color;
  const _SetupPage({
    required this.title,
    this.subtitle,
    required this.child,
    required this.color,
  });
  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      Text(
        title,
        style: TextStyle(
          fontSize: 26,
          height: 1.2,
          fontWeight: FontWeight.w700,
          color: color.textMain,
        ),
      ),
      if (subtitle != null) ...[
        const SizedBox(height: 10),
        Text(
          subtitle!,
          style: TextStyle(fontSize: 13, height: 1.5, color: color.textDim),
        ),
      ],
      const SizedBox(height: 24),
      child,
    ],
  );
}

class _ConnectionChoice extends StatelessWidget {
  final ThemeColors color;
  final bool selected;
  final IconData icon;
  final String title, description;
  final VoidCallback onTap;
  const _ConnectionChoice({
    super.key,
    required this.color,
    required this.selected,
    required this.icon,
    required this.title,
    required this.description,
    required this.onTap,
  });
  @override
  Widget build(BuildContext context) => Semantics(
    selected: selected,
    button: true,
    child: Material(
      color: selected ? MemoTheme.accent.withValues(alpha: .10) : color.bgApp,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: BorderSide(color: selected ? MemoTheme.accent : color.borderSoft),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(icon, color: MemoTheme.accent, size: 24),
                  const Spacer(),
                  Icon(
                    selected ? Icons.check_circle : Icons.circle_outlined,
                    size: 18,
                    color: selected ? MemoTheme.accent : color.textDim,
                  ),
                ],
              ),
              const SizedBox(height: 14),
              Text(
                title,
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: color.textMain,
                ),
              ),
              const SizedBox(height: 6),
              Text(
                description,
                style: TextStyle(
                  fontSize: 12,
                  height: 1.5,
                  color: color.textDim,
                ),
              ),
            ],
          ),
        ),
      ),
    ),
  );
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
    return Semantics(
      selected: selected,
      child: OutlinedButton(
        onPressed: onTap,
        style: OutlinedButton.styleFrom(
          backgroundColor: selected
              ? accent.withValues(alpha: .10)
              : color.bgElement,
          foregroundColor: selected ? accent : color.textSecondary,
          side: BorderSide(color: selected ? accent : color.borderSoft),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(10),
          ),
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        ),
        child: Text(
          label,
          style: TextStyle(
            fontSize: 13,
            fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
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
/// The setup wizard's switch for the anonymous daily summary: on by default in
/// an official build, switched off right here or later in Settings › Privacy.
/// Hidden while the backend cannot be asked (null status) - a switch that cannot
/// save would lie about its state.
class SetupTelemetryToggle extends ConsumerWidget {
  const SetupTelemetryToggle({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final tel = ref.watch(telemetryStatusProvider).valueOrNull;
    if (tel == null) return const SizedBox.shrink();
    final c = MemoTheme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: 10),
        _PreferenceToggle(
          icon: Icons.insights_outlined,
          title: L10n.t('setup_pref_telemetry_title'),
          desc: L10n.t('setup_pref_telemetry_desc'),
          value: tel.sharing,
          color: c,
          onChanged: tel.envOff
              ? null
              : (v) async {
                  try {
                    await ref.read(telemetryActionsProvider).answer(v);
                  } catch (e) {
                    debugPrint('setup_wizard: telemetry answer: $e');
                  }
                },
        ),
        TextButton(
          key: const Key('setupTelemetryDetails'),
          onPressed: () => showDialog<void>(
            context: context,
            builder: (dialogContext) => AlertDialog(
              title: Text(L10n.t('setup_pref_telemetry_title')),
              content: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 520),
                child: const SingleChildScrollView(child: TelemetryDetails()),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.of(dialogContext).pop(),
                  child: Text(L10n.t('close')),
                ),
              ],
            ),
          ),
          child: Text(L10n.t('setup_pref_telemetry_details')),
        ),
      ],
    );
  }
}

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
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: color.textMain,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  desc,
                  style: TextStyle(
                    fontSize: 11.5,
                    height: 1.4,
                    color: color.textDim,
                  ),
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
  });

  @override
  Widget build(BuildContext context) {
    return _Card(
      color: color,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [_content()],
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
            child: CircularProgressIndicator(
              strokeWidth: 2,
              color: color.textDim,
            ),
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
    final ramLine = gpu.ramTotalMb > 0
        ? gpu.ramFormatted
        : L10n.t('setup_model_ram_unknown');

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          L10n.t('setup_local_recommendation'),
          style: TextStyle(
            fontSize: 13,
            fontWeight: FontWeight.w600,
            color: color.textMain,
          ),
        ),
        SizedBox(height: 10),
        _SpecRow(
          label: L10n.t('setup_model_ram'),
          value: ramLine,
          color: color,
        ),
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
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10),
                ),
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
              Icon(
                Icons.check_circle_rounded,
                color: MemoTheme.green,
                size: 18,
              ),
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
                child: Text(
                  label,
                  style: TextStyle(fontSize: 12, color: color.textDim),
                ),
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

  const _DownloadProgressRow({required this.progress, required this.color});

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
    final label = p.filename.isNotEmpty
        ? p.filename
        : L10n.t('setup_preparing');
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
  final bool optional;
  final ThemeColors color;
  final String? tooltip;

  const _DiagRow({
    required this.title,
    required this.ok,
    this.optional = false,
    required this.color,
    this.tooltip,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(
          ok
              ? Icons.check_circle_rounded
              : optional
              ? Icons.remove_circle_outline
              : Icons.error_outline_rounded,
          color: ok
              ? MemoTheme.green
              : optional
              ? color.textDim
              : MemoTheme.red,
          size: 20,
        ),
        SizedBox(width: 10),
        Expanded(
          child: Text(
            title,
            style: TextStyle(
              fontSize: 13,
              height: 1.4,
              color: ok
                  ? color.textMain
                  : optional
                  ? color.textDim
                  : MemoTheme.red,
            ),
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
