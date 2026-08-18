import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/dev_gateway.dart';
import '../providers/chat_provider.dart';
import '../providers/settings_provider.dart';
import '../core/friendly_error.dart';

/// Developer screen (NavRail): the local Anthropic-compatible API gateway's
/// status/reference/model list, plus a live log of requests passing through
/// it and its settings. Moved out of the Settings dialog into its own
/// top-level screen (like WhatsApp/Routines) so it can show a live-updating
/// log without needing the Settings dialog to stay open.
///
/// Layout mirrors LM Studio's own Developer page: a docs-tree-style nav
/// sidebar on the left (jumps to sections rather than owning separate
/// pages — this screen only has one real endpoint, so a full multi-page
/// docs site would be mostly empty) and a single scrolling content column
/// on the right. Sidebar hides below ~760px — see build().
class DeveloperScreen extends ConsumerStatefulWidget {
  const DeveloperScreen({super.key});

  @override
  ConsumerState<DeveloperScreen> createState() => _DeveloperScreenState();
}

class _DeveloperScreenState extends ConsumerState<DeveloperScreen> {
  final _referenceKey = GlobalKey();
  final _modelsKey = GlobalKey();
  final _settingsKey = GlobalKey();
  final _logKey = GlobalKey();

  void _scrollTo(GlobalKey key) {
    final ctx = key.currentContext;
    if (ctx != null) {
      Scrollable.ensureVisible(ctx, duration: const Duration(milliseconds: 300), curve: Curves.easeOut);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final configAsync = ref.watch(devGatewayConfigProvider);
    final modelsAsync = ref.watch(gatewayModelsProvider);
    final baseUrl = ref.watch(apiClientProvider).baseUrl;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _TopBar(baseUrl: baseUrl, onSettingsTap: () => _scrollTo(_settingsKey)),
        Divider(height: 1, color: theme.borderSoft),
        Expanded(
          child: LayoutBuilder(
            builder: (context, constraints) {
              final showSidebar = constraints.maxWidth >= 760;
              return Row(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (showSidebar) ...[
                    _NavSidebar(
                      onTapReference: () => _scrollTo(_referenceKey),
                      onTapModels: () => _scrollTo(_modelsKey),
                      onTapSettings: () => _scrollTo(_settingsKey),
                      onTapLog: () => _scrollTo(_logKey),
                    ),
                    VerticalDivider(width: 1, color: theme.borderSoft),
                  ],
                  Expanded(
                    child: ListView(
                      padding: const EdgeInsets.all(24),
                      children: [
                        _ReferenceSection(key: _referenceKey, baseUrl: baseUrl),
                        const SizedBox(height: 32),
                        _ModelsPanel(key: _modelsKey, modelsAsync: modelsAsync),
                        const SizedBox(height: 32),
                        KeyedSubtree(
                          key: _settingsKey,
                          child: configAsync.when(
                            loading: () => const Center(child: CircularProgressIndicator()),
                            error: (e, _) => Text(
                              '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
                              style: TextStyle(color: MemoTheme.red),
                            ),
                            data: (config) => _SettingsPanel(config: config),
                          ),
                        ),
                        const SizedBox(height: 32),
                        KeyedSubtree(key: _logKey, child: const _LogSection()),
                      ],
                    ),
                  ),
                ],
              );
            },
          ),
        ),
      ],
    );
  }
}

Widget sectionLabel(BuildContext context, String s) {
  return Text(
    s,
    style: TextStyle(
      fontSize: 11,
      fontWeight: FontWeight.w600,
      color: MemoTheme.of(context).textDim,
      letterSpacing: 1.2,
    ),
  );
}

void copyToClipboard(BuildContext context, String value) {
  Clipboard.setData(ClipboardData(text: value));
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(content: Text(L10n.t('copied'))),
  );
}

Widget copyableValueBox(BuildContext context, String value, {bool monospace = false, Color? borderColor}) {
  final theme = MemoTheme.of(context);
  return Container(
    width: double.infinity,
    padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
    decoration: BoxDecoration(
      color: theme.bgElement,
      borderRadius: BorderRadius.circular(10),
      border: Border.all(color: borderColor ?? theme.borderSoft),
    ),
    child: Row(
      children: [
        Expanded(
          child: Text(
            value,
            style: TextStyle(
              fontFamily: monospace ? 'JetBrainsMono' : null,
              fontSize: 13,
              color: MemoTheme.accent,
            ),
          ),
        ),
        GestureDetector(
          onTap: () => copyToClipboard(context, value),
          child: Icon(Icons.copy_rounded, size: 18, color: theme.textDim),
        ),
      ],
    ),
  );
}

