import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../models/provider_config.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/provider_provider.dart';

/// Settings → CLI Bağlantıları. Checks whether each CLI-backed provider's
/// underlying executable (claude, ...) is actually installed and on PATH —
/// and if so, registers it as a provider automatically (no API key, no
/// model id to type — there's nothing for the user to fill in), so it just
/// shows up in a chat's model picker right after checking. Separate from
/// Providers, which manages regular API providers the user configures by
/// hand.
class CLIConnectionsTab extends ConsumerStatefulWidget {
  const CLIConnectionsTab({super.key});

  @override
  ConsumerState<CLIConnectionsTab> createState() => _CLIConnectionsTabState();
}

class _CLIConnectionsTabState extends ConsumerState<CLIConnectionsTab> {
  final Map<String, Map<String, dynamic>?> _status = {};
  final Set<String> _checking = {};

  static const _clis = [
    ('claude-code-cli', 'Claude Code'),
  ];

  @override
  void initState() {
    super.initState();
    // Auto-check on open — previously required a manual tap every time the
    // tab was reopened, which read as "it forgot" when it was really just
    // never asked again.
    for (final (type, _) in _clis) {
      _check(type);
    }
  }

  Future<void> _check(String cliType) async {
    setState(() => _checking.add(cliType));
    try {
      final res = await ref.read(apiClientProvider).getCLIStatus(cliType);
      if (mounted) setState(() => _status[cliType] = res);
      if (res['installed'] == true) {
        await _ensureProviderRegistered(cliType);
      }
    } catch (e) {
      if (mounted) setState(() => _status[cliType] = {'error': '$e'});
    } finally {
      if (mounted) setState(() => _checking.remove(cliType));
    }
  }

  /// Registers cliType as a provider (enabled, no key/model to fill in) if
  /// it isn't already one — so finding it here is enough to make it appear
  /// in a chat's model picker, no separate "Add Provider" step.
  Future<void> _ensureProviderRegistered(String cliType) async {
    final api = ref.read(apiClientProvider);
    final existing = await ref.read(providerListProvider.future);
    if (existing.any((p) => p.type == cliType)) return;
    await api.updateProvider(ProviderConfig(
      type: cliType,
      name: ProviderDefaults.displayNames[cliType] ?? cliType,
      model: ProviderDefaults.defaultModels[cliType] ?? cliType,
      enabled: true,
    ));
    ref.invalidate(providerListProvider);
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('tab_cli_connections'),
          style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: theme.textMain),
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('cli_connections_desc'),
          style: TextStyle(fontSize: 13, height: 1.45, color: theme.textDim),
        ),
        const SizedBox(height: 24),
        for (final (type, label) in _clis) ...[
          _CLIRow(
            label: label,
            status: _status[type],
            checking: _checking.contains(type),
            onCheck: () => _check(type),
          ),
          const SizedBox(height: 12),
        ],
      ],
    );
  }
}

class _CLIRow extends StatelessWidget {
  final String label;
  final Map<String, dynamic>? status;
  final bool checking;
  final VoidCallback onCheck;

  const _CLIRow({
    required this.label,
    required this.status,
    required this.checking,
    required this.onCheck,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final installed = status?['installed'] as bool? ?? false;
    final version = status?['version'] as String?;
    final binaryName = status?['binary_name'] as String?;

    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: theme.bgPanel,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.terminal_rounded, size: 18, color: theme.textDim),
              const SizedBox(width: 8),
              Text(label, style: TextStyle(fontWeight: FontWeight.w600, color: theme.textMain)),
              const Spacer(),
              if (checking)
                const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2))
              else if (status != null)
                Icon(
                  installed ? Icons.check_circle_rounded : Icons.error_outline_rounded,
                  size: 18,
                  color: installed ? MemoTheme.green : MemoTheme.red,
                ),
            ],
          ),
          const SizedBox(height: 8),
          if (status == null)
            Text(L10n.t('cli_connections_not_checked'), style: TextStyle(fontSize: 12, color: theme.textDim))
          else if (installed) ...[
            Text(
              L10n.t('cli_connections_installed', {'version': version ?? '?'}),
              style: TextStyle(fontSize: 12, color: MemoTheme.green),
            ),
            const SizedBox(height: 2),
            Text(
              L10n.t('cli_connections_ready_in_picker'),
              style: TextStyle(fontSize: 11, color: theme.textDim),
            ),
          ] else
            Text(
              L10n.t('cli_connections_not_found', {'bin': binaryName ?? ''}),
              style: TextStyle(fontSize: 12, color: MemoTheme.red),
            ),
          const SizedBox(height: 10),
          FilledButton.tonalIcon(
            onPressed: checking ? null : onCheck,
            icon: const Icon(Icons.refresh_rounded, size: 16),
            label: Text(L10n.t('cli_connections_check_btn')),
          ),
        ],
      ),
    );
  }
}
