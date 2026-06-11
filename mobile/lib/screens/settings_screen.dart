import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../core/api_client.dart';
import '../core/theme.dart';
import '../providers/connection_provider.dart' hide ConnectionState;

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return DefaultTabController(
      length: 3,
      child: Scaffold(
        backgroundColor: MemoTheme.bg,
        appBar: AppBar(
          title: const Text('Settings'),
          bottom: const TabBar(
            tabs: [
              Tab(text: 'General'),
              Tab(text: 'Providers'),
              Tab(text: 'Models'),
            ],
          ),
        ),
        body: const TabBarView(
          children: [
            _GeneralTab(),
            _ProvidersTab(),
            _ModelsTab(),
          ],
        ),
      ),
    );
  }
}

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
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Saved')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Error: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        TextField(
          controller: _urlCtrl,
          decoration: const InputDecoration(
            labelText: 'Backend URL',
            prefixIcon: Icon(Icons.link),
          ),
          keyboardType: TextInputType.url,
        ),
        const SizedBox(height: 12),
        TextField(
          controller: _tokenCtrl,
          decoration: const InputDecoration(
            labelText: 'Token (optional)',
            prefixIcon: Icon(Icons.vpn_key),
          ),
        ),
        const SizedBox(height: 20),
        SizedBox(
          width: double.infinity,
          height: 48,
          child: FilledButton(
            onPressed: _saving ? null : _save,
            style: FilledButton.styleFrom(
              backgroundColor: MemoTheme.accent,
              foregroundColor: MemoTheme.bg,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12),
              ),
            ),
            child: _saving
                ? const SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: MemoTheme.bg,
                    ),
                  )
                : const Text('Save & Reconnect',
                    style: TextStyle(fontWeight: FontWeight.w600)),
          ),
        ),
      ],
    );
  }
}

class _ProvidersTab extends ConsumerWidget {
  const _ProvidersTab();

  Future<void> _toggleProvider(WidgetRef ref, ProviderConfig p) async {
    try {
      await ref
          .read(apiClientProvider)
          .updateProvider(p.copyWith(enabled: !p.enabled));
    } catch (_) {}
  }

  Future<void> _deleteProvider(BuildContext context, WidgetRef ref, String type) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: MemoTheme.surface,
        title: const Text('Delete Provider'),
        content: const Text('Are you sure?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Delete', style: TextStyle(color: MemoTheme.error)),
          ),
        ],
      ),
    );
    if (confirm == true) {
      try {
        await ref.read(apiClientProvider).deleteProvider(type);
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('Provider deleted')),
          );
        }
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Error: $e')),
          );
        }
      }
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return FutureBuilder<List<ProviderConfig>>(
      future: ref.read(apiClientProvider).getProviders(),
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: CircularProgressIndicator());
        }
        if (snapshot.hasError) {
          return Center(
            child: Padding(
              padding: const EdgeInsets.all(32),
              child: Text('Error: ${snapshot.error}',
                  style: const TextStyle(color: MemoTheme.error)),
            ),
          );
        }
        final providers = snapshot.data ?? [];

        if (providers.isEmpty) {
          return const Center(
            child: Text('No providers configured.',
                style: TextStyle(color: MemoTheme.textDim)),
          );
        }

        return ListView.builder(
          padding: const EdgeInsets.all(16),
          itemCount: providers.length,
          itemBuilder: (context, index) {
            final p = providers[index];
            return Card(
              color: MemoTheme.surface,
              margin: const EdgeInsets.only(bottom: 8),
              child: InkWell(
                onTap: () => _showProviderDetail(context, ref, p),
                borderRadius: BorderRadius.circular(12),
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Row(
                    children: [
                      Container(
                        width: 44,
                        height: 44,
                        decoration: BoxDecoration(
                          color: MemoTheme.accent.withAlpha(20),
                          borderRadius: BorderRadius.circular(12),
                        ),
                        child: const Icon(Icons.cloud,
                            color: MemoTheme.accent, size: 22),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(p.name,
                                style: const TextStyle(
                                    fontWeight: FontWeight.w600)),
                            const SizedBox(height: 2),
                            Text(p.model,
                                style: const TextStyle(
                                    fontSize: 12, color: MemoTheme.textDim)),
                          ],
                        ),
                      ),
                      GestureDetector(
                        onTap: () => _toggleProvider(ref, p),
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                              horizontal: 10, vertical: 6),
                          decoration: BoxDecoration(
                            color: p.enabled
                                ? MemoTheme.success.withAlpha(25)
                                : MemoTheme.textDim.withAlpha(25),
                            borderRadius: BorderRadius.circular(6),
                          ),
                          child: Text(
                            p.enabled ? 'ON' : 'OFF',
                            style: TextStyle(
                              fontSize: 11,
                              fontWeight: FontWeight.w600,
                              color: p.enabled
                                  ? MemoTheme.success
                                  : MemoTheme.textDim,
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            );
          },
        );
      },
    );
  }

  void _showProviderDetail(
      BuildContext context, WidgetRef ref, ProviderConfig p) {
    showModalBottomSheet(
      context: context,
      backgroundColor: MemoTheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(p.name,
                style: const TextStyle(
                    fontSize: 18, fontWeight: FontWeight.w600)),
            const SizedBox(height: 4),
            Text(p.type,
                style: const TextStyle(color: MemoTheme.textDim, fontSize: 13)),
            const SizedBox(height: 16),
            _detailRow('Model', p.model),
            if (p.baseUrl != null && p.baseUrl!.isNotEmpty)
              _detailRow('Base URL', p.baseUrl!),
            _detailRow('Status', p.connected ? 'Connected' : 'Unknown'),
            const SizedBox(height: 20),
            SizedBox(
              width: double.infinity,
              child: OutlinedButton.icon(
                onPressed: () {
                  Navigator.pop(ctx);
                  _deleteProvider(context, ref, p.type);
                },
                icon: const Icon(Icons.delete_outline, size: 16),
                label: const Text('Delete Provider'),
                style: OutlinedButton.styleFrom(
                  foregroundColor: MemoTheme.error,
                  side: BorderSide(color: MemoTheme.error.withAlpha(60)),
                ),
              ),
            ),
            const SizedBox(height: 12),
          ],
        ),
      ),
    );
  }

  Widget _detailRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 80,
            child: Text(label,
                style: const TextStyle(
                    color: MemoTheme.textDim, fontSize: 13)),
          ),
          Expanded(
            child: Text(value,
                style: const TextStyle(fontSize: 13)),
          ),
        ],
      ),
    );
  }
}