/// Slim status strip across the top of the screen — mirrors the "Status" +
/// address bar LM Studio's own Developer page shows, minus a start/stop
/// toggle: the gateway has no separate on/off state of its own, it's just
/// routes on the same server the whole app already needs running, so a fake
/// toggle here would be misleading.
class _TopBar extends StatelessWidget {
  final String baseUrl;
  final VoidCallback onSettingsTap;
  const _TopBar({required this.baseUrl, required this.onSettingsTap});

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    // Two independently-wrapping groups laid out via spaceBetween, NOT a
    // single Wrap with a Spacer in the middle — Spacer wraps Expanded,
    // which requires a direct Flex (Row/Column) ancestor; Wrap isn't one,
    // so that combination throws at runtime (RenderFlex/ParentDataWidget
    // error) and blanks the whole screen in a release build, where the
    // error widget renders as an empty box with no visible message.
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
      child: Row(
        children: [
          Flexible(
            child: Wrap(
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: 12,
              runSpacing: 8,
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                  decoration: BoxDecoration(
                    color: theme.bgElement,
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: theme.borderSoft),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Container(
                        width: 8,
                        height: 8,
                        decoration: const BoxDecoration(color: MemoTheme.green, shape: BoxShape.circle),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        L10n.t('dev_gateway_status_active'),
                        style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: theme.textMain),
                      ),
                    ],
                  ),
                ),
                Material(
                  color: theme.bgElement,
                  borderRadius: BorderRadius.circular(8),
                  child: InkWell(
                    onTap: onSettingsTap,
                    borderRadius: BorderRadius.circular(8),
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                      decoration: BoxDecoration(borderRadius: BorderRadius.circular(8), border: Border.all(color: theme.borderSoft)),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(Icons.tune_rounded, size: 14, color: theme.textDim),
                          const SizedBox(width: 6),
                          Text(
                            L10n.t('dev_gateway_settings_title'),
                            style: TextStyle(fontSize: 12, color: theme.textSecondary),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          Flexible(
            child: Wrap(
              alignment: WrapAlignment.end,
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: 12,
              runSpacing: 8,
              children: [
                Text(
                  L10n.t('dev_gateway_title'),
                  style: TextStyle(fontSize: 15, fontWeight: FontWeight.bold, color: theme.textMain),
                ),
                GestureDetector(
                  onTap: () => copyToClipboard(context, baseUrl),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                    decoration: BoxDecoration(
                      color: theme.bgElement,
                      borderRadius: BorderRadius.circular(8),
                      border: Border.all(color: theme.borderSoft),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          baseUrl,
                          style: const TextStyle(fontFamily: 'JetBrainsMono', fontSize: 12, color: MemoTheme.accent),
                        ),
                        const SizedBox(width: 6),
                        Icon(Icons.copy_rounded, size: 14, color: theme.textDim),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Left docs-tree-style nav — jumps the main content list to a section
/// rather than swapping to a separate page (this screen has one real
/// endpoint, so separate pages would mostly be empty). Uses bgApp (the
/// theme's deepest surface level) to read as a distinct rail from the
/// content behind it, in both light and dark mode.
class _NavSidebar extends StatelessWidget {
  final VoidCallback onTapReference;
  final VoidCallback onTapModels;
  final VoidCallback onTapSettings;
  final VoidCallback onTapLog;
  const _NavSidebar({
    required this.onTapReference,
    required this.onTapModels,
    required this.onTapSettings,
    required this.onTapLog,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Container(
      width: 220,
      color: theme.bgApp,
      child: ListView(
        padding: const EdgeInsets.symmetric(vertical: 14),
        children: [
          _navSectionLabel(context, L10n.t('dev_gateway_nav_section')),
          _navItem(context, icon: Icons.dns_rounded, label: L10n.t('dev_gateway_nav_gateway'), active: true),
          const SizedBox(height: 10),
          _navSectionLabel(context, L10n.t('dev_gateway_reference_title')),
          _navSubItem(context, label: '/v1/messages', onTap: onTapReference, method: 'POST'),
          _navSubItem(context, label: '/v1/chat/completions', onTap: onTapReference, method: 'POST'),
          _navSubItem(context, label: '/v1/models', onTap: onTapReference, method: 'GET'),
          const SizedBox(height: 10),
          _navSectionLabel(context, L10n.t('dev_gateway_models_title')),
          _navSubItem(context, label: L10n.t('dev_gateway_models_title'), onTap: onTapModels),
          const SizedBox(height: 10),
          _navSectionLabel(context, L10n.t('dev_gateway_settings_title')),
          _navSubItem(context, label: L10n.t('dev_gateway_require_key_label'), onTap: onTapSettings),
          _navSubItem(context, label: L10n.t('dev_gateway_use_memory_label'), onTap: onTapSettings),
          _navSubItem(context, label: L10n.t('dev_gateway_system_prompt_label'), onTap: onTapSettings),
          const SizedBox(height: 10),
          _navSectionLabel(context, L10n.t('dev_gateway_logs_title')),
          _navSubItem(context, label: L10n.t('dev_gateway_logs_title'), onTap: onTapLog),
        ],
      ),
    );
  }

  Widget _navSectionLabel(BuildContext context, String s) {
    // Deliberately not .toUpperCase() — Dart's default case mapping isn't
    // Turkish-locale-aware (lowercase "i" maps to "I", not the correct
    // Turkish "İ"), so letter-spaced sentence case avoids a subtly wrong
    // Turkish label instead of a fully-capitalized one.
    return Padding(
      padding: const EdgeInsets.fromLTRB(14, 6, 14, 6),
      child: Text(
        s,
        style: TextStyle(fontSize: 10, fontWeight: FontWeight.w700, color: MemoTheme.of(context).textDim, letterSpacing: 0.6),
      ),
    );
  }

  Widget _navItem(BuildContext context, {required IconData icon, required String label, bool active = false}) {
    final theme = MemoTheme.of(context);
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 8),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(
        color: active ? MemoTheme.accentMuted : null,
        border: active ? const Border(left: BorderSide(color: MemoTheme.accent, width: 2)) : null,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Row(
        children: [
          Icon(icon, size: 15, color: active ? MemoTheme.accent : theme.textSecondary),
          const SizedBox(width: 8),
          Text(label, style: TextStyle(fontSize: 13, fontWeight: FontWeight.w500, color: active ? MemoTheme.accent : theme.textMain)),
        ],
      ),
    );
  }

  Widget _navSubItem(BuildContext context, {required String label, required VoidCallback onTap, String? method}) {
    final theme = MemoTheme.of(context);
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(28, 6, 14, 6),
          child: Row(
            children: [
              if (method != null) ...[
                _MethodBadge(method: method),
                const SizedBox(width: 8),
              ],
              Expanded(
                child: Text(
                  label,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 12, color: theme.textSecondary),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Reference card: a compact, always-accurate local reference for the
/// gateway's one real endpoint (deliberately not a fetched copy of
/// memocpp.com's docs — see handoff.md: this stays available offline, and a
/// 1-endpoint surface doesn't need a whole fetched docs tree).
class _ReferenceSection extends StatelessWidget {
  final String baseUrl;
  const _ReferenceSection({super.key, required this.baseUrl});

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        sectionLabel(context, L10n.t('dev_gateway_reference_title')),
        const SizedBox(height: 12),
        Container(
          width: double.infinity,
          decoration: BoxDecoration(
            color: theme.bgElement,
            borderRadius: BorderRadius.circular(10),
            border: Border.all(color: theme.borderSoft),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                decoration: BoxDecoration(border: Border(bottom: BorderSide(color: theme.borderSoft))),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        L10n.t('dev_gateway_reference_title'),
                        style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: theme.textMain),
                      ),
                    ),
                    _apiBadge(L10n.t('dev_gateway_reference_anthropic_badge')),
                    const SizedBox(width: 6),
                    _apiBadge(L10n.t('dev_gateway_reference_openai_badge')),
                  ],
                ),
              ),
              _endpointRow(context, method: 'POST', path: '/v1/messages', desc: L10n.t('dev_gateway_reference_messages_desc')),
              Divider(height: 1, color: theme.borderSoft),
              _endpointRow(context, method: 'POST', path: '/v1/chat/completions', desc: L10n.t('dev_gateway_reference_completions_desc')),
              Divider(height: 1, color: theme.borderSoft),
              _endpointRow(context, method: 'GET', path: '/v1/models', desc: L10n.t('dev_gateway_reference_models_desc')),
            ],
          ),
        ),
        const SizedBox(height: 14),
        Text(
          L10n.t('dev_gateway_base_url_hint'),
          style: TextStyle(color: theme.textDim, fontSize: 11),
        ),
        const SizedBox(height: 4),
        copyableValueBox(context, 'ANTHROPIC_BASE_URL=$baseUrl', monospace: true),
        const SizedBox(height: 8),
        Text(
          L10n.t('dev_gateway_openai_base_url_hint'),
          style: TextStyle(color: theme.textDim, fontSize: 11),
        ),
        const SizedBox(height: 4),
        copyableValueBox(context, 'OPENAI_BASE_URL=$baseUrl/v1', monospace: true),
      ],
    );
  }

  Widget _apiBadge(String label) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(color: MemoTheme.accentMuted, borderRadius: BorderRadius.circular(10)),
      child: Text(
        label,
        style: const TextStyle(fontSize: 10, fontWeight: FontWeight.w600, color: MemoTheme.accent),
      ),
    );
  }

  Widget _endpointRow(BuildContext context, {required String method, required String path, required String desc}) {
    final theme = MemoTheme.of(context);
    return Padding(
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              _MethodBadge(method: method),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  path,
                  style: const TextStyle(fontFamily: 'JetBrainsMono', fontSize: 13, color: MemoTheme.accent),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              Icon(Icons.info_outline_rounded, size: 15, color: theme.textDim),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            desc,
            style: TextStyle(fontSize: 12, color: theme.textDim, height: 1.4),
          ),
        ],
      ),
    );
  }
}

