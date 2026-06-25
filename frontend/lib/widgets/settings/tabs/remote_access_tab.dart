import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import '../../../core/theme.dart';
import 'dart:async';
import '../../../providers/chat_provider.dart';
import '../../../providers/settings_provider.dart';

class RemoteAccessTab extends ConsumerStatefulWidget {
  @override
  RemoteAccessTabState createState() => RemoteAccessTabState();
}

class RemoteAccessTabState extends ConsumerState<RemoteAccessTab> {
  final _ngrokTokenCtrl = TextEditingController();
  final _tsKeyCtrl = TextEditingController();
  final _tsHostCtrl = TextEditingController();
  final _backendUrlCtrl = TextEditingController();
  int _listenPort = 8090;
  bool _tsFunnel = false;
  bool _tsBusy = false;
  bool _enabling = false;
  bool _disposed = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.invalidate(remoteAccessProvider);
      // Fetch the actual listen port from the backend instead of hardcoding 8090.
      ref.read(apiClientProvider).getListenPort().then((port) {
        if (mounted) setState(() => _listenPort = port);
      });
    });
  }

  @override
  void dispose() {
    _disposed = true;
    _ngrokTokenCtrl.dispose();
    _tsKeyCtrl.dispose();
    _tsHostCtrl.dispose();
    _backendUrlCtrl.dispose();
    super.dispose();
  }

  Future<void> _enableTailscale() async {
    setState(() => _tsBusy = true);
    try {
      await ref.read(apiClientProvider).setTailscaleMode(
            true,
            _listenPort,
            authKey: _tsKeyCtrl.text.trim(),
            hostname: _tsHostCtrl.text.trim(),
            funnel: _tsFunnel,
          );
      await Future.delayed(const Duration(seconds: 2));
      ref.invalidate(remoteAccessProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Tailscale hatası: $e')));
      }
    } finally {
      if (mounted) setState(() => _tsBusy = false);
    }
  }

  Future<void> _disableTailscale() async {
    setState(() => _tsBusy = true);
    try {
      await ref.read(apiClientProvider).setTailscaleMode(false, _listenPort);
      ref.invalidate(remoteAccessProvider);
    } catch (_) {
    } finally {
      if (mounted) setState(() => _tsBusy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final raAsync = ref.watch(remoteAccessProvider);

    return raAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Text('Failed to load: $err',
              style: TextStyle(color: MemoTheme.red)),
        ),
      ),
      data: (data) => _buildStatus(context, theme, data),
    );
  }

  Widget _buildStatus(BuildContext context, ThemeColors theme, Map<String, dynamic> data) {
    final enabled = data['enabled'] as bool? ?? false;
    final running = data['running'] as bool? ?? false;
    final token = data['token'] as String? ?? '';
    final ngrokUrl = data['ngrok_url'] as String? ?? '';
    final ngrokError = data['ngrok_error'] as String? ?? '';
    final addresses = (data['addresses'] as List?)?.cast<String>() ?? [];
    final savedNgrokToken = data['ngrok_token'] as String? ?? '';
    final ngrokAutoStart = data['ngrok_auto_start'] as bool? ?? false;
    if (_ngrokTokenCtrl.text.isEmpty && savedNgrokToken.isNotEmpty) {
      _ngrokTokenCtrl.text = savedNgrokToken;
    }

    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Row(
          children: [
            Container(
              width: 12,
              height: 12,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: running ? MemoTheme.green : theme.textDim,
              ),
            ),
            const SizedBox(width: 10),
            Text(
              running ? 'Remote access active' : 'Remote access off',
              style: TextStyle(
                fontSize: 15,
                fontWeight: FontWeight.w600,
                color: theme.textMain,
              ),
            ),
          ],
        ),
        const SizedBox(height: 24),

        if (token.isNotEmpty) ...[
          _label('Access Token'),
          const SizedBox(height: 6),
          _valueBox(
            child: Row(
              children: [
                Expanded(
                  child: Text(token,
                      style: TextStyle(
                          fontFamily: 'JetBrainsMono',
                          fontSize: 13,
                          color: MemoTheme.accent)),
                ),
                GestureDetector(
                  onTap: () {
                    Clipboard.setData(ClipboardData(text: token));
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('Token copied')),
                    );
                  },
                  child: Icon(Icons.copy_rounded, size: 18, color: theme.textDim),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
        ],

        // ── Beta features toggle ──────────────────────────────────────
        SwitchListTile(
          title: Text('Beta Özellikler',
              style: TextStyle(fontSize: 13, color: theme.textMain)),
          subtitle: Text(
            'Deneysel özellikleri aç (örn. Tailscale tüneli)',
            style: TextStyle(fontSize: 11, color: theme.textDim),
          ),
          value: data['beta'] as bool? ?? false,
          onChanged: (v) async {
            await ref.read(apiClientProvider).setBeta(v);
            ref.invalidate(remoteAccessProvider);
          },
          dense: true,
          contentPadding: EdgeInsets.zero,
          activeColor: MemoTheme.accent,
        ),
        const SizedBox(height: 12),

        // ── Tailscale (embedded, stable URL) — beta only ──────────────
        if (data['beta'] as bool? ?? false) ...[
          _buildTailscaleSection(context, theme, data),
          const SizedBox(height: 24),
        ],

        if (ngrokUrl.isNotEmpty) ...[
          _label('Ngrok Tunnel URL'),
          const SizedBox(height: 6),
          _valueBox(
            borderColor: MemoTheme.accent,
            child: Row(
              children: [
                Icon(Icons.public_rounded, size: 16, color: MemoTheme.accent),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(ngrokUrl,
                      style: TextStyle(
                          fontFamily: 'JetBrainsMono',
                          fontSize: 13,
                          color: MemoTheme.accentLight)),
                ),
                GestureDetector(
                  onTap: () {
                    Clipboard.setData(ClipboardData(text: ngrokUrl));
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('URL copied')),
                    );
                  },
                  child: Icon(Icons.copy_rounded, size: 18, color: theme.textDim),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
        ],
        if (savedNgrokToken.isNotEmpty) ...[
          _label('Ngrok Auth Token (saved)'),
          const SizedBox(height: 6),
          _valueBox(
            child: Row(
              children: [
                Icon(Icons.vpn_key_outlined, size: 16, color: theme.textDim),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(savedNgrokToken,
                      style: TextStyle(
                          fontFamily: 'JetBrainsMono',
                          fontSize: 13,
                          color: theme.textMain)),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
        ],

        if (ngrokError.isNotEmpty) ...[
          Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: MemoTheme.red.withValues(alpha: 0.10),
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: MemoTheme.red.withValues(alpha: 0.35)),
            ),
            child: Row(
              children: [
                Icon(Icons.error_outline_rounded, size: 18, color: MemoTheme.red),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(ngrokError,
                      style: TextStyle(fontSize: 13, color: MemoTheme.red)),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
        ],

        if (addresses.isNotEmpty) ...[
          _label('Local Addresses'),
          const SizedBox(height: 6),
          ...addresses.map((addr) => Padding(
                padding: const EdgeInsets.only(bottom: 4),
                child: Text(addr,
                    style: TextStyle(
                        fontFamily: 'JetBrainsMono',
                        fontSize: 12,
                        color: theme.textDim)),
              )),
          const SizedBox(height: 20),
        ],

        _label('Auto-Start on Backend Launch'),
        const SizedBox(height: 6),
        SwitchListTile(
          title: Text('Start ngrok tunnel automatically',
              style: TextStyle(fontSize: 13, color: theme.textMain)),
          subtitle: Text(
            ngrokAutoStart
                ? 'Will start on next backend launch'
                : 'Start manually from this panel',
            style: TextStyle(fontSize: 11, color: theme.textDim),
          ),
          value: ngrokAutoStart,
          onChanged: (v) => _setAutoStart(v),
          dense: true,
          contentPadding: EdgeInsets.zero,
        ),
        const SizedBox(height: 20),
        _label('Configure Remote Access'),
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              child: TextField(
                controller: _ngrokTokenCtrl,
                decoration: const InputDecoration(
                  labelText: 'Ngrok Auth Token',
                  hintText: '2hP2x...',
                  prefixIcon: Icon(Icons.vpn_key_outlined, size: 20),
                ),
                style: TextStyle(
                    fontFamily: 'JetBrainsMono',
                    fontSize: 14,
                    color: theme.textMain),
              ),
            ),
            const SizedBox(width: 12),
            if (enabled)
              FilledButton.tonalIcon(
                onPressed: _enabling ? null : () => _disable(),
                icon: _enabling
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2))
                    : const Icon(Icons.power_off_rounded, size: 18),
                label: Text(_enabling ? '...' : 'Disable'),
                style: FilledButton.styleFrom(
                  backgroundColor: MemoTheme.red,
                  foregroundColor: theme.textInverse,
                  padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
                ),
              )
            else
              FilledButton.icon(
                onPressed: _enabling ? null : () => _enable(),
                icon: _enabling
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2))
                    : const Icon(Icons.power_settings_new_rounded, size: 18),
                label: Text(_enabling ? '...' : 'Enable & Start'),
                style: FilledButton.styleFrom(
                  backgroundColor: MemoTheme.accent,
                  foregroundColor: theme.textInverse,
                  padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
                ),
              ),
          ],
        ),

        if (!enabled) ...[
          const SizedBox(height: 12),
          Text(
            'Enter your ngrok auth token to start a public tunnel.\n'
            'Get it from https://dashboard.ngrok.com',
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),
        ],

        const SizedBox(height: 32),
        _label('Backend Server URL'),
        const SizedBox(height: 8),
        _buildBackendUrlSection(),
      ],
    );
  }

  Widget _buildBackendUrlSection() {
    final theme = MemoTheme.of(context);
    final currentUrl = ref.watch(backendUrlProvider);
    if (_backendUrlCtrl.text.isEmpty) {
      _backendUrlCtrl.text = currentUrl;
    }

    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.dns_outlined, size: 18, color: theme.textDim),
              const SizedBox(width: 8),
              Expanded(
                child: TextField(
                  controller: _backendUrlCtrl,
                  decoration: const InputDecoration(
                    labelText: 'Backend URL',
                    hintText: 'http://127.0.0.1:8090',
                    isDense: true,
                    prefixIcon: Icon(Icons.link, size: 18),
                  ),
                  style: TextStyle(
                      fontFamily: 'JetBrainsMono',
                      fontSize: 14,
                      color: theme.textMain),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          SizedBox(
            width: double.infinity,
            child: FilledButton.tonalIcon(
              onPressed: () async {
                await ref.read(backendUrlProvider.notifier).save(_backendUrlCtrl.text);
                ref.invalidate(apiClientProvider);
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(
                      content: Text('Backend URL updated. Reconnect if needed.'),
                      duration: Duration(seconds: 2),
                    ),
                  );
                }
              },
              icon: const Icon(Icons.save_outlined, size: 16),
              label: const Text('Apply'),
              style: FilledButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: 12),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _valueBox({Widget? child, Color? borderColor}) {
    final theme = MemoTheme.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: borderColor ?? theme.borderSoft),
      ),
      child: child,
    );
  }

  Widget _buildTailscaleSection(
      BuildContext context, ThemeColors theme, Map<String, dynamic> data) {
    final tsUrl = data['tailscale_url'] as String? ?? '';
    final tsIp = data['tailscale_ip'] as String? ?? '';
    final tsRunning = data['tailscale_running'] as bool? ?? false;
    final tsError = data['tailscale_error'] as String? ?? '';
    final savedHost = data['tailscale_hostname'] as String? ?? 'memo';
    if (_tsHostCtrl.text.isEmpty) _tsHostCtrl.text = savedHost;

    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
            color: tsRunning ? MemoTheme.accent : theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.hub_outlined,
                  size: 18,
                  color: tsRunning ? MemoTheme.accent : theme.textDim),
              const SizedBox(width: 8),
              Text('Tailscale (sabit URL, gömülü)',
                  style: TextStyle(
                      fontWeight: FontWeight.w600, color: theme.textMain)),
              const Spacer(),
              if (tsRunning)
                Container(
                  width: 10,
                  height: 10,
                  decoration: const BoxDecoration(
                      shape: BoxShape.circle, color: MemoTheme.green),
                ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            'ngrok\'un aksine URL hiç değişmez ve ayrı binary indirmez. '
            'Tek seferlik bir auth key gerekir (login.tailscale.com → Settings → Keys).',
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),
          const SizedBox(height: 12),

          if (tsUrl.isNotEmpty) ...[
            _valueBox(
              borderColor: MemoTheme.accent,
              child: Row(
                children: [
                  Icon(Icons.public_rounded, size: 16, color: MemoTheme.accent),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(tsUrl,
                        style: TextStyle(
                            fontFamily: 'JetBrainsMono',
                            fontSize: 13,
                            color: MemoTheme.accent)),
                  ),
                  GestureDetector(
                    onTap: () {
                      Clipboard.setData(ClipboardData(text: tsUrl));
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('URL kopyalandı')),
                      );
                    },
                    child:
                        Icon(Icons.copy_rounded, size: 18, color: theme.textDim),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 8),
          ],

          if (tsIp.isNotEmpty) ...[
            _valueBox(
              child: Row(
                children: [
                  Icon(Icons.lan_outlined, size: 16, color: theme.textDim),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text('$tsIp  (MagicDNS kapalıysa bunu kullan)',
                        style: TextStyle(
                            fontFamily: 'JetBrainsMono',
                            fontSize: 12.5,
                            color: theme.textMain)),
                  ),
                  GestureDetector(
                    onTap: () {
                      Clipboard.setData(ClipboardData(text: tsIp));
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('IP kopyalandı')),
                      );
                    },
                    child:
                        Icon(Icons.copy_rounded, size: 18, color: theme.textDim),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 12),
          ],

          if (tsError.isNotEmpty) ...[
            Text('Hata: $tsError',
                style: TextStyle(fontSize: 12, color: MemoTheme.red)),
            const SizedBox(height: 12),
          ],

          if (!tsRunning) ...[
            TextField(
              controller: _tsKeyCtrl,
              decoration: const InputDecoration(
                labelText: 'Tailscale Auth Key',
                hintText: 'tskey-auth-...',
                prefixIcon: Icon(Icons.vpn_key_outlined, size: 20),
                isDense: true,
              ),
              style: TextStyle(
                  fontFamily: 'JetBrainsMono', fontSize: 13, color: theme.textMain),
            ),
            const SizedBox(height: 10),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _tsHostCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Cihaz adı',
                      hintText: 'memo',
                      isDense: true,
                    ),
                    style: TextStyle(fontSize: 13, color: theme.textMain),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: SwitchListTile(
                    title: Text('Funnel (public)',
                        style: TextStyle(fontSize: 12, color: theme.textMain)),
                    subtitle: Text('Telefona kurulum gerekmez',
                        style: TextStyle(fontSize: 10, color: theme.textDim)),
                    value: _tsFunnel,
                    onChanged: (v) => setState(() => _tsFunnel = v),
                    dense: true,
                    contentPadding: EdgeInsets.zero,
                    activeColor: MemoTheme.accent,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            FilledButton.icon(
              onPressed: _tsBusy ? null : _enableTailscale,
              icon: _tsBusy
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.power_settings_new_rounded, size: 18),
              label: Text(_tsBusy ? 'Başlatılıyor...' : 'Tailscale ile Başlat'),
              style: FilledButton.styleFrom(
                backgroundColor: MemoTheme.accent,
                foregroundColor: theme.textInverse,
                padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
              ),
            ),
          ] else
            FilledButton.tonalIcon(
              onPressed: _tsBusy ? null : _disableTailscale,
              icon: _tsBusy
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2))
                  : const Icon(Icons.power_off_rounded, size: 18),
              label: const Text('Tailscale Durdur'),
              style: FilledButton.styleFrom(
                backgroundColor: MemoTheme.red,
                foregroundColor: theme.textInverse,
              ),
            ),
        ],
      ),
    );
  }

  void _setAutoStart(bool v) async {
    try {
      await ref.read(apiClientProvider).setRemoteAccessAutoStart(v);
      // If enabling, poll until ngrok URL appears
      if (v) {
        for (int i = 0; i < 30; i++) {
          if (_disposed) return;
          await Future.delayed(const Duration(seconds: 1));
          if (_disposed) return;
          try {
            final status = await ref.read(apiClientProvider).getRemoteAccess();
            final url = status['ngrok_url'] as String? ?? '';
            final err = status['ngrok_error'] as String? ?? '';
            if (url.isNotEmpty || err.isNotEmpty) break;
          } catch (_) {}
        }
      }
      ref.invalidate(remoteAccessProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Failed: $e')));
      }
    }
  }

  void _enable() async {
    final token = _ngrokTokenCtrl.text.trim();
    if (token.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Enter ngrok auth token first')),
      );
      return;
    }
    setState(() => _enabling = true);
    try {
      await ref.read(apiClientProvider).setRemoteAccess(true, _listenPort,
          ngrokMode: true, ngrokToken: token);
      // Poll until ngrok URL appears or error (takes a few seconds)
      for (int i = 0; i < 30; i++) {
        if (_disposed) return;
        await Future.delayed(const Duration(seconds: 1));
        if (_disposed) return;
        try {
          final status = await ref.read(apiClientProvider).getRemoteAccess();
          final url = status['ngrok_url'] as String? ?? '';
          final err = status['ngrok_error'] as String? ?? '';
          if (url.isNotEmpty || err.isNotEmpty) break;
        } catch (_) {}
      }
      ref.invalidate(remoteAccessProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('Failed: $e')));
      }
    } finally {
      if (mounted) setState(() => _enabling = false);
    }
  }

  void _disable() async {
    setState(() => _enabling = true);
    try {
      await ref.read(apiClientProvider).setRemoteAccess(false, _listenPort);
      ref.invalidate(remoteAccessProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _enabling = false);
    }
  }

  Widget _label(String s) {
    return Text(s,
        style: TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textDim,
            letterSpacing: 1.2));
  }
}