class _ModelsTab extends ConsumerStatefulWidget {
  const _ModelsTab();

  @override
  ConsumerState<_ModelsTab> createState() => _ModelsTabState();
}

class _ModelsTabState extends ConsumerState<_ModelsTab> {
  bool _stopping = false;

  Future<void> _stopModel() async {
    setState(() => _stopping = true);
    try {
      await ref.read(apiClientProvider).stopModel();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Model stopped')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Error: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _stopping = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<List<LocalModel>>(
      future: ref.read(apiClientProvider).listLocalModels(),
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: CircularProgressIndicator());
        }
        if (snapshot.hasError) {
          return Center(
            child: Padding(
              padding: const EdgeInsets.all(32),
              child: Text('Error: ${snapshot.error}',
                  style: const TextStyle(color: MemoTheme.error)),
            ),
          );
        }
        final models = snapshot.data ?? [];

        return Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
              child: SizedBox(
                width: double.infinity,
                child: OutlinedButton.icon(
                  onPressed: _stopping ? null : _stopModel,
                  icon: _stopping
                      ? const SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.stop, size: 16),
                  label: const Text('Stop Running Model'),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: MemoTheme.error,
                    side: BorderSide(color: MemoTheme.error.withAlpha(60)),
                  ),
                ),
              ),
            ),
            Expanded(
              child: models.isEmpty
                  ? const Center(
                      child: Text('No models found.',
                          style: TextStyle(color: MemoTheme.textDim)),
                    )
                  : ListView.builder(
                      padding: const EdgeInsets.all(16),
                      itemCount: models.length,
                      itemBuilder: (context, index) {
                        final m = models[index];
                        return _ModelCard(m: m);
                      },
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

  Future<void> _startModel() async {
    setState(() => _starting = true);
    try {
      await ref.read(apiClientProvider).startModel(path: widget.m.path);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${widget.m.filename} started')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Error: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _starting = false);
    }
  }

  String _formatSize(int bytes) {
    if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(0)} KB';
    if (bytes < 1024 * 1024 * 1024) {
      return '${(bytes / (1024 * 1024)).toStringAsFixed(0)} MB';
    }
    return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
  }

  @override
  Widget build(BuildContext context) {
    return Card(
      color: MemoTheme.surface,
      margin: const EdgeInsets.only(bottom: 8),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: MemoTheme.accent.withAlpha(20),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Icon(
                widget.m.isEmbedding ? Icons.auto_awesome : Icons.memory,
                color: MemoTheme.accent,
                size: 22,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(widget.m.filename,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontWeight: FontWeight.w600)),
                  const SizedBox(height: 2),
                  Text(
                    '${_formatSize(widget.m.sizeBytes)}${widget.m.isEmbedding ? " · embedding" : ""}',
                    style: const TextStyle(
                        fontSize: 12, color: MemoTheme.textDim),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 8),
            SizedBox(
              height: 36,
              child: ElevatedButton(
                onPressed: _starting ? null : _startModel,
                style: ElevatedButton.styleFrom(
                  backgroundColor: MemoTheme.accent,
                  foregroundColor: MemoTheme.bg,
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8),
                  ),
                ),
                child: _starting
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: MemoTheme.bg,
                        ),
                      )
                    : const Text('Start', style: TextStyle(fontSize: 13)),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
