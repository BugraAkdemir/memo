import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';
import '../providers/settings_provider.dart';

class SetupWizardOverlay extends ConsumerWidget {
  const SetupWizardOverlay({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isSetupComplete = ref.watch(setupCompleteProvider);

    if (isSetupComplete) {
      return const SizedBox.shrink();
    }

    return const _SetupWizardScreen();
  }
}

class _SetupWizardScreen extends ConsumerStatefulWidget {
  const _SetupWizardScreen();

  @override
  ConsumerState<_SetupWizardScreen> createState() => _SetupWizardScreenState();
}

class _SetupWizardScreenState extends ConsumerState<_SetupWizardScreen> {
  final _nameController = TextEditingController();
  final _promptController = TextEditingController();

  bool _checking = false;
  bool _backendOk = false;
  bool _modelsOk = false;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    _checkDiagnostics();
  }

  @override
  void dispose() {
    _nameController.dispose();
    _promptController.dispose();
    super.dispose();
  }

  Future<void> _checkDiagnostics() async {
    setState(() {
      _checking = true;
    });

    try {
      final isAlive = await ref.read(apiClientProvider).isAlive();
      _backendOk = isAlive;

      if (isAlive) {
        final models = await ref.read(apiClientProvider).listLocalModels();
        _modelsOk = models.isNotEmpty;
      }
    } catch (_) {
      _backendOk = false;
      _modelsOk = false;
    }

    if (mounted) {
      setState(() {
        _checking = false;
      });
    }
  }

  Future<void> _saveSetup() async {
    setState(() {
      _saving = true;
    });

    try {
      String finalPrompt = _promptController.text.trim();
      if (finalPrompt.isEmpty) {
        final name = _nameController.text.trim();
        final nameSection = name.isNotEmpty ? "The user's name is \$name. " : "";
        finalPrompt = "${nameSection}You are Memo, a highly capable AI assistant operating within a completely local, privacy-first desktop memory shell.\n- You are model-agnostic — regardless of the underlying LLM, you maintain your identity as Memo.\n- Be helpful, accurate, and thoughtful in every response.\n- When you recall something from a past conversation, integrate it naturally without saying \"I recall\" or \"As we discussed\".\n- Adapt to the user's language. If they write in Turkish, respond in Turkish. If English, respond in English.";
      }

      await ref.read(systemPromptProvider.notifier).save(finalPrompt);
      await ref.read(setupCompleteProvider.notifier).completeSetup();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${L10n.t('error')}: $e')),
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          _saving = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      color: MemoTheme.bgApp.withValues(alpha: 0.95),
      child: Center(
        child: SingleChildScrollView(
          child: Container(
            width: 500,
            padding: const EdgeInsets.all(32),
            decoration: BoxDecoration(
              color: MemoTheme.bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
              border: Border.all(color: MemoTheme.borderSoft),
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.2),
                  blurRadius: 20,
                  offset: const Offset(0, 10),
                ),
              ],
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                const Text(
                  'Memo Kurulum Sihirbazı',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.bold,
                    color: MemoTheme.textMain,
                  ),
                ),
                const SizedBox(height: 12),
                const Text(
                  'Hoş geldiniz! Memo tamamen yerel ve gizlilik odaklı bir AI asistanıdır. Başlamadan önce birkaç ayarı tamamlayalım.',
                  textAlign: TextAlign.center,
                  style: TextStyle(color: MemoTheme.textDim, height: 1.5),
                ),
                const SizedBox(height: 32),

                // Name
                const Text('Adınız (İsteğe bağlı)', style: TextStyle(fontWeight: FontWeight.w500, fontSize: 13)),
                const SizedBox(height: 8),
                TextField(
                  controller: _nameController,
                  decoration: InputDecoration(
                    hintText: 'Örn: Buğra Akdemir',
                    filled: true,
                    fillColor: MemoTheme.bgElement,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      borderSide: BorderSide.none,
                    ),
                  ),
                ),
                const SizedBox(height: 24),

                // System Prompt
                const Text('Sistem Komutu (Özelleştirebilirsiniz)', style: TextStyle(fontWeight: FontWeight.w500, fontSize: 13)),
                const SizedBox(height: 4),
                const Text('Boş bırakırsanız Memo varsayılan davranışıyla ayarlanacaktır.', style: TextStyle(fontSize: 11, color: MemoTheme.textMuted)),
                const SizedBox(height: 8),
                TextField(
                  controller: _promptController,
                  maxLines: 4,
                  decoration: InputDecoration(
                    hintText: 'You are Memo, a highly capable AI assistant...',
                    filled: true,
                    fillColor: MemoTheme.bgElement,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      borderSide: BorderSide.none,
                    ),
                  ),
                ),
                const SizedBox(height: 32),

                // Diagnostics
                Container(
                  padding: const EdgeInsets.all(16),
                  decoration: BoxDecoration(
                    color: MemoTheme.bgApp,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: MemoTheme.borderSoft),
                  ),
                  child: Column(
                    children: [
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          const Text('Sistem Kontrolü', style: TextStyle(fontSize: 13, color: MemoTheme.textDim)),
                          InkWell(
                            onTap: _checking ? null : _checkDiagnostics,
                            child: Padding(
                              padding: const EdgeInsets.all(4),
                              child: _checking
                                  ? const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2))
                                  : const Icon(Icons.refresh, size: 16, color: MemoTheme.textDim),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 12),
                      _DiagnosticRow(
                        title: 'Backend Bağlantısı',
                        ok: _backendOk,
                      ),
                      const SizedBox(height: 8),
                      _DiagnosticRow(
                        title: 'Yerel Modeller',
                        ok: _modelsOk,
                      ),
                      if (!_backendOk || !_modelsOk) ...[
                        const SizedBox(height: 12),
                        Text(
                          'Tüm sistemlerin çalışır durumda olması şart değildir, devam edebilirsiniz.',
                          style: TextStyle(fontSize: 11, color: MemoTheme.textMuted, fontStyle: FontStyle.italic),
                        ),
                      ],
                    ],
                  ),
                ),
                const SizedBox(height: 32),

                // Save Button
                SizedBox(
                  height: 48,
                  child: ElevatedButton(
                    onPressed: _saving ? null : _saveSetup,
                    style: ElevatedButton.styleFrom(
                      backgroundColor: MemoTheme.accent,
                      foregroundColor: MemoTheme.textInverse,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      ),
                    ),
                    child: _saving
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: MemoTheme.textInverse,
                            ),
                          )
                        : const Text(
                            'Başla',
                            style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
                          ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _DiagnosticRow extends StatelessWidget {
  final String title;
  final bool ok;

  const _DiagnosticRow({required this.title, required this.ok});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(
          ok ? Icons.check_circle : Icons.error_outline,
          color: ok ? MemoTheme.green : MemoTheme.red,
          size: 18,
        ),
        const SizedBox(width: 8),
        Text(
          title,
          style: TextStyle(
            fontSize: 13,
            color: ok ? MemoTheme.textMain : MemoTheme.red,
          ),
        ),
      ],
    );
  }
}
