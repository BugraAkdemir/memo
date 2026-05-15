import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/theme.dart';
import '../providers/models_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/settings_provider.dart';

class LlamaInstallerOverlay extends ConsumerWidget {
  const LlamaInstallerOverlay({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Don't show llama installer until setup wizard is complete
    final isSetupComplete = ref.watch(setupCompleteProvider);
    if (!isSetupComplete) return const SizedBox.shrink();

    final installedAsync = ref.watch(llamaInstalledProvider);

    return installedAsync.when(
      data: (installed) {
        if (installed) return const SizedBox.shrink();
        return const _InstallerScreen();
      },
      loading: () => const SizedBox.shrink(),
      error: (e, _) => const SizedBox.shrink(),
    );
  }
}

class _InstallerScreen extends ConsumerStatefulWidget {
  const _InstallerScreen();

  @override
  ConsumerState<_InstallerScreen> createState() => _InstallerScreenState();
}

class _InstallerScreenState extends ConsumerState<_InstallerScreen> {
  bool _installing = false;
  String _error = '';

  Future<void> _startInstall() async {
    setState(() {
      _installing = true;
      _error = '';
    });

    try {
      await ref.read(apiClientProvider).installLlamaServer();
      ref.invalidate(llamaInstalledProvider);
    } catch (e) {
      setState(() {
        _error = e.toString();
        _installing = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      color: MemoTheme.bgApp.withValues(alpha: 0.95),
      child: Center(
        child: Container(
          width: 400,
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
            children: [
              Container(
                width: 64,
                height: 64,
                decoration: BoxDecoration(
                  color: MemoTheme.accentPale,
                  shape: BoxShape.circle,
                ),
                child: const Icon(Icons.memory, size: 32, color: MemoTheme.accent),
              ),
              const SizedBox(height: 24),
              const Text(
                'Llama.cpp Eksik',
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
                  color: MemoTheme.textMain,
                ),
              ),
              const SizedBox(height: 12),
              const Text(
                'Uygulamanın modelleri çalıştırabilmesi için Llama.cpp motorunun kurulması gerekiyor. Bu işlem sisteminize uygun sürümü indirecektir.',
                textAlign: TextAlign.center,
                style: TextStyle(color: MemoTheme.textDim, height: 1.5),
              ),
              const SizedBox(height: 32),
              if (_error.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(bottom: 16),
                  child: Text(
                    _error,
                    style: const TextStyle(color: MemoTheme.red, fontSize: 12),
                    textAlign: TextAlign.center,
                  ),
                ),
              SizedBox(
                width: double.infinity,
                height: 44,
                child: ElevatedButton(
                  onPressed: _installing ? null : _startInstall,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: MemoTheme.accent,
                    foregroundColor: MemoTheme.textInverse,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    ),
                  ),
                  child: _installing
                      ? const SizedBox(
                          width: 20,
                          height: 20,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: MemoTheme.textInverse,
                          ),
                        )
                      : const Text(
                          'Şimdi Kur',
                          style: TextStyle(fontWeight: FontWeight.w600),
                        ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
