import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../models/dev_gateway.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/settings_provider.dart';

class DevGatewayTab extends ConsumerWidget {
  const DevGatewayTab({super.key});

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
          error: (e, _) => Text('${L10n.t('error')}: $e', style: TextStyle(color: MemoTheme.red)),
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
          error: (e, _) => Text('${L10n.t('error')}: $e', style: TextStyle(color: MemoTheme.red)),
          data: (config) => _ConfigSection(config: config),
        ),
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
            SnackBar(content: Text(L10n.t('dev_gateway_save_error', {'e': '$e'}))),
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
