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
/// Layout is a 3-panel LM Studio-style arrangement (reference | models+logs
/// | settings) above ~900px, collapsing to a single scrolling column below
/// that — see _wide/_narrow.
class DeveloperScreen extends ConsumerWidget {
  const DeveloperScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    final configAsync = ref.watch(devGatewayConfigProvider);
    final modelsAsync = ref.watch(gatewayModelsProvider);
    final baseUrl = ref.watch(apiClientProvider).baseUrl;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _TopBar(baseUrl: baseUrl),
        Divider(height: 1, color: theme.borderSoft),
        Expanded(
          child: LayoutBuilder(
            builder: (context, constraints) {
              if (constraints.maxWidth < 900) {
                return _narrow(context, baseUrl, modelsAsync, configAsync);
              }
              return _wide(context, baseUrl, modelsAsync, configAsync);
            },
          ),
        ),
      ],
    );
  }

  Widget _wide(
    BuildContext context,
    String baseUrl,
    AsyncValue<List<GatewayModel>> modelsAsync,
    AsyncValue<DevGatewayConfig> configAsync,
  ) {
    final theme = MemoTheme.of(context);
    return Row(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SizedBox(
          width: 280,
          child: ListView(
            padding: const EdgeInsets.all(20),
            children: [_ReferencePanel(baseUrl: baseUrl)],
          ),
        ),
        VerticalDivider(width: 1, color: theme.borderSoft),
        Expanded(
          child: ListView(
            padding: const EdgeInsets.all(24),
            children: [
              _ModelsPanel(modelsAsync: modelsAsync),
              const SizedBox(height: 28),
              const _LogSection(),
            ],
          ),
        ),
        VerticalDivider(width: 1, color: theme.borderSoft),
        SizedBox(
          width: 320,
          child: ListView(
            padding: const EdgeInsets.all(20),
            children: [
              configAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (e, _) => Text(
                  '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
                  style: TextStyle(color: MemoTheme.red),
                ),
                data: (config) => _SettingsPanel(config: config),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _narrow(
    BuildContext context,
    String baseUrl,
    AsyncValue<List<GatewayModel>> modelsAsync,
    AsyncValue<DevGatewayConfig> configAsync,
  ) {
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        _ReferencePanel(baseUrl: baseUrl),
        const SizedBox(height: 28),
        _ModelsPanel(modelsAsync: modelsAsync),
        const SizedBox(height: 28),
        configAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Text(
            '${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}',
            style: TextStyle(color: MemoTheme.red),
          ),
          data: (config) => _SettingsPanel(config: config),
        ),
        const SizedBox(height: 28),
        const _LogSection(),
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
  const _TopBar({required this.baseUrl});

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
      child: Wrap(
        crossAxisAlignment: WrapCrossAlignment.center,
        spacing: 20,
        runSpacing: 8,
        children: [
          Row(
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
              const SizedBox(width: 16),
              Text(
                L10n.t('dev_gateway_title'),
                style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold, color: theme.textMain),
              ),
            ],
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
    );
  }
}

/// Left panel: a compact, always-accurate local reference for the gateway's
/// one real endpoint (deliberately not a fetched copy of memocpp.com's
/// docs — see handoff.md: this stays available offline, and a 1-endpoint
/// surface doesn't need a whole docs tree).
class _ReferencePanel extends StatelessWidget {
  final String baseUrl;
  const _ReferencePanel({required this.baseUrl});

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
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(
            color: theme.bgElement,
            borderRadius: BorderRadius.circular(10),
            border: Border.all(color: theme.borderSoft),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  const _MethodBadge(method: 'POST'),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      '/v1/messages',
                      style: const TextStyle(fontFamily: 'JetBrainsMono', fontSize: 12, color: MemoTheme.accent),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              Text(
                L10n.t('dev_gateway_reference_messages_desc'),
                style: TextStyle(fontSize: 11, color: theme.textDim, height: 1.4),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),
        Text(
          L10n.t('dev_gateway_base_url_hint'),
          style: TextStyle(color: theme.textDim, fontSize: 11),
        ),
        const SizedBox(height: 4),
        copyableValueBox(context, 'ANTHROPIC_BASE_URL=$baseUrl', monospace: true),
      ],
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

/// Center panel: every model currently reachable through the gateway (one
/// entry per enabled provider + the running local model, if any) — the
/// "Loaded Models" equivalent, except Memo has no separate load/unload step
/// for the gateway itself, so this just reflects whatever's already active.
class _ModelsPanel extends StatelessWidget {
  final AsyncValue<List<GatewayModel>> modelsAsync;
  const _ModelsPanel({required this.modelsAsync});

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

/// Right panel: per-request-shaping settings — the LM Studio "selected
/// model settings" equivalent, except these apply to every gateway request
/// rather than one loaded model.
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

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        sectionLabel(context, L10n.t('dev_gateway_settings_title')),
        const SizedBox(height: 12),
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
        const SizedBox(height: 16),
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
        const SizedBox(height: 20),
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
              fillColor: theme.bgElement,
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