class _MethodBadge extends StatelessWidget {
  final String method;
  const _MethodBadge({required this.method});

  @override
  Widget build(BuildContext context) {
    final color = method == 'GET' ? MemoTheme.green : MemoTheme.accent;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.16),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        method,
        style: TextStyle(fontSize: 10, fontWeight: FontWeight.w700, color: color, letterSpacing: 0.5),
      ),
    );
  }
}

/// Available models — one entry per enabled provider + the running local
/// model, if any. Memo has no separate load/unload step for the gateway
/// itself, so this just reflects whatever's already active.
class _ModelsPanel extends StatelessWidget {
  final AsyncValue<List<GatewayModel>> modelsAsync;
  const _ModelsPanel({super.key, required this.modelsAsync});

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        sectionLabel(context, L10n.t('dev_gateway_models_title')),
        const SizedBox(height: 6),
        Text(
          L10n.t('dev_gateway_models_hint'),
          style: TextStyle(color: theme.textDim, fontSize: 11),
        ),
        const SizedBox(height: 10),
        modelsAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Text(
            '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
            style: TextStyle(color: MemoTheme.red),
          ),
          data: (models) {
            if (models.isEmpty) {
              return Container(
                padding: const EdgeInsets.all(14),
                decoration: BoxDecoration(
                  color: theme.bgElement,
                  borderRadius: BorderRadius.circular(10),
                  border: Border.all(color: theme.borderSoft),
                ),
                child: Text(
                  L10n.t('dev_gateway_models_empty'),
                  style: TextStyle(color: theme.textDim, fontSize: 12),
                ),
              );
            }
            return Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                for (final m in models)
                  GestureDetector(
                    onTap: () => copyToClipboard(context, m.id),
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                      decoration: BoxDecoration(
                        color: theme.bgElement,
                        borderRadius: BorderRadius.circular(8),
                        border: Border.all(color: theme.borderSoft),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            m.id,
                            style: const TextStyle(fontFamily: 'JetBrainsMono', fontSize: 12, color: MemoTheme.accent),
                          ),
                          const SizedBox(width: 6),
                          Icon(Icons.copy_rounded, size: 13, color: theme.textDim),
                        ],
                      ),
                    ),
                  ),
              ],
            );
          },
        ),
      ],
    );
  }
}

