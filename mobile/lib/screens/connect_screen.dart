import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/theme.dart';
import '../providers/connection_provider.dart';
import '../widgets/branding.dart';

class ConnectScreen extends ConsumerStatefulWidget {
  const ConnectScreen({super.key});

  @override
  ConsumerState<ConnectScreen> createState() => _ConnectScreenState();
}

class _ConnectScreenState extends ConsumerState<ConnectScreen> {
  late final TextEditingController _urlCtrl;
  late final TextEditingController _tokenCtrl;
  bool _remoteMode = false;

  @override
  void initState() {
    super.initState();
    _urlCtrl = TextEditingController();
    _tokenCtrl = TextEditingController();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(connectionStateProvider.notifier).loadSavedUrl();
      final state = ref.read(connectionStateProvider);
      _urlCtrl.text = state.baseUrl;
      _tokenCtrl.text = state.token;
      setState(() => _remoteMode = state.remoteMode);
    });
  }

  @override
  void dispose() {
    _urlCtrl.dispose();
    _tokenCtrl.dispose();
    super.dispose();
  }

  void _connect() {
    final url = _urlCtrl.text.trim();
    if (url.isEmpty) return;
    HapticFeedback.lightImpact();
    FocusScope.of(context).unfocus();
    ref.read(connectionStateProvider.notifier).connect(
          url,
          token: _tokenCtrl.text.trim(),
          remote: _remoteMode,
        );
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(connectionStateProvider);

    return Scaffold(
      body: Stack(
        children: [
          const Positioned(top: -120, left: -40, child: LampGlow(size: 460, opacity: 0.9)),
          SafeArea(
            child: Center(
              child: SingleChildScrollView(
                padding: const EdgeInsets.fromLTRB(28, 32, 28, 32),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 440),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Center(child: const MemoLogo(size: 76)),
                      const SizedBox(height: 22),
                      Center(
                        child: Text('Memo', style: Theme.of(context).textTheme.displayLarge),
                      ),
                      const SizedBox(height: 8),
                      Center(
                        child: Text(
                          'Pair with your desktop to reach\nyour second brain from here.',
                          textAlign: TextAlign.center,
                          style: MemoTheme.body(14.5, color: MemoTheme.textDim, height: 1.5),
                        ),
                      ),
                      const SizedBox(height: 34),
                      _segmented(),
                      const SizedBox(height: 18),
                      _fieldLabel(_remoteMode ? 'PUBLIC URL' : 'DESKTOP ADDRESS'),
                      const SizedBox(height: 8),
                      TextField(
                        controller: _urlCtrl,
                        keyboardType: TextInputType.url,
                        autocorrect: false,
                        textInputAction: _remoteMode ? TextInputAction.next : TextInputAction.go,
                        style: MemoTheme.mono(14, color: MemoTheme.text),
                        decoration: InputDecoration(
                          hintText: _remoteMode ? 'https://abc123.ngrok.io' : 'http://192.168.1.100:8090',
                          prefixIcon: const Icon(Icons.lan_outlined, size: 20),
                        ),
                        onSubmitted: (_) => _remoteMode ? null : _connect(),
                      ),
                      if (_remoteMode) ...[
                        const SizedBox(height: 16),
                        _fieldLabel('ACCESS TOKEN'),
                        const SizedBox(height: 8),
                        TextField(
                          controller: _tokenCtrl,
                          autocorrect: false,
                          textInputAction: TextInputAction.go,
                          style: MemoTheme.mono(14, color: MemoTheme.text),
                          decoration: const InputDecoration(
                            hintText: 'memo-abc123…',
                            prefixIcon: Icon(Icons.key_outlined, size: 20),
                          ),
                          onSubmitted: (_) => _connect(),
                        ),
                      ],
                      AnimatedSize(
                        duration: const Duration(milliseconds: 220),
                        curve: Curves.easeOut,
                        child: state.error != null
                            ? Padding(
                                padding: const EdgeInsets.only(top: 16),
                                child: _errorBox(state.error!),
                              )
                            : const SizedBox.shrink(),
                      ),
                      const SizedBox(height: 24),
                      _connectButton(state.connecting),
                      const SizedBox(height: 18),
                      Center(
                        child: Text(
                          _remoteMode
                              ? 'Find the URL and token under Remote Access on your desktop.'
                              : 'Phone and desktop need to be on the same Wi-Fi.',
                          textAlign: TextAlign.center,
                          style: MemoTheme.body(12.5, color: MemoTheme.textFaint, height: 1.45),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _fieldLabel(String s) => Text(s, style: MemoTheme.mono(11, color: MemoTheme.textFaint, ls: 1.2));

  Widget _segmented() {
    Widget tab(String label, IconData icon, bool active, VoidCallback onTap) {
      return Expanded(
        child: GestureDetector(
          onTap: () {
            HapticFeedback.selectionClick();
            onTap();
          },
          behavior: HitTestBehavior.opaque,
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 200),
            padding: const EdgeInsets.symmetric(vertical: 11),
            decoration: BoxDecoration(
              color: active ? MemoTheme.accent : Colors.transparent,
              borderRadius: BorderRadius.circular(11),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(icon, size: 17, color: active ? MemoTheme.onAccent : MemoTheme.textDim),
                const SizedBox(width: 7),
                Text(label,
                    style: MemoTheme.body(14,
                        w: FontWeight.w600, color: active ? MemoTheme.onAccent : MemoTheme.textDim)),
              ],
            ),
          ),
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(
        color: MemoTheme.surfaceAlt,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: MemoTheme.border),
      ),
      child: Row(
        children: [
          tab('Same network', Icons.wifi_rounded, !_remoteMode, () => setState(() => _remoteMode = false)),
          tab('Remote', Icons.public_rounded, _remoteMode, () => setState(() => _remoteMode = true)),
        ],
      ),
    );
  }

  Widget _errorBox(String msg) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: MemoTheme.error.withValues(alpha: 0.10),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: MemoTheme.error.withValues(alpha: 0.35)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(Icons.error_outline_rounded, size: 18, color: MemoTheme.error),
          const SizedBox(width: 10),
          Expanded(
            child: Text(msg, style: MemoTheme.body(13, color: MemoTheme.error, height: 1.45)),
          ),
        ],
      ),
    );
  }

  Widget _connectButton(bool busy) {
    return SizedBox(
      height: 54,
      child: FilledButton(
        onPressed: busy ? null : _connect,
        style: FilledButton.styleFrom(
          backgroundColor: MemoTheme.accent,
          foregroundColor: MemoTheme.onAccent,
          disabledBackgroundColor: MemoTheme.surfaceHi,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
        ),
        child: busy
            ? const SizedBox(
                width: 22, height: 22,
                child: CircularProgressIndicator(strokeWidth: 2.5, color: MemoTheme.accent),
              )
            : Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Text('Connect', style: MemoTheme.body(16, w: FontWeight.w700, color: MemoTheme.onAccent)),
                  const SizedBox(width: 8),
                  const Icon(Icons.arrow_forward_rounded, size: 19),
                ],
              ),
      ),
    );
  }
}
