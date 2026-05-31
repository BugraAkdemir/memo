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
  bool _dismissed = false;

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
    if (_dismissed) return const SizedBox.shrink();

    final gpuAsync = ref.watch(gpuInfoProvider);
    final hasGpu = gpuAsync.whenOrNull(data: (g) => g.hasGpu) ?? false;
    final gpuName = gpuAsync.whenOrNull(data: (g) => g.name) ?? '';

    final title = 'Llama.cpp Eksik';
    final description = hasGpu
        ? 'Uygulamanın modelleri çalıştırabilmesi için Llama.cpp motorunun kurulması gerekiyor. '
            'Sisteminizde $gpuName bulundu — GPU destekli sürüm indirilecek.'
        : 'Uygulamanın modelleri çalıştırabilmesi için Llama.cpp motorunun kurulması gerekiyor. '
            'Bu işlem sisteminize uygun CPU sürümünü indirecektir.';
    final primaryLabel =
        hasGpu ? 'Ekran Kartı İçin Kur (Önerilen)' : 'Motoru İndir ve Kur';

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
              Text(
                title,
                style: const TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
                  color: MemoTheme.textMain,
                ),
              ),
              const SizedBox(height: 12),
              Text(
                description,
                textAlign: TextAlign.center,
                style: const TextStyle(color: MemoTheme.textDim, height: 1.5),
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
                      : Text(
                          primaryLabel,
                          style: const TextStyle(fontWeight: FontWeight.w600),
                        ),
                ),
              ),
              // Only show "Skip GPU / Use CPU" button when a GPU was actually detected.
              if (hasGpu) ...[
                const SizedBox(height: 12),
                SizedBox(
                  width: double.infinity,
                  height: 40,
                  child: OutlinedButton(
                    onPressed: _installing
                        ? null
                        : () {
                            setState(() {
                              _dismissed = true;
                            });
                          },
                    style: OutlinedButton.styleFrom(
                      foregroundColor: MemoTheme.textDim,
                      side: const BorderSide(color: MemoTheme.borderSoft),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      ),
                    ),
                    child: const Text(
                      'Şimdilik Atla (Daha Sonra Ayarlardan Kur)',
                      style: TextStyle(fontWeight: FontWeight.w500),
                    ),
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