/// Gateway settings — Require API Key, Use Memory, and an extra system
/// instruction applied to every request through the gateway.
class _SettingsPanel extends ConsumerStatefulWidget {
  final DevGatewayConfig config;
  const _SettingsPanel({required this.config});

  @override
  ConsumerState<_SettingsPanel> createState() => _SettingsPanelState();
}

class _SettingsPanelState extends ConsumerState<_SettingsPanel> {
  late final TextEditingController _promptController;

  @override
  void initState() {
    super.initState();
    _promptController = TextEditingController(text: widget.config.systemPrompt);
  }

  @override
  void didUpdateWidget(_SettingsPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    // Only resync from the server value if the field isn't mid-edit — a
    // save-in-flight refresh (see save() below) must not clobber keystrokes
    // the user typed after tapping save but before the response landed.
    if (!_promptController.value.composing.isValid && _promptController.text != widget.config.systemPrompt && !_focused) {
      _promptController.text = widget.config.systemPrompt;
    }
  }

  @override
  void dispose() {
    _promptController.dispose();
    super.dispose();
  }

  bool _focused = false;
  bool _saving = false;

  Future<void> _update({bool? requireAPIKey, bool? useMemory, String? systemPrompt}) async {
    setState(() => _saving = true);
    try {
      await ref.read(devGatewayConfigProvider.notifier).save(
            requireAPIKey: requireAPIKey ?? widget.config.requireAPIKey,
            useMemory: useMemory ?? widget.config.useMemory,
            systemPrompt: systemPrompt ?? widget.config.systemPrompt,
          );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('dev_gateway_save_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final config = widget.config;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          sectionLabel(context, L10n.t('dev_gateway_settings_title')),
          const SizedBox(height: 8),
          SwitchListTile(
            title: Text(
              L10n.t('dev_gateway_require_key_label'),
              style: TextStyle(fontSize: 13, color: theme.textMain),
            ),
            subtitle: Text(
              L10n.t('dev_gateway_require_key_desc'),
              style: TextStyle(fontSize: 11, color: theme.textDim),
            ),
            value: config.requireAPIKey,
            onChanged: (v) => _update(requireAPIKey: v),
            dense: true,
            contentPadding: EdgeInsets.zero,
            activeThumbColor: MemoTheme.accent,
          ),
          if (config.requireAPIKey) ...[
            const SizedBox(height: 8),
            sectionLabel(context, L10n.t('dev_gateway_token_label')),
            const SizedBox(height: 6),
            copyableValueBox(context, config.token, monospace: true, borderColor: MemoTheme.accent),
          ],
          const SizedBox(height: 12),
          Divider(height: 1, color: theme.borderSoft),
          const SizedBox(height: 12),
          SwitchListTile(
            title: Text(
              L10n.t('dev_gateway_use_memory_label'),
              style: TextStyle(fontSize: 13, color: theme.textMain),
            ),
            subtitle: Text(
              L10n.t('dev_gateway_use_memory_desc'),
              style: TextStyle(fontSize: 11, color: theme.textDim),
            ),
            value: config.useMemory,
            onChanged: (v) => _update(useMemory: v),
            dense: true,
            contentPadding: EdgeInsets.zero,
            activeThumbColor: MemoTheme.accent,
          ),
          const SizedBox(height: 12),
          Divider(height: 1, color: theme.borderSoft),
          const SizedBox(height: 16),
          sectionLabel(context, L10n.t('dev_gateway_system_prompt_label')),
          const SizedBox(height: 6),
          Text(
            L10n.t('dev_gateway_system_prompt_desc'),
            style: TextStyle(fontSize: 11, color: theme.textDim),
          ),
          const SizedBox(height: 8),
          Focus(
            onFocusChange: (has) => _focused = has,
            child: TextField(
              controller: _promptController,
              maxLines: 4,
              minLines: 3,
              style: TextStyle(fontSize: 13, color: theme.textMain),
              decoration: InputDecoration(
                hintText: L10n.t('dev_gateway_system_prompt_placeholder'),
                hintStyle: TextStyle(fontSize: 12, color: theme.textDim),
                filled: true,
                fillColor: theme.bgApp,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(10),
                  borderSide: BorderSide(color: theme.borderSoft),
                ),
                enabledBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(10),
                  borderSide: BorderSide(color: theme.borderSoft),
                ),
                contentPadding: const EdgeInsets.all(12),
              ),
            ),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.centerRight,
            child: TextButton.icon(
              onPressed: _saving ? null : () => _update(systemPrompt: _promptController.text),
              icon: _saving
                  ? const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.check, size: 16),
              label: Text(L10n.t('save')),
              style: TextButton.styleFrom(foregroundColor: MemoTheme.accent),
            ),
          ),
        ],
      ),
    );
  }
}

