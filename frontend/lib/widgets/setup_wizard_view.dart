import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/friendly_error.dart';
import '../core/l10n.dart';
import '../core/notification_service.dart';
import '../core/platform_capabilities.dart';
import '../core/theme.dart';
import '../models/curated_models.dart';
import '../models/persona_presets.dart';
import '../providers/auth_gate_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/mail_provider.dart';
import '../providers/provider_provider.dart';
import '../providers/settings_provider.dart';
import 'persona_picker.dart';
import 'provider_config_dialog.dart';
import 'setup/channels_step.dart';
import 'setup/decision_step.dart';
import 'setup/embedding_step.dart';
import 'setup/local_model_step.dart';
import 'setup/mail_model_step.dart';
import 'setup/mail_step.dart';
import 'setup/memory_step.dart';
import 'setup/privacy_step.dart';
import 'setup/model_store_dialog.dart';
import 'setup/setup_progress.dart';
import 'setup/setup_widgets.dart';
import 'setup/summary_step.dart';
import 'setup/voice_image_step.dart';
import 'settings/tabs/subscriptions_tab.dart';

export 'setup/telemetry_toggle.dart';

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

  /// True when this run was started again from Settings. The persona is then
  /// only written if the user touched it here, so a custom prompt survives.
  bool _isRerun = false;
  bool _personaTouched = false;
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
  bool _replyModelSet = false;

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
    _isRerun = ref.read(prefsProvider).getBool('memo_setup_rerun') ?? false;
    // A re-run starts from the theme in use, not from the first-run default.
    if (_isRerun) _selectedTheme = ref.read(themeModeProvider);
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
        try {
          final twin = await ref.read(apiClientProvider).getTwinState();
          _replyModelSet =
              '${twin.replyModel['provider'] ?? ''}'.isNotEmpty &&
              '${twin.replyModel['model'] ?? ''}'.isNotEmpty;
        } catch (_) {
          _replyModelSet = false;
        }
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
  /// Opens the Model Store window and re-reads what is installed once it closes.
  Future<void> _openStore() async {
    await showSetupModelStore(context);
    if (mounted) await _checkDiagnostics();
  }

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
      if (!_isRerun || _personaTouched) {
        await ref.read(systemPromptProvider.notifier).save(_finalPrompt);
      }
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

  /// What the wizard knows about the app right now.
  SetupSnapshot _snapshotFor(int mailAccounts) => SetupSnapshot(
    hasActiveProvider: _providerConfigured,
    localChatModels: _localModelCount,
    hasEmbeddingModel: _hasEmbeddingModel,
    mailAccounts: mailAccounts,
    mailReplyModelSet: _replyModelSet,
  );

  SetupSnapshot get _snapshot =>
      _snapshotFor(ref.read(mailAccountsProvider).valueOrNull?.length ?? 0);

  /// Steps that have a page. The rules know them all (setup_progress.dart); the
  /// rest join as their pages are built.
  static const _implemented = {
    SetupStepId.languageTheme,
    SetupStepId.model,
    SetupStepId.embedding,
    SetupStepId.persona,
    SetupStepId.mail,
    SetupStepId.mailModel,
    SetupStepId.decision,
    SetupStepId.memory,
    SetupStepId.voiceImage,
    SetupStepId.channels,
    SetupStepId.permissions,
    SetupStepId.summary,
  };

  /// The pages to show now: the reply-model page only exists while a mail
  /// account is connected.
  List<SetupStepId> _stepsFor(SetupSnapshot s) => [
    for (final id in visibleSteps(s))
      if (_implemented.contains(id)) id,
  ];

  /// Steps the user has opened; the summary counts a reviewed optional step.
  final Set<SetupStepId> _visited = {SetupStepId.languageTheme};

  int _page = 0;
  String _connection = 'local';
  bool _connectingSubscription = false;
  String? _subscriptionError;
  final _pageScroll = ScrollController();

  void _goTo(int page) {
    setState(() => _page = page);
    if (_pageScroll.hasClients) _pageScroll.jumpTo(0);
    final steps = _stepsFor(_snapshot);
    if (page >= 0 && page < steps.length) _visited.add(steps[page]);
    if (page < steps.length && steps[page] == SetupStepId.summary) {
      _checkDiagnostics();
    }
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
          // Which step each page is, so required steps can be judged from the
          // real state (setup_progress.dart) instead of the page number.
          final snapshot = _snapshotFor(
            ref.watch(mailAccountsProvider).valueOrNull?.length ?? 0,
          );
          final pageSteps = _stepsFor(snapshot);
          final missing = missingRequiredSteps(snapshot);
          final canFinish = canFinishSetup(snapshot);
          // A required step must be done before moving past it.
          final blocksNext =
              _page < pageSteps.length &&
              stepState(pageSteps[_page], snapshot) ==
                  SetupStepState.requiredTodo;
          // Warn about a required step only once the user has reached it, so the
          // list does not open with an alarm; before that the * is enough.
          bool needsAttention(int i) =>
              i <= _page &&
              stepState(pageSteps[i], snapshot) == SetupStepState.requiredTodo;
          String labelOf(SetupStepId id) => L10n.t(switch (id) {
            SetupStepId.languageTheme => 'setup_step_language_theme',
            SetupStepId.model => 'setup_step_model',
            SetupStepId.embedding => 'setup_step_embedding',
            SetupStepId.persona => 'setup_step_persona',
            SetupStepId.mail => 'setup_step_mail',
            SetupStepId.mailModel => 'setup_step_mail_model',
            SetupStepId.decision => 'setup_step_decision',
            SetupStepId.voiceImage => 'setup_step_voice_image',
            SetupStepId.channels => 'setup_step_channels',
            SetupStepId.permissions => 'setup_step_permissions',
            SetupStepId.memory => 'setup_step_preferences',
            _ => 'setup_step_check',
          });
          final baseLabels = [for (final id in pageSteps) labelOf(id)];
          // A trailing * marks what cannot be skipped.
          final labels = [
            for (var i = 0; i < baseLabels.length; i++)
              requiredSteps(snapshot).contains(pageSteps[i])
                  ? '${baseLabels[i]} *'
                  : baseLabels[i],
          ];
          final builders = <SetupStepId, Widget Function()>{
            SetupStepId.languageTheme: () => SetupPage(
              title: L10n.t('setup_step_language_theme'),
              color: c,
              child: Builder(
                builder: (context) {
                  final languageCard = SetupCard(
                    color: c,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        SetupLabel(text: L10n.t('language'), color: c),
                        SizedBox(height: 10),
                        SetupPill(
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
                        SetupPill(
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
                  final themeCard = SetupCard(
                    color: c,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        SetupLabel(text: L10n.t('theme'), color: c),
                        SizedBox(height: 10),
                        SetupPill(
                          label: L10n.t('theme_dark'),
                          selected: _selectedTheme == 'dark',
                          accent: MemoTheme.accent,
                          color: c,
                          onTap: () => setState(() => _selectedTheme = 'dark'),
                        ),
                        SizedBox(height: 6),
                        SetupPill(
                          label: L10n.t('theme_light'),
                          selected: _selectedTheme == 'light',
                          accent: MemoTheme.accent,
                          color: c,
                          onTap: () => setState(() => _selectedTheme = 'light'),
                        ),
                        SizedBox(height: 6),
                        SetupPill(
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
            SetupStepId.model: () => SetupPage(
              title: L10n.t('setup_step_model'),
              subtitle: L10n.t('setup_connection_intro'),
              color: c,
              child: _buildConnection(c),
            ),
            SetupStepId.embedding: () => SetupPage(
              title: L10n.t('setup_step_embedding'),
              subtitle: L10n.t('setup_embedding_desc'),
              color: c,
              child: SetupEmbeddingStep(
                color: c,
                hasModel: _hasEmbeddingModel,
                downloading: _memoryModelDownloading,
                error: _memoryModelDownloadError,
                onDownloadRecommended: _downloadMemoryModel,
                onOpenStore: _openStore,
              ),
            ),
            SetupStepId.persona: () => SetupPage(
              title: L10n.t('setup_step_persona'),
              subtitle: L10n.t('setup_step_persona_desc'),
              color: c,
              child: PersonaPicker(
                selectedKey: _customPrompt ? 'custom' : _selectedPrompt,
                nameController: _nameController,
                customController: _customPromptController,
                onSelect: (key) => setState(() {
                  _personaTouched = true;
                  if (key == 'custom') {
                    _customPrompt = true;
                  } else {
                    _customPrompt = false;
                    _selectedPrompt = key;
                  }
                }),
                onNameChanged: (_) => _personaTouched = true,
                onCustomTextChanged: (_) => _personaTouched = true,
              ),
            ),
            SetupStepId.mail: () => SetupPage(
              title: L10n.t('setup_step_mail'),
              subtitle: L10n.t('setup_step_mail_desc'),
              color: c,
              child: SetupMailStep(color: c),
            ),
            SetupStepId.mailModel: () => SetupPage(
              title: L10n.t('setup_step_mail_model'),
              subtitle: L10n.t('setup_step_mail_model_desc'),
              color: c,
              child: SetupMailModelStep(
                color: c,
                modelSet: _replyModelSet,
                onModelChanged: (v) => setState(() => _replyModelSet = v),
              ),
            ),
            SetupStepId.decision: () => SetupPage(
              title: L10n.t('setup_step_decision'),
              subtitle: L10n.t('setup_step_decision_desc'),
              color: c,
              child: SetupDecisionStep(
                color: c,
                mailConnected: snapshot.hasMail,
              ),
            ),
            SetupStepId.memory: () => SetupPage(
              title: L10n.t('setup_step_preferences'),
              subtitle: L10n.t('setup_step_preferences_desc'),
              color: c,
              child: Column(
                children: [
                  SetupMemoryStep(
                    color: c,
                    hasEmbeddingModel: _hasEmbeddingModel,
                  ),
                ],
              ),
            ),
            SetupStepId.voiceImage: () => SetupPage(
              title: L10n.t('setup_step_voice_image'),
              subtitle: L10n.t('setup_step_voice_image_desc'),
              color: c,
              child: SetupVoiceImageStep(color: c),
            ),
            SetupStepId.channels: () => SetupPage(
              title: L10n.t('setup_step_channels'),
              subtitle: L10n.t('setup_step_channels_desc'),
              color: c,
              child: SetupChannelsStep(color: c),
            ),
            SetupStepId.permissions: () => SetupPage(
              title: L10n.t('setup_step_permissions'),
              subtitle: L10n.t('setup_step_permissions_desc'),
              color: c,
              child: SetupPrivacyStep(
                color: c,
                notificationsGranted: notificationsSupported
                    ? _notifPermissionGranted
                    : null,
                notificationsAsking: _notifPermissionAsking,
                onAskNotifications: _askNotificationPermission,
              ),
            ),
            SetupStepId.summary: () => SetupPage(
              title: L10n.t('setup_step_check'),
              color: c,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  SetupSummaryStep(
                    color: c,
                    steps: pageSteps,
                    snapshot: snapshot,
                    visited: _visited,
                    labelOf: labelOf,
                    onGoTo: (id) => _goTo(pageSteps.indexOf(id)),
                  ),
                  const SizedBox(height: 12),
                  SetupCard(
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
                        SetupDiagRow(
                          title: L10n.t('backend_connection'),
                          ok: _backendOk,
                          color: c,
                          tooltip: L10n.t('setup_check_backend_tooltip'),
                        ),
                        SizedBox(height: 8),
                        SetupDiagRow(
                          title: L10n.t('local_models_status'),
                          ok: _modelsOk,
                          optional: _providerConfigured,
                          color: c,
                          tooltip: L10n.t('setup_check_models_tooltip'),
                        ),
                        SizedBox(height: 8),
                        SetupDiagRow(
                          title: L10n.t('setup_check_ready'),
                          ok: _modelsOk || _providerConfigured,
                          color: c,
                        ),
                        if (missing.isNotEmpty) ...[
                          SizedBox(height: 12),
                          Container(
                            key: const Key('setup_missing_required'),
                            padding: EdgeInsets.all(10),
                            decoration: BoxDecoration(
                              color: MemoTheme.warningOrange.withValues(
                                alpha: 0.08,
                              ),
                              borderRadius: BorderRadius.circular(8),
                            ),
                            child: Text(
                              L10n.t('setup_missing_required', {
                                'steps': missing
                                    .where(pageSteps.contains)
                                    .map(
                                      (s) => baseLabels[pageSteps.indexOf(s)],
                                    )
                                    .join(', '),
                              }),
                              style: TextStyle(
                                fontSize: 12,
                                color: MemoTheme.warningOrange,
                                height: 1.3,
                              ),
                            ),
                          ),
                        ],
                        if (!_backendOk ||
                            !(_modelsOk || _providerConfigured)) ...[
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
                ],
              ),
            ),
          };
          final pages = [for (final id in pageSteps) builders[id]!];
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
                                      Expanded(
                                        child: SingleChildScrollView(
                                          child: Column(
                                            children: [
                                              for (
                                                var i = 0;
                                                i < labels.length;
                                                i++
                                              )
                                                Padding(
                                                  padding:
                                                      const EdgeInsets.only(
                                                        bottom: 8,
                                                      ),
                                                  child: ListTile(
                                                    contentPadding:
                                                        const EdgeInsets.symmetric(
                                                          horizontal: 8,
                                                        ),
                                                    shape: RoundedRectangleBorder(
                                                      borderRadius:
                                                          BorderRadius.circular(
                                                            10,
                                                          ),
                                                    ),
                                                    selected: i == _page,
                                                    selectedTileColor: MemoTheme
                                                        .accent
                                                        .withValues(alpha: .12),
                                                    leading: Icon(
                                                      needsAttention(i)
                                                          ? Icons.error_outline
                                                          : (i < _page ||
                                                                stepState(
                                                                      pageSteps[i],
                                                                      snapshot,
                                                                    ) ==
                                                                    SetupStepState
                                                                        .done)
                                                          ? Icons
                                                                .check_circle_outline
                                                          : Icons
                                                                .circle_outlined,
                                                      color: needsAttention(i)
                                                          ? MemoTheme
                                                                .warningOrange
                                                          : i == _page
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
                                            ],
                                          ),
                                        ),
                                      ),
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
                                        onPressed: _saving || blocksNext
                                            ? null
                                            : _page == pages.length - 1
                                            ? (canFinish ? _saveSetup : null)
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
                    child: SetupConnectionChoice(
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
          SetupLocalModelCard(
            color: c,
            checking: _checking,
            chatModelCount: _localModelCount,
            onOpenStore: _openStore,
          ),
        if (_connection == 'api')
          SetupCard(
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
