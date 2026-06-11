import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/theme.dart';
import '../providers/connection_provider.dart';

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
      _remoteMode = state.remoteMode;
    });
  }

  @override
  void dispose() {
    _urlCtrl.dispose();
    _tokenCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(connectionStateProvider);

    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.symmetric(horizontal: 32),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Container(
                  width: 80,
                  height: 80,
                  decoration: BoxDecoration(
                    color: MemoTheme.accent.withAlpha(30),
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: const Icon(
                    Icons.memory_outlined,
                    size: 40,
                    color: MemoTheme.accent,
                  ),
                ),
                const SizedBox(height: 24),
                Text(
                  'Memo',
                  style: Theme.of(context).textTheme.headlineLarge,
                ),
                const SizedBox(height: 8),
                Text(
                  'Connect to your Memo backend',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
                const SizedBox(height: 32),
                Container(
                  padding: const EdgeInsets.all(4),
                  decoration: BoxDecoration(
                    color: MemoTheme.surfaceAlt,
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Row(
                    children: [
                      Expanded(
                        child: GestureDetector(
                          onTap: () => setState(() => _remoteMode = false),
                          child: Container(
                            padding: const EdgeInsets.symmetric(vertical: 10),
                            decoration: BoxDecoration(
                              color: !_remoteMode
                                  ? MemoTheme.accent.withAlpha(30)
                                  : Colors.transparent,
                              borderRadius: BorderRadius.circular(10),
                            ),
                            child: Row(
                              mainAxisAlignment: MainAxisAlignment.center,
                              children: [
                                Icon(
                                  Icons.home,
                                  size: 16,
                                  color: !_remoteMode
                                      ? MemoTheme.accent
                                      : MemoTheme.textDim,
                                ),
                                const SizedBox(width: 6),
                                Text(
                                  'Local',
                                  style: TextStyle(
                                    color: !_remoteMode
                                        ? MemoTheme.accent
                                        : MemoTheme.textDim,
                                    fontWeight: !_remoteMode
                                        ? FontWeight.w600
                                        : FontWeight.w400,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                      Expanded(
                        child: GestureDetector(
                          onTap: () => setState(() => _remoteMode = true),
                          child: Container(
                            padding: const EdgeInsets.symmetric(vertical: 10),
                            decoration: BoxDecoration(
                              color: _remoteMode
                                  ? MemoTheme.accent.withAlpha(30)
                                  : Colors.transparent,
                              borderRadius: BorderRadius.circular(10),
                            ),
                            child: Row(
                              mainAxisAlignment: MainAxisAlignment.center,
                              children: [
                                Icon(
                                  Icons.public,
                                  size: 16,
                                  color: _remoteMode
                                      ? MemoTheme.accent
                                      : MemoTheme.textDim,
                                ),
                                const SizedBox(width: 6),
                                Text(
                                  'Remote',
                                  style: TextStyle(
                                    color: _remoteMode
                                        ? MemoTheme.accent
                                        : MemoTheme.textDim,
                                    fontWeight: _remoteMode
                                        ? FontWeight.w600
                                        : FontWeight.w400,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 20),
                TextField(
                  controller: _urlCtrl,
                  decoration: InputDecoration(
                    labelText: _remoteMode ? 'ngrok / Public URL' : 'Local IP',
                    hintText: _remoteMode
                        ? 'https://abc123.ngrok.io'
                        : 'http://192.168.1.100:8090',
                    prefixIcon: const Icon(Icons.link),
                  ),
                  keyboardType: TextInputType.url,
                  textInputAction: TextInputAction.next,
                ),
                if (_remoteMode) ...[
                  const SizedBox(height: 12),
                  TextField(
                    controller: _tokenCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Token',
                      hintText: 'memo-abc123...',
                      prefixIcon: Icon(Icons.vpn_key),
                    ),
                    textInputAction: TextInputAction.go,
                    onSubmitted: (_) => _connect(),
                  ),
                ],
                if (state.error != null) ...[
                  const SizedBox(height: 12),
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: MemoTheme.error.withAlpha(25),
                      borderRadius: BorderRadius.circular(10),
                      border: Border.all(
                        color: MemoTheme.error.withAlpha(60),
                      ),
                    ),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Icon(Icons.error_outline,
                            size: 18, color: MemoTheme.error),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Text(
                            state.error!,
                            style: const TextStyle(
                              color: MemoTheme.error,
                              fontSize: 13,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
                const SizedBox(height: 24),
                SizedBox(
                  width: double.infinity,
                  height: 50,
                  child: FilledButton(
                    onPressed: state.connecting ? null : _connect,
                    style: FilledButton.styleFrom(
                      backgroundColor: MemoTheme.accent,
                      foregroundColor: MemoTheme.bg,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                    ),
                    child: state.connecting
                        ? const SizedBox(
                            width: 22,
                            height: 22,
                            child: CircularProgressIndicator(
                              strokeWidth: 2.5,
                              color: MemoTheme.bg,
                            ),
                          )
                        : const Text(
                            'Connect',
                            style: TextStyle(
                              fontSize: 16,
                              fontWeight: FontWeight.w600,
                            ),
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

  void _connect() {
    final url = _urlCtrl.text.trim();
    if (url.isEmpty) return;
    final token = _tokenCtrl.text.trim();
    ref.read(connectionStateProvider.notifier).connect(
      url,
      token: token,
      remote: _remoteMode,
    );
  }
}