class _LogSection extends ConsumerWidget {
  const _LogSection();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    final logsAsync = ref.watch(gatewayLogsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                L10n.t('dev_gateway_logs_title'),
                style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: theme.textMain,
                ),
              ),
            ),
            IconButton(
              tooltip: L10n.t('stats_refresh'),
              icon: Icon(Icons.refresh, size: 18, color: theme.textDim),
              onPressed: () => ref.read(gatewayLogsProvider.notifier).startPolling(),
            ),
          ],
        ),
        SizedBox(height: 4),
        Text(
          L10n.t('dev_gateway_logs_subtitle'),
          style: TextStyle(color: theme.textDim, fontSize: 11),
        ),
        SizedBox(height: 12),
        logsAsync.when(
          loading: () => Center(child: CircularProgressIndicator()),
          error: (e, _) => Text(
            L10n.t('dev_gateway_logs_error', {'e': FriendlyError.describeGeneric(e)}),
            style: TextStyle(color: MemoTheme.red),
          ),
          data: (logs) {
            if (logs.isEmpty) {
              return Container(
                padding: EdgeInsets.all(14),
                decoration: BoxDecoration(
                  color: theme.bgElement,
                  borderRadius: BorderRadius.circular(10),
                  border: Border.all(color: theme.borderSoft),
                ),
                child: Text(
                  L10n.t('dev_gateway_logs_empty'),
                  style: TextStyle(color: theme.textDim, fontSize: 12),
                ),
              );
            }
            // Newest first, so the most recent request is always visible
            // without scrolling.
            final reversed = logs.reversed.toList();
            return Column(
              children: [
                for (final entry in reversed) ...[
                  _LogEntryTile(entry: entry),
                  SizedBox(height: 8),
                ],
              ],
            );
          },
        ),
      ],
    );
  }
}

