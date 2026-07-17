import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
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
    if (!isSetupComplete) return  SizedBox.shrink();

    final installedAsync = ref.watch(llamaInstalledProvider);

    return installedAsync.when(
      data: (installed) {
        if (installed) return  SizedBox.shrink();
        return  _InstallerScreen();
      },
      loading: () =>  SizedBox.shrink(),
      error: (e, _) =>  SizedBox.shrink(),
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
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _installing = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_dismissed) return  SizedBox.shrink();

    final gpuAsync = ref.watch(gpuInfoProvider);
    final hasGpu = gpuAsync.whenOrNull(data: (g) => g.hasGpu) ?? false;
    final gpuName = gpuAsync.whenOrNull(data: (g) => g.name) ?? '';

    final title = L10n.t('llama_missing_title');
    final description = hasGpu
        ? L10n.t('llama_missing_desc_gpu', {'gpu': gpuName})
        : L10n.t('llama_missing_desc_cpu');
    final primaryLabel =
        hasGpu ? L10n.t('llama_install_gpu') : L10n.t('llama_install');

    return Container(
      color: MemoTheme.of(context).bgApp.withValues(alpha: 0.95),
      child: Center(
        child: Container(
          width: 400,
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
            children: [
              Container(
                width: 64,
                height: 64,
                decoration: BoxDecoration(
                  color: MemoTheme.accentPale,
                  shape: BoxShape.circle,
                ),
                child:  Icon(Icons.memory, size: 32, color: MemoTheme.accent),
              ),
               SizedBox(height: 24),
              Text(
                title,
                style:  TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
               SizedBox(height: 12),
              Text(
                description,
                textAlign: TextAlign.center,
                style:  TextStyle(color: MemoTheme.of(context).textDim, height: 1.5),
              ),
               SizedBox(height: 32),
              if (_error.isNotEmpty)
                Padding(
                  padding:  EdgeInsets.only(bottom: 16),
                  child: Text(
                    _error,
                    style:  TextStyle(color: MemoTheme.red, fontSize: 12),
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
                    foregroundColor: MemoTheme.of(context).textInverse,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    ),
                  ),
                  child: _installing
                      ?  SizedBox(
                          width: 20,
                          height: 20,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: MemoTheme.of(context).textInverse,
                          ),
                        )
                      : Text(
                          primaryLabel,
                          style:  TextStyle(fontWeight: FontWeight.w600),
                        ),
                ),
              ),
              // Only show "Skip GPU / Use CPU" button when a GPU was actually detected.
              if (hasGpu) ...[
                 SizedBox(height: 12),
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
                      foregroundColor: MemoTheme.of(context).textDim,
                      side:  BorderSide(color: MemoTheme.of(context).borderSoft),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      ),
                    ),
                    child: Text(
                      L10n.t('skip'),
                      style: const TextStyle(fontWeight: FontWeight.w500),
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
