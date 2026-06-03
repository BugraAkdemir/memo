import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';
import '../providers/settings_provider.dart';

class SetupWizardOverlay extends ConsumerWidget {
   SetupWizardOverlay({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isSetupComplete = ref.watch(setupCompleteProvider);

    if (isSetupComplete) {
      return  SizedBox.shrink();
    }

    return  _SetupWizardScreen();
  }
}

class _SetupWizardScreen extends ConsumerStatefulWidget {
   _SetupWizardScreen();

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
        final nameSection = name.isNotEmpty ? "The user's name is $name. " : "";
        finalPrompt = "${nameSection}You are Memo, a highly capable, privacy-first AI assistant running entirely locally on the user's device.\n\nCORE DIRECTIVES:\n1. Identity: You are always Memo, regardless of the underlying LLM. Act as a smart, reliable, and direct partner.\n2. Anti-Hallucination: Never invent, guess, or fabricate information. If you are unsure or do not know the answer, explicitly state that you do not know.\n3. Conciseness & Structure: Keep your answers clear, well-structured, and strictly to the point. Avoid long, rambling introductions or unnecessary filler words.\n4. Seamless Memory: You have access to the user's personal context. Use this information naturally to inform your answers. STRICTLY FORBIDDEN: Do not use phrases like \"I remember,\" \"As we discussed,\" \"Based on your data,\" or \"I recall.\" Simply present the information as shared context.\n5. Language Mirroring: Always respond in the exact language the user communicates in (e.g., if the user asks in Turkish, your entire response must be in Turkish).";
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
      color: MemoTheme.of(context).bgApp.withValues(alpha: 0.95),
      child: Center(
        child: SingleChildScrollView(
          child: Container(
            width: 500,
            padding:  EdgeInsets.all(32),
            decoration: BoxDecoration(
              color: MemoTheme.of(context).bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
              border: Border.all(color: MemoTheme.of(context).borderSoft),
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.2),
                  blurRadius: 20,
                  offset:  Offset(0, 10),
                ),
              ],
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                 Text(
                  'Memo Kurulum Sihirbazı',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.bold,
                    color: MemoTheme.of(context).textMain,
                  ),
                ),
                 SizedBox(height: 12),
                 Text(
                  'Hoş geldiniz! Memo tamamen yerel ve gizlilik odaklı bir AI asistanıdır. Başlamadan önce birkaç ayarı tamamlayalım.',
                  textAlign: TextAlign.center,
                  style: TextStyle(color: MemoTheme.of(context).textDim, height: 1.5),
                ),
                 SizedBox(height: 32),

                // Name
                 Text('Adınız (İsteğe bağlı)', style: TextStyle(fontWeight: FontWeight.w500, fontSize: 13)),
                 SizedBox(height: 8),
                TextField(
                  controller: _nameController,
                  decoration: InputDecoration(
                    hintText: 'Örn: Buğra Akdemir',
                    filled: true,
                    fillColor: MemoTheme.of(context).bgElement,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      borderSide: BorderSide.none,
                    ),
                  ),
                ),
                 SizedBox(height: 24),

                // System Prompt
                 Text('Sistem Komutu (Özelleştirebilirsiniz)', style: TextStyle(fontWeight: FontWeight.w500, fontSize: 13)),
                 SizedBox(height: 4),
                Text('Boş bırakırsanız Memo varsayılan davranışıyla ayarlanacaktır.', style: TextStyle(fontSize: 11, color: MemoTheme.of(context).textMuted)),
                 SizedBox(height: 8),
                TextField(
                  controller: _promptController,
                  maxLines: 4,
                  decoration: InputDecoration(
                    hintText: 'You are Memo, a highly capable AI assistant...',
                    filled: true,
                    fillColor: MemoTheme.of(context).bgElement,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      borderSide: BorderSide.none,
                    ),
                  ),
                ),
                 SizedBox(height: 32),

                // Diagnostics
                Container(
                  padding:  EdgeInsets.all(16),
                  decoration: BoxDecoration(
                    color: MemoTheme.of(context).bgApp,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: MemoTheme.of(context).borderSoft),
                  ),
                  child: Column(
                    children: [
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Text('Sistem Kontrolü', style: TextStyle(fontSize: 13, color: MemoTheme.of(context).textDim)),
                          InkWell(
                            onTap: _checking ? null : _checkDiagnostics,
                            child: Padding(
                              padding:  EdgeInsets.all(4),
                              child: _checking
                                  ?  SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2))
                                  :  Icon(Icons.refresh, size: 16, color: MemoTheme.of(context).textDim),
                            ),
                          ),
                        ],
                      ),
                       SizedBox(height: 12),
                      _DiagnosticRow(
                        title: 'Backend Bağlantısı',
                        ok: _backendOk,
                      ),
                       SizedBox(height: 8),
                      _DiagnosticRow(
                        title: 'Yerel Modeller',
                        ok: _modelsOk,
                      ),
                      if (!_backendOk || !_modelsOk) ...[
                         SizedBox(height: 12),
                        Text(
                          'Tüm sistemlerin çalışır durumda olması şart değildir, devam edebilirsiniz.',
                          style: TextStyle(fontSize: 11, color: MemoTheme.of(context).textMuted, fontStyle: FontStyle.italic),
                        ),
                      ],
                    ],
                  ),
                ),
                 SizedBox(height: 32),

                // Save Button
                SizedBox(
                  height: 48,
                  child: ElevatedButton(
                    onPressed: _saving ? null : _saveSetup,
                    style: ElevatedButton.styleFrom(
                      backgroundColor: MemoTheme.accent,
                      foregroundColor: MemoTheme.of(context).textInverse,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      ),
                    ),
                    child: _saving
                        ?  SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: MemoTheme.of(context).textInverse,
                            ),
                          )
                        :  Text(
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

   _DiagnosticRow({required this.title, required this.ok});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(
          ok ? Icons.check_circle : Icons.error_outline,
          color: ok ? MemoTheme.green : MemoTheme.red,
          size: 18,
        ),
         SizedBox(width: 8),
        Text(
          title,
          style: TextStyle(
            fontSize: 13,
            color: ok ? MemoTheme.of(context).textMain : MemoTheme.red,
          ),
        ),
      ],
    );
  }
}
