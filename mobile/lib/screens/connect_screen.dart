import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
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
  late final TextEditingController _ngrokTokenCtrl;
  bool _remoteMode = false;

  @override
  void initState() {
    super.initState();
    _urlCtrl = TextEditingController();
    _tokenCtrl = TextEditingController();
    _ngrokTokenCtrl = TextEditingController();
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
    _ngrokTokenCtrl.dispose();
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

  void _enableNgrok() {
    final token = _ngrokTokenCtrl.text.trim();
    if (token.isEmpty) return;
    HapticFeedback.lightImpact();
    ref.read(remoteAccessProvider.notifier).enableNgrok(token);
  }

  void _disableNgrok() {
    HapticFeedback.lightImpact();
    ref.read(remoteAccessProvider.notifier).disableNgrok();
  }

  void _setAutoStart(bool v) {
    ref.read(remoteAccessProvider.notifier).setAutoStart(v);
  }

  void _fillAndConnect(String url, String token) {
    _urlCtrl.text = url;
    _tokenCtrl.text = token;
    _connect();
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(connectionStateProvider);
    final ra = ref.watch(remoteAccessProvider);

    return Scaffold(
      body: Stack(
        children: [
          const Positioned(
              top: -120, left: -40, child: LampGlow(size: 460, opacity: 0.9)),
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
                        child: Text('Memo',
                            style: Theme.of(context).textTheme.displayLarge),
                      ),
                      const SizedBox(height: 8),
                      Center(
                        child: Text(
                          'Pair with your desktop to reach\nyour second brain from here.',
                          textAlign: TextAlign.center,
                          style: MemoTheme.body(14.5,
                              color: MemoTheme.textDim, height: 1.5),
                        ),
                      ),
                      const SizedBox(height: 34),
                      _segmented(),
                      const SizedBox(height: 18),
                      if (!_remoteMode) ...[
                        _lanFields(state),
                      ] else ...[
                        _remoteFields(state, ra),
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
                      if (!_remoteMode) _connectButton(state.connecting),
                      const SizedBox(height: 18),
                      Center(
                        child: Text(
                          _remoteMode
                              ? 'Enable remote access on your desktop\nor enter existing URL and token manually.'
                              : 'Phone and desktop need to be on the same Wi-Fi.',
                          textAlign: TextAlign.center,
                          style: MemoTheme.body(12.5,
                              color: MemoTheme.textFaint, height: 1.45),
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

  Widget _lanFields(ConnectionState state) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _fieldLabel('DESKTOP ADDRESS'),
        const SizedBox(height: 8),
        TextField(
          controller: _urlCtrl,
          keyboardType: TextInputType.url,
          autocorrect: false,
          textInputAction: TextInputAction.go,
          style: MemoTheme.mono(14, color: MemoTheme.text),
          decoration: const InputDecoration(
            hintText: 'http://192.168.1.100:8090',
            prefixIcon: Icon(Icons.lan_outlined, size: 20),
          ),
          onSubmitted: (_) => _connect(),
        ),
      ],
    );
  }

  Widget _remoteFields(ConnectionState state, RemoteAccessState ra) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (ra.loading)
          const Center(
            child: Padding(
              padding: EdgeInsets.symmetric(vertical: 32),
              child: CircularProgressIndicator(strokeWidth: 2.5),
            ),
          )
        else ...[
          if (ra.error != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 16),
              child: _errorBox(ra.error!),
            ),
          if (ra.status != null) ...[
            Builder(builder: (_) {
              if (_ngrokTokenCtrl.text.isEmpty && ra.status!.ngrokToken.isNotEmpty) {
                _ngrokTokenCtrl.text = ra.status!.ngrokToken;
              }
              return const SizedBox.shrink();
            }),
            _remoteStatusCard(ra.status!, state),
          ],
          const SizedBox(height: 16),
          _fieldLabel('NGROK AUTH TOKEN'),
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _ngrokTokenCtrl,
                  autocorrect: false,
                  style: MemoTheme.mono(14, color: MemoTheme.text),
                  decoration: const InputDecoration(
                    hintText: '2hP2x...',
                    prefixIcon: Icon(Icons.vpn_key_outlined, size: 20),
                  ),
                ),
              ),
              const SizedBox(width: 10),
              if (ra.status == null || !ra.status!.enabled)
                _smallButton('Enable', () => _enableNgrok(),
                    busy: ra.enabling)
              else
                _smallButton('Disable', () => _disableNgrok(),
                    busy: ra.enabling, destructive: true),
            ],
          ),
          const SizedBox(height: 16),
          _fieldLabel('PUBLIC URL'),
          const SizedBox(height: 8),
          TextField(
            controller: _urlCtrl,
            keyboardType: TextInputType.url,
            autocorrect: false,
            textInputAction: TextInputAction.next,
            style: MemoTheme.mono(14, color: MemoTheme.text),
            decoration: const InputDecoration(
              hintText: 'https://abc123.ngrok.io',
              prefixIcon: Icon(Icons.public_outlined, size: 20),
            ),
          ),
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
          const SizedBox(height: 18),
          _connectButton(state.connecting),
        ],
      ],
    );
  }

  Widget _remoteStatusCard(RemoteAccessStatus status, ConnectionState state) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: MemoTheme.surfaceAlt,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: MemoTheme.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 10,
                height: 10,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  color: status.running ? MemoTheme.sage : MemoTheme.textFaint,
                ),
              ),
              const SizedBox(width: 8),
              Text(
                status.running ? 'Remote access active' : 'Remote access off',
                style: MemoTheme.body(13,
                    w: FontWeight.w600, color: MemoTheme.text),
              ),
            ],
          ),
          if (status.token.isNotEmpty) ...[
            const SizedBox(height: 8),
            _statusRow(Icons.key_outlined, 'Token', status.token),
          ],
          if (status.ngrokUrl.isNotEmpty) ...[
            const SizedBox(height: 6),
            GestureDetector(
              onTap: () {
                _fillAndConnect(status.ngrokUrl, status.token);
              },
              child: _statusRow(Icons.public_outlined, 'Ngrok',
                  status.ngrokUrl, tapHint: 'Tap to connect'),
            ),
          ],
          if (status.ngrokToken.isNotEmpty) ...[
            const SizedBox(height: 6),
            _statusRow(Icons.vpn_key_outlined, 'Saved token', status.ngrokToken),
          ],
          if (status.ngrokError.isNotEmpty) ...[
            const SizedBox(height: 6),
            _statusRow(Icons.error_outline, 'Error', status.ngrokError),
          ],
          const SizedBox(height: 10),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text('Auto-start on launch',
                  style: MemoTheme.body(13, color: MemoTheme.textDim)),
              Switch(
                value: status.ngrokAutoStart,
                onChanged: (v) => _setAutoStart(v),
                materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
              ),
            ],
          ),
          if (status.publicUrl != null &&
              state.baseUrl != status.publicUrl) ...[
            const SizedBox(height: 10),
            SizedBox(
              width: double.infinity,
              child: OutlinedButton.icon(
                onPressed: () =>
                    _fillAndConnect(status.publicUrl!, status.token),
                icon: const Icon(Icons.link, size: 16),
                label: const Text('Connect with this URL',
                    style: TextStyle(fontSize: 13)),
                style: OutlinedButton.styleFrom(
                  padding: const EdgeInsets.symmetric(vertical: 12),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(10)),
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _statusRow(IconData icon, String label, String value,
      {String? tapHint}) {
    return Row(
      children: [
        Icon(icon, size: 14, color: MemoTheme.textDim),
        const SizedBox(width: 6),
        Text('$label: ',
            style: MemoTheme.body(12,
                w: FontWeight.w600, color: MemoTheme.textDim)),
        Expanded(
          child: Text(
            value,
            style: MemoTheme.mono(12,
                color: tapHint != null ? MemoTheme.accent : MemoTheme.text),
            overflow: TextOverflow.ellipsis,
          ),
        ),
        if (tapHint != null)
          Padding(
            padding: const EdgeInsets.only(left: 4),
            child: Icon(Icons.tap_and_play, size: 13, color: MemoTheme.accent),
          ),
      ],
    );
  }

  Widget _smallButton(String label, VoidCallback onPressed,
      {bool busy = false, bool destructive = false}) {
    return SizedBox(
      height: 50,
      child: FilledButton(
        onPressed: busy ? null : onPressed,
        style: FilledButton.styleFrom(
          backgroundColor:
              destructive ? MemoTheme.error.withValues(alpha: 0.85) : MemoTheme.accent,
          foregroundColor: MemoTheme.onAccent,
          disabledBackgroundColor: MemoTheme.surfaceHi,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          padding: const EdgeInsets.symmetric(horizontal: 18),
        ),
        child: busy
            ? const SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(
                    strokeWidth: 2.5, color: MemoTheme.onAccent),
              )
            : Text(label,
                style: MemoTheme.body(13, w: FontWeight.w700)),
      ),
    );
  }

  Widget _fieldLabel(String s) =>
      Text(s, style: MemoTheme.mono(11, color: MemoTheme.textFaint, ls: 1.2));

  Widget _segmented() {
    Widget tab(String label, IconData icon, bool active, VoidCallback onTap) {
      return Expanded(
        child: GestureDetector(
          onTap: () {
            HapticFeedback.selectionClick();
            if (active) return;
            setState(() => _remoteMode = !_remoteMode);
            if (_remoteMode) {
              ref.read(remoteAccessProvider.notifier).loadStatus();
            }
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
                Icon(icon,
                    size: 17,
                    color: active ? MemoTheme.onAccent : MemoTheme.textDim),
                const SizedBox(width: 7),
                Text(label,
                    style: MemoTheme.body(14,
                        w: FontWeight.w600,
                        color:
                            active ? MemoTheme.onAccent : MemoTheme.textDim)),
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
          tab('Same network', Icons.wifi_rounded, !_remoteMode, () {}),
          tab('Remote', Icons.public_rounded, _remoteMode, () {}),
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
            child: Text(msg,
                style: MemoTheme.body(13, color: MemoTheme.error, height: 1.45)),
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
                width: 22,
                height: 22,
                child: CircularProgressIndicator(
                    strokeWidth: 2.5, color: MemoTheme.accent),
              )
            : Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Text('Connect',
                      style: MemoTheme.body(16,
                          w: FontWeight.w700, color: MemoTheme.onAccent)),
                  const SizedBox(width: 8),
                  const Icon(Icons.arrow_forward_rounded, size: 19),
                ],
              ),
      ),
    );
  }
}
