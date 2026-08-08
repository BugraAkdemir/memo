import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/dev_gateway.dart';
import '../providers/chat_provider.dart';
import '../providers/settings_provider.dart';
import '../core/friendly_error.dart';

/// Developer screen (NavRail): the local Anthropic/OpenAI-compatible API
/// gateway's Base URL/model list/settings, plus a live log of requests
/// passing through it. Moved out of the Settings dialog into its own
/// top-level screen (like WhatsApp/Routines) so it can show a live-updating
/// log without needing the Settings dialog to stay open.
class DeveloperScreen extends ConsumerWidget {
  const DeveloperScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    final configAsync = ref.watch(devGatewayConfigProvider);
    final modelsAsync = ref.watch(gatewayModelsProvider);
    final baseUrl = ref.watch(apiClientProvider).baseUrl;

    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('dev_gateway_title'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: theme.textMain,
          ),
        ),
        SizedBox(height: 8),
        Text(
          L10n.t('dev_gateway_subtitle'),
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        SizedBox(height: 24),

        _label(context, L10n.t('dev_gateway_base_url_label')),
        SizedBox(height: 6),
        _copyableValueBox(context, baseUrl),
        SizedBox(height: 6),
        Text(
          L10n.t('dev_gateway_base_url_hint'),
          style: TextStyle(color: theme.textDim, fontSize: 11),
        ),
        SizedBox(height: 4),
        _copyableValueBox(context, 'ANTHROPIC_BASE_URL=$baseUrl', monospace: true),
        SizedBox(height: 24),

        _label(context, L10n.t('dev_gateway_models_title')),
        SizedBox(height: 6),
        Text(
          L10n.t('dev_gateway_models_hint'),
          style: TextStyle(color: theme.textDim, fontSize: 11),
        ),
        SizedBox(height: 8),
        modelsAsync.when(
          loading: () => Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}', style: TextStyle(color: MemoTheme.red)),
          data: (models) {
            if (models.isEmpty) {
              return Container(
                padding: EdgeInsets.all(14),
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
            return Column(
              children: [
                for (final m in models) ...[
                  _copyableValueBox(context, m.id, monospace: true),
                  SizedBox(height: 6),
                ],
              ],
            );
          },
        ),
        SizedBox(height: 24),

        configAsync.when(
          loading: () => Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: ${FriendlyError.describeGeneric(e)}', style: TextStyle(color: MemoTheme.red)),
          data: (config) => _ConfigSection(config: config),
        ),
        SizedBox(height: 32),

        const _LogSection(),
      ],
    );
  }

  Widget _label(BuildContext context, String s) {
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

  Widget _copyableValueBox(BuildContext context, String value, {bool monospace = false}) {
    final theme = MemoTheme.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.borderSoft),
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
            onTap: () {
              Clipboard.setData(ClipboardData(text: value));
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text(L10n.t('copied'))),
              );
            },
            child: Icon(Icons.copy_rounded, size: 18, color: theme.textDim),
          ),
        ],
      ),
    );
  }
}

class _ConfigSection extends ConsumerWidget {
  final DevGatewayConfig config;
  const _ConfigSection({required this.config});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);

    Future<void> update({bool? requireAPIKey, bool? useMemory}) async {
      try {
        await ref.read(devGatewayConfigProvider.notifier).save(
              requireAPIKey: requireAPIKey ?? config.requireAPIKey,
              useMemory: useMemory ?? config.useMemory,
            );
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(L10n.t('dev_gateway_save_error', {'e': FriendlyError.describeGeneric(e)}))),
          );
        }
      }
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
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
          onChanged: (v) => update(requireAPIKey: v),
          dense: true,
          contentPadding: EdgeInsets.zero,
          activeThumbColor: MemoTheme.accent,
        ),
        if (config.requireAPIKey) ...[
          SizedBox(height: 8),
          Text(
            L10n.t('dev_gateway_token_label'),
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w600,
              color: theme.textDim,
              letterSpacing: 1.2,
            ),
          ),
          SizedBox(height: 6),
          Builder(
            builder: (context) => Container(
              width: double.infinity,
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
              decoration: BoxDecoration(
                color: theme.bgElement,
                borderRadius: BorderRadius.circular(10),
                border: Border.all(color: MemoTheme.accent),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      config.token,
                      style: TextStyle(fontFamily: 'JetBrainsMono', fontSize: 13, color: MemoTheme.accent),
                    ),
                  ),
                  GestureDetector(
                    onTap: () {
                      Clipboard.setData(ClipboardData(text: config.token));
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(L10n.t('copied'))),
                      );
                    },
                    child: Icon(Icons.copy_rounded, size: 18, color: theme.textDim),
                  ),
                ],
              ),
            ),
          ),
        ],
        SizedBox(height: 12),
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
          onChanged: (v) => update(useMemory: v),
          dense: true,
          contentPadding: EdgeInsets.zero,
          activeThumbColor: MemoTheme.accent,
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