class _LogEntryTile extends StatelessWidget {
  final GatewayLogEntry entry;
  const _LogEntryTile({required this.entry});

  String _formatTime(String iso) {
    final dt = DateTime.tryParse(iso);
    if (dt == null) return iso;
    final local = dt.toLocal();
    String two(int n) => n.toString().padLeft(2, '0');
    return '${two(local.hour)}:${two(local.minute)}:${two(local.second)}';
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final borderColor = entry.isError ? MemoTheme.red.withValues(alpha: 0.4) : theme.borderSoft;

    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: borderColor),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(
                _formatTime(entry.timestamp),
                style: TextStyle(fontSize: 11, fontFamily: 'JetBrainsMono', color: theme.textDim),
              ),
              SizedBox(width: 8),
              Flexible(
                child: Text(
                  entry.model,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 12, fontFamily: 'JetBrainsMono', fontWeight: FontWeight.w600, color: MemoTheme.accent),
                ),
              ),
              if (entry.stream) ...[
                SizedBox(width: 6),
                _Badge(text: L10n.t('dev_gateway_logs_stream_badge')),
              ],
              if (entry.hasTools) ...[
                SizedBox(width: 6),
                _Badge(text: L10n.t('dev_gateway_logs_tools_badge')),
              ],
              Spacer(),
              Text(
                L10n.t('dev_gateway_logs_duration', {'ms': '${entry.durationMs}'}),
                style: TextStyle(fontSize: 11, color: theme.textDim),
              ),
            ],
          ),
          SizedBox(height: 8),
          _previewRow(context, L10n.t('dev_gateway_logs_request_label'), entry.requestPreview, theme.textMain),
          if (entry.isError) ...[
            SizedBox(height: 4),
            _previewRow(context, L10n.t('dev_gateway_logs_error_label'), entry.error, MemoTheme.red),
          ] else if (entry.responsePreview.isNotEmpty) ...[
            SizedBox(height: 4),
            _previewRow(context, L10n.t('dev_gateway_logs_response_label'), entry.responsePreview, theme.textSecondary),
          ],
        ],
      ),
    );
  }

  Widget _previewRow(BuildContext context, String label, String value, Color color) {
    final theme = MemoTheme.of(context);
    return RichText(
      maxLines: 2,
      overflow: TextOverflow.ellipsis,
      text: TextSpan(
        children: [
          TextSpan(
            text: '$label: ',
            style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: theme.textDim),
          ),
          TextSpan(
            text: value,
            style: TextStyle(fontSize: 12, color: color),
          ),
        ],
      ),
    );
  }
}

class _Badge extends StatelessWidget {
  final String text;
  const _Badge({required this.text});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: MemoTheme.accentMuted,
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        text,
        style: TextStyle(fontSize: 9, color: MemoTheme.accent, fontWeight: FontWeight.w600),
      ),
    );
  }
}
