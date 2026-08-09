import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import 'dart:async';
import '../../../providers/chat_provider.dart';
import '../../../providers/settings_provider.dart';
import '../../../core/friendly_error.dart';

class RemoteAccessTab extends ConsumerStatefulWidget {
  const RemoteAccessTab({super.key});

  @override
  RemoteAccessTabState createState() => RemoteAccessTabState();
}

class RemoteAccessTabState extends ConsumerState<RemoteAccessTab> {
  final _ngrokTokenCtrl = TextEditingController();
  final _tsKeyCtrl = TextEditingController();
  final _tsHostCtrl = TextEditingController();
  final _backendUrlCtrl = TextEditingController();
  final _backendTokenCtrl = TextEditingController();
  int _listenPort = 8090;
  // Funnel defaults to on: without it, a phone can only reach the tunnel by
  // also installing the Tailscale app and joining the same tailnet, which
  // defeats the "just open a link" pitch mobile relies on (see
  // remote_ts_funnel_off_warning_body below for the user-facing version of
  // this).
  bool _tsFunnel = true;
  bool _tsBusy = false;
  bool _tsAdvanced = false;
  bool _ngrokAdvanced = false;
  String _tsPendingAuthUrl = '';
  bool _enabling = false;
  bool _disposed = false;

  // ── Auth mode (Faz 2) ──────────────────────────────────────────────
  final _authUsernameCtrl = TextEditingController();
  final _authPasswordCtrl = TextEditingController();
  String _authMode = 'token';
  bool _authModeFieldsLoaded =
      false; // one-time fill from server data, like _ngrokTokenCtrl above
  bool _authSaving = false;
  List<Map<String, dynamic>> _devices = [];
  bool _devicesLoading = false;
  String? _devicesError;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.invalidate(remoteAccessProvider);
      // Fetch the actual listen port from the backend instead of hardcoding 8090.
      ref.read(apiClientProvider).getListenPort().then((port) {
        if (mounted) setState(() => _listenPort = port);
      });
      _loadDevices();
    });
  }

  @override
  void dispose() {
    _disposed = true;
    _ngrokTokenCtrl.dispose();
    _tsKeyCtrl.dispose();
    _tsHostCtrl.dispose();
    _backendUrlCtrl.dispose();
    _backendTokenCtrl.dispose();
    _authUsernameCtrl.dispose();
    _authPasswordCtrl.dispose();
    super.dispose();
  }

  Future<void> _loadDevices() async {
    setState(() => _devicesLoading = true);
    try {
      final devices = await ref.read(apiClientProvider).listRemoteDevices();
      if (mounted) setState(() => _devices = devices);
    } catch (e) {
      if (mounted) {
        setState(() => _devicesError = FriendlyError.describeGeneric(e));
      }
    } finally {
      if (mounted) setState(() => _devicesLoading = false);
    }
  }

  Future<void> _saveAuthConfig() async {
    setState(() => _authSaving = true);
    try {
      await ref
          .read(apiClientProvider)
          .setRemoteAuthConfig(
            _authMode,
            username: _authUsernameCtrl.text.trim(),
            password: _authPasswordCtrl.text,
          );
      _authPasswordCtrl.clear();
      ref.invalidate(remoteAccessProvider);
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(L10n.t('remote_auth_saved'))));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              L10n.t('remote_auth_save_failed', {
                'e': FriendlyError.describeGeneric(e),
              }),
            ),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _authSaving = false);
    }
  }

  Future<void> _addDevice() async {
    final nameCtrl = TextEditingController();
    final name = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('remote_device_add_dialog_title')),
        content: TextField(
          controller: nameCtrl,
          autofocus: true,
          decoration: InputDecoration(
            hintText: L10n.t('remote_device_add_dialog_hint'),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(nameCtrl.text.trim()),
            child: Text(L10n.t('remote_device_add')),
          ),
        ],
      ),
    );
    nameCtrl.dispose();
    if (name == null || !mounted) return;
    try {
      final token = await ref.read(apiClientProvider).createRemoteDevice(name);
      await _loadDevices();
      if (mounted) await _showNewDeviceToken(token);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              L10n.t('remote_device_add_failed', {
                'e': FriendlyError.describeGeneric(e),
              }),
            ),
          ),
        );
      }
    }
  }

  Future<void> _showNewDeviceToken(String token) async {
    final theme = MemoTheme.of(context);
    await showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('remote_device_new_token_title')),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(L10n.t('remote_device_new_token_body')),
            const SizedBox(height: 14),
            _valueBox(
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      token,
                      style: const TextStyle(
                        fontFamily: 'JetBrainsMono',
                        fontSize: 13,
                        color: MemoTheme.accent,
                      ),
                    ),
                  ),
                  GestureDetector(
                    onTap: () {
                      Clipboard.setData(ClipboardData(text: token));
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(L10n.t('remote_token_copied'))),
                      );
                    },
                    child: Icon(
                      Icons.copy_rounded,
                      size: 18,
                      color: theme.textDim,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(L10n.t('ok')),
          ),
        ],
      ),
    );
  }

  Future<void> _revokeDevice(String id, String name) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('remote_device_revoke_confirm_title')),
        content: Text(
          L10n.t('remote_device_revoke_confirm_body', {'name': name}),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(
              L10n.t('remote_device_revoke'),
              style: const TextStyle(color: MemoTheme.red),
            ),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(apiClientProvider).revokeRemoteDevice(id);
      await _loadDevices();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              L10n.t('remote_device_revoke_failed', {
                'e': FriendlyError.describeGeneric(e),
              }),
            ),
          ),
        );
      }
    }
  }

  Future<void> _enableTailscale() async {
    setState(() {
      _tsBusy = true;
      _tsPendingAuthUrl = '';
    });
    try {
      await ref
          .read(apiClientProvider)
          .setTailscaleMode(
            true,
            _listenPort,
            authKey: _tsAdvanced ? _tsKeyCtrl.text.trim() : '',
            hostname: _tsHostCtrl.text.trim(),
            funnel: _tsFunnel,
          );
      // Interactive login (no auth key) waits on a human approving a browser
      // prompt, which can take minutes — poll instead of a single fixed
      // delay, and surface the pending login URL live so the fallback
      // "open login page" link/hint appears as soon as it's known.
      for (int i = 0; i < 300; i++) {
        if (_disposed) return;
        await Future.delayed(const Duration(seconds: 1));
        if (_disposed) return;
        try {
          final status = await ref.read(apiClientProvider).getRemoteAccess();
          final running = status['tailscale_running'] as bool? ?? false;
          final err = status['tailscale_error'] as String? ?? '';
          final authUrl = status['tailscale_auth_url'] as String? ?? '';
          if (mounted) setState(() => _tsPendingAuthUrl = authUrl);
          if (running || err.isNotEmpty) break;
        } catch (_) {}
      }
      ref.invalidate(remoteAccessProvider);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              L10n.t('remote_ts_error', {
                'e': FriendlyError.describeGeneric(e),
              }),
            ),
          ),
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          _tsBusy = false;
          _tsPendingAuthUrl = '';
        });
      }
    }
  }

  Future<void> _onFunnelChanged(bool v) async {
    if (v) {
      setState(() => _tsFunnel = v);
      return;
    }
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('remote_ts_funnel_off_warning_title')),
        content: Text(L10n.t('remote_ts_funnel_off_warning_body')),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(
              L10n.t('remote_ts_funnel_off_warning_confirm'),
              style: TextStyle(color: MemoTheme.red),
            ),
          ),
        ],
      ),
    );
    if (confirmed == true && mounted) {
      setState(() => _tsFunnel = false);
    }
  }

  Future<void> _openTsLoginLink(String url) async {
    final uri = Uri.tryParse(url);
    if (uri == null) return;
    await launchUrl(uri, mode: LaunchMode.externalApplication);
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

    // The backend URL/token fields are local SharedPreferences state
    // (backendUrlProvider/backendTokenProvider) — unlike everything else on
    // this tab, they don't depend on getRemoteAccess() actually succeeding.
    // They used to live inside _buildStatus, i.e. only reachable in the
    // `data` branch below: a user landing here specifically *because* the
    // configured backend is unreachable (e.g. from BackendUnreachableView's
    // "Sunucuyu Değiştir") would hit nothing but the bare error text and
    // never see the one control that could actually fix it. Rendered
    // unconditionally, first, instead.
    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        _label(L10n.t('remote_backend_url_label')),
        const SizedBox(height: 8),
        _buildBackendUrlSection(),
        const SizedBox(height: 32),
        raAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => Padding(
            padding: const EdgeInsets.symmetric(vertical: 16),
            child: Text(
              L10n.t('remote_load_failed', {'err': '$err'}),
              style: TextStyle(color: MemoTheme.red),
            ),
          ),
          data: (data) => _buildStatus(context, theme, data),
        ),
      ],
    );
  }

  Widget _buildStatus(
    BuildContext context,
    ThemeColors theme,
    Map<String, dynamic> data,
  ) {
    final enabled = data['enabled'] as bool? ?? false;
    final running = data['running'] as bool? ?? false;
    final token = data['token'] as String? ?? '';
    final ngrokUrl = data['ngrok_url'] as String? ?? '';
    final ngrokError = data['ngrok_error'] as String? ?? '';
    final addresses = (data['addresses'] as List?)?.cast<String>() ?? [];
    final savedNgrokToken = data['ngrok_token'] as String? ?? '';
    final ngrokAutoStart = data['ngrok_auto_start'] as bool? ?? false;
    final authWarning = data['auth_warning'] as String? ?? '';
    if (_ngrokTokenCtrl.text.isEmpty && savedNgrokToken.isNotEmpty) {
      _ngrokTokenCtrl.text = savedNgrokToken;
    }
    if (!_authModeFieldsLoaded) {
      _authMode = data['auth_mode'] as String? ?? 'token';
      _authUsernameCtrl.text = data['username'] as String? ?? '';
      _authModeFieldsLoaded = true;
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
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
              running
                  ? L10n.t('remote_status_active')
                  : L10n.t('remote_status_off'),
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
          _label(L10n.t('remote_access_token_label')),
          const SizedBox(height: 6),
          _valueBox(
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    token,
                    style: TextStyle(
                      fontFamily: 'JetBrainsMono',
                      fontSize: 13,
                      color: MemoTheme.accent,
                    ),
                  ),
                ),
                GestureDetector(
                  onTap: () {
                    Clipboard.setData(ClipboardData(text: token));
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(L10n.t('remote_token_copied'))),
                    );
                  },
                  child: Icon(
                    Icons.copy_rounded,
                    size: 18,
                    color: theme.textDim,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
        ],

        if (authWarning.isNotEmpty) ...[
          Container(
            width: double.infinity,
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            decoration: BoxDecoration(
              color: MemoTheme.warningOrange.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: MemoTheme.warningOrange),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Icon(
                  Icons.warning_amber_rounded,
                  color: MemoTheme.warningOrange,
                  size: 20,
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    L10n.t('remote_auth_warning_banner'),
                    style: const TextStyle(
                      color: MemoTheme.warningOrange,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
        ],

        // ── Auth mode (Faz 2) ───────────────────────────────────────────
        _buildAuthSection(context, theme),
        const SizedBox(height: 24),

        // ── Paired devices (Faz 2) ──────────────────────────────────────
        _buildDevicesSection(context, theme),
        const SizedBox(height: 24),

        // ── Tailscale (embedded, stable URL) ──────────────────────────
        _buildTailscaleSection(context, theme, data),
        const SizedBox(height: 24),

        // ── ngrok (advanced/fallback option, collapsed by default) ─────
        _buildNgrokSection(
          context,
          theme,
          enabled: enabled,
          ngrokUrl: ngrokUrl,
          ngrokError: ngrokError,
          addresses: addresses,
          savedNgrokToken: savedNgrokToken,
          ngrokAutoStart: ngrokAutoStart,
        ),
      ],
    );
  }

  Widget _buildAuthSection(BuildContext context, ThemeColors theme) {
    final needsCredentials =
        _authMode == 'password' || _authMode == 'token_password';
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            L10n.t('remote_auth_section_title'),
            style: TextStyle(
              fontSize: 15,
              fontWeight: FontWeight.w600,
              color: theme.textMain,
            ),
          ),
          const SizedBox(height: 14),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              _authModeChip('none', L10n.t('remote_auth_mode_none')),
              _authModeChip('token', L10n.t('remote_auth_mode_token')),
              _authModeChip('password', L10n.t('remote_auth_mode_password')),
              _authModeChip(
                'token_password',
                L10n.t('remote_auth_mode_token_password'),
              ),
            ],
          ),
          if (needsCredentials) ...[
            const SizedBox(height: 16),
            _label(L10n.t('remote_auth_username_label')),
            const SizedBox(height: 6),
            TextField(
              controller: _authUsernameCtrl,
              decoration: const InputDecoration(isDense: true),
            ),
            const SizedBox(height: 12),
            _label(L10n.t('remote_auth_password_label')),
            const SizedBox(height: 6),
            TextField(
              controller: _authPasswordCtrl,
              obscureText: true,
              decoration: InputDecoration(
                isDense: true,
                hintText: L10n.t('remote_auth_password_hint'),
              ),
            ),
          ],
          const SizedBox(height: 16),
          Align(
            alignment: Alignment.centerRight,
            child: FilledButton(
              onPressed: _authSaving ? null : _saveAuthConfig,
              child: _authSaving
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(L10n.t('remote_auth_save')),
            ),
          ),
        ],
      ),
    );
  }

  Widget _authModeChip(String mode, String label) {
    final selected = _authMode == mode;
    return ChoiceChip(
      label: Text(label),
      selected: selected,
      onSelected: (_) => setState(() => _authMode = mode),
    );
  }

  Widget _buildDevicesSection(BuildContext context, ThemeColors theme) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  L10n.t('remote_devices_section_title'),
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w600,
                    color: theme.textMain,
                  ),
                ),
              ),
              TextButton.icon(
                onPressed: _addDevice,
                icon: const Icon(Icons.add_rounded, size: 18),
                label: Text(L10n.t('remote_device_add')),
              ),
            ],
          ),
          if (_devicesLoading)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 12),
              child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
            )
          else if (_devicesError != null)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Text(
                _devicesError!,
                style: TextStyle(color: MemoTheme.red),
              ),
            )
          else if (_devices.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Text(
                L10n.t('remote_devices_empty'),
                style: TextStyle(color: theme.textDim),
              ),
            )
          else
            ..._devices.map((d) => _deviceRow(theme, d)),
        ],
      ),
    );
  }

  Widget _deviceRow(ThemeColors theme, Map<String, dynamic> device) {
    final id = device['id'] as String? ?? '';
    final name = device['name'] as String? ?? '';
    final lastSeen = device['last_seen_at'] as String? ?? '';
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  name,
                  style: TextStyle(
                    color: theme.textMain,
                    fontWeight: FontWeight.w500,
                  ),
                ),
                Text(
                  lastSeen.isEmpty
                      ? L10n.t('remote_device_never_seen')
                      : L10n.t('remote_device_last_seen', {'when': lastSeen}),
                  style: TextStyle(color: theme.textDim, fontSize: 12),
                ),
              ],
            ),
          ),
          TextButton(
            onPressed: () => _revokeDevice(id, name),
            child: Text(
              L10n.t('remote_device_revoke'),
              style: const TextStyle(color: MemoTheme.red),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildBackendUrlSection() {
    final theme = MemoTheme.of(context);
    final currentUrl = ref.watch(backendUrlProvider);
    final currentToken = ref.watch(backendTokenProvider);
    if (_backendUrlCtrl.text.isEmpty) {
      _backendUrlCtrl.text = currentUrl;
    }
    if (_backendTokenCtrl.text.isEmpty) {
      _backendTokenCtrl.text = currentToken;
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
                  decoration: InputDecoration(
                    labelText: L10n.t('remote_backend_url_field_label'),
                    hintText: 'http://127.0.0.1:8090',
                    isDense: true,
                    prefixIcon: const Icon(Icons.link, size: 18),
                  ),
                  style: TextStyle(
                    fontFamily: 'JetBrainsMono',
                    fontSize: 14,
                    color: theme.textMain,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          // Only needed when the URL above points at a backend running with
          // --lan/remote access enabled on its own (a Docker/CasaOS
          // deployment, another machine's exposed backend) — such a backend
          // requires this token on every request from the very first one, so
          // unlike a same-origin remote-access setup there is no automatic
          // way for this app to learn it; it has to be pasted in by hand.
          Row(
            children: [
              Icon(Icons.key_outlined, size: 18, color: theme.textDim),
              const SizedBox(width: 8),
              Expanded(
                child: TextField(
                  controller: _backendTokenCtrl,
                  decoration: InputDecoration(
                    labelText: L10n.t('remote_backend_token_field_label'),
                    hintText: 'memo-...',
                    isDense: true,
                    prefixIcon: const Icon(Icons.vpn_key_outlined, size: 18),
                  ),
                  style: TextStyle(
                    fontFamily: 'JetBrainsMono',
                    fontSize: 14,
                    color: theme.textMain,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            L10n.t('remote_backend_token_field_hint'),
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),
          const SizedBox(height: 8),
          SizedBox(
            width: double.infinity,
            child: FilledButton.tonalIcon(
              onPressed: () async {
                await ref
                    .read(backendUrlProvider.notifier)
                    .save(_backendUrlCtrl.text);
                await ref
                    .read(backendTokenProvider.notifier)
                    .save(_backendTokenCtrl.text);
                ref.invalidate(apiClientProvider);
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                      content: Text(L10n.t('remote_backend_url_updated')),
                      duration: const Duration(seconds: 2),
                    ),
                  );
                }
              },
              icon: const Icon(Icons.save_outlined, size: 16),
              label: Text(L10n.t('apply')),
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

  // ngrok used to be the primary/only remote-access option; now that
  // Tailscale+Funnel covers the same "just open a link" case with no
  // tailnet setup needed, ngrok is kept only as a collapsed fallback for
  // cases Tailscale doesn't cover (e.g. no Tailscale account, or a
  // per-request ephemeral URL is actually wanted). Auto-expands if ngrok
  // is already configured/running, so an existing setup isn't hidden.
  Widget _buildNgrokSection(
    BuildContext context,
    ThemeColors theme, {
    required bool enabled,
    required String ngrokUrl,
    required String ngrokError,
    required List<String> addresses,
    required String savedNgrokToken,
    required bool ngrokAutoStart,
  }) {
    final expanded = _ngrokAdvanced || enabled;

    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: theme.bgElement,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: enabled ? MemoTheme.accent : theme.borderSoft,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          InkWell(
            onTap: enabled
                ? null
                : () => setState(() => _ngrokAdvanced = !_ngrokAdvanced),
            child: Row(
              children: [
                Icon(
                  Icons.public_rounded,
                  size: 18,
                  color: enabled ? MemoTheme.accent : theme.textDim,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    L10n.t('remote_ngrok_advanced_title'),
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontWeight: FontWeight.w600,
                      color: theme.textMain,
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                if (enabled)
                  Container(
                    width: 10,
                    height: 10,
                    decoration: const BoxDecoration(
                      shape: BoxShape.circle,
                      color: MemoTheme.green,
                    ),
                  )
                else
                  Icon(
                    expanded
                        ? Icons.expand_less_rounded
                        : Icons.expand_more_rounded,
                    size: 20,
                    color: theme.textDim,
                  ),
              ],
            ),
          ),
          const SizedBox(height: 4),
          Text(
            L10n.t('remote_ngrok_advanced_desc'),
            style: TextStyle(fontSize: 12, color: theme.textDim),
          ),

          if (expanded) ...[
            const SizedBox(height: 12),
            if (ngrokUrl.isNotEmpty) ...[
              _label(L10n.t('remote_ngrok_tunnel_url_label')),
              const SizedBox(height: 6),
              _valueBox(
                borderColor: MemoTheme.accent,
                child: Row(
                  children: [
                    Icon(
                      Icons.public_rounded,
                      size: 16,
                      color: MemoTheme.accent,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        ngrokUrl,
                        style: TextStyle(
                          fontFamily: 'JetBrainsMono',
                          fontSize: 13,
                          color: MemoTheme.accentLight,
                        ),
                      ),
                    ),
                    GestureDetector(
                      onTap: () {
                        Clipboard.setData(ClipboardData(text: ngrokUrl));
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text(L10n.t('remote_url_copied'))),
                        );
                      },
                      child: Icon(
                        Icons.copy_rounded,
                        size: 18,
                        color: theme.textDim,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 12),
            ],
            if (savedNgrokToken.isNotEmpty) ...[
              _label(L10n.t('remote_ngrok_token_saved_label')),
              const SizedBox(height: 6),
              _valueBox(
                child: Row(
                  children: [
                    Icon(
                      Icons.vpn_key_outlined,
                      size: 16,
                      color: theme.textDim,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        savedNgrokToken,
                        style: TextStyle(
                          fontFamily: 'JetBrainsMono',
                          fontSize: 13,
                          color: theme.textMain,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 12),
            ],

            if (ngrokError.isNotEmpty) ...[
              Container(
                padding: const EdgeInsets.all(14),
                decoration: BoxDecoration(
                  color: MemoTheme.red.withValues(alpha: 0.10),
                  borderRadius: BorderRadius.circular(10),
                  border: Border.all(
                    color: MemoTheme.red.withValues(alpha: 0.35),
                  ),
                ),
                child: Row(
                  children: [
                    Icon(
                      Icons.error_outline_rounded,
                      size: 18,
                      color: MemoTheme.red,
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Text(
                        ngrokError,
                        style: TextStyle(fontSize: 13, color: MemoTheme.red),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 12),
            ],

            if (addresses.isNotEmpty) ...[
              _label(L10n.t('remote_local_addresses_label')),
              const SizedBox(height: 6),
              ...addresses.map(
                (addr) => Padding(
                  padding: const EdgeInsets.only(bottom: 4),
                  child: Text(
                    addr,
                    style: TextStyle(
                      fontFamily: 'JetBrainsMono',
                      fontSize: 12,
                      color: theme.textDim,
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 12),
            ],

            _label(L10n.t('remote_autostart_label')),
            const SizedBox(height: 6),
            SwitchListTile(
              title: Text(
                L10n.t('remote_autostart_ngrok_title'),
                style: TextStyle(fontSize: 13, color: theme.textMain),
              ),
              subtitle: Text(
                ngrokAutoStart
                    ? L10n.t('remote_autostart_will_start')
                    : L10n.t('remote_autostart_manual'),
                style: TextStyle(fontSize: 11, color: theme.textDim),
              ),
              value: ngrokAutoStart,
              onChanged: (v) => _setAutoStart(v),
              dense: true,
              contentPadding: EdgeInsets.zero,
            ),
            const SizedBox(height: 12),
            _label(L10n.t('remote_configure_label')),
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _ngrokTokenCtrl,
                    decoration: InputDecoration(
                      labelText: L10n.t('remote_ngrok_token_field_label'),
                      hintText: '2hP2x...',
                      prefixIcon: const Icon(Icons.vpn_key_outlined, size: 20),
                      isDense: true,
                    ),
                    style: TextStyle(
                      fontFamily: 'JetBrainsMono',
                      fontSize: 14,
                      color: theme.textMain,
                    ),
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
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.power_off_rounded, size: 18),
                    label: Text(
                      _enabling ? '...' : L10n.t('remote_disable_btn'),
                    ),
                    style: FilledButton.styleFrom(
                      backgroundColor: MemoTheme.red,
                      foregroundColor: theme.textInverse,
                      padding: const EdgeInsets.symmetric(
                        horizontal: 20,
                        vertical: 12,
                      ),
                    ),
                  )
                else
                  FilledButton.icon(
                    onPressed: _enabling ? null : () => _enable(),
                    icon: _enabling
                        ? const SizedBox(
                            width: 16,
                            height: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(
                            Icons.power_settings_new_rounded,
                            size: 18,
                          ),
                    label: Text(
                      _enabling ? '...' : L10n.t('remote_enable_start_btn'),
                    ),
                    style: FilledButton.styleFrom(
                      backgroundColor: MemoTheme.accent,
                      foregroundColor: theme.textInverse,
                      padding: const EdgeInsets.symmetric(
                        horizontal: 20,
                        vertical: 12,
                      ),
                    ),
                  ),
              ],
            ),

            if (!enabled) ...[
              const SizedBox(height: 12),
              Text(
                L10n.t('remote_ngrok_hint_text'),
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
            ],
          ],
        ],
      ),
    );
  }

  Widget _buildTailscaleSection(
    BuildContext context,
    ThemeColors theme,
    Map<String, dynamic> data,
  ) {
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
          color: tsRunning ? MemoTheme.accent : theme.borderSoft,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.hub_outlined,
                size: 18,
                color: tsRunning ? MemoTheme.accent : theme.textDim,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  L10n.t('remote_tailscale_title'),
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontWeight: FontWeight.w600,
                    color: theme.textMain,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              if (tsRunning)
                Container(
                  width: 10,
                  height: 10,
                  decoration: const BoxDecoration(
                    shape: BoxShape.circle,
                    color: MemoTheme.green,
                  ),
                ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            L10n.t('remote_tailscale_desc'),
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
                    child: Text(
                      tsUrl,
                      style: TextStyle(
                        fontFamily: 'JetBrainsMono',
                        fontSize: 13,
                        color: MemoTheme.accent,
                      ),
                    ),
                  ),
                  GestureDetector(
                    onTap: () {
                      Clipboard.setData(ClipboardData(text: tsUrl));
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(L10n.t('remote_url_copied'))),
                      );
                    },
                    child: Icon(
                      Icons.copy_rounded,
                      size: 18,
                      color: theme.textDim,
                    ),
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
                    child: Text(
                      '$tsIp  ${L10n.t('remote_ts_ip_note')}',
                      style: TextStyle(
                        fontFamily: 'JetBrainsMono',
                        fontSize: 12.5,
                        color: theme.textMain,
                      ),
                    ),
                  ),
                  GestureDetector(
                    onTap: () {
                      Clipboard.setData(ClipboardData(text: tsIp));
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(L10n.t('remote_ip_copied'))),
                      );
                    },
                    child: Icon(
                      Icons.copy_rounded,
                      size: 18,
                      color: theme.textDim,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 12),
          ],

          if (tsError.isNotEmpty) ...[
            Text(
              L10n.t('remote_ts_error', {'e': tsError}),
              style: TextStyle(fontSize: 12, color: MemoTheme.red),
            ),
            const SizedBox(height: 12),
          ],

          if (!tsRunning) ...[
            if (_tsBusy && _tsPendingAuthUrl.isNotEmpty) ...[
              Container(
                padding: const EdgeInsets.all(12),
                margin: const EdgeInsets.only(bottom: 12),
                decoration: BoxDecoration(
                  color: MemoTheme.accent.withValues(alpha: 0.08),
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(
                    color: MemoTheme.accent.withValues(alpha: 0.35),
                  ),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      L10n.t('remote_ts_awaiting_login'),
                      style: TextStyle(fontSize: 12, color: theme.textMain),
                    ),
                    const SizedBox(height: 8),
                    TextButton.icon(
                      onPressed: () => _openTsLoginLink(_tsPendingAuthUrl),
                      icon: const Icon(Icons.open_in_new_rounded, size: 16),
                      label: Text(L10n.t('remote_ts_open_login_link')),
                    ),
                  ],
                ),
              ),
            ],
            TextButton(
              onPressed: () => setState(() => _tsAdvanced = !_tsAdvanced),
              style: TextButton.styleFrom(
                padding: EdgeInsets.zero,
                alignment: Alignment.centerLeft,
              ),
              child: Text(
                L10n.t('remote_ts_advanced_toggle'),
                style: TextStyle(fontSize: 12, color: theme.textDim),
              ),
            ),
            if (_tsAdvanced) ...[
              const SizedBox(height: 4),
              Text(
                L10n.t('remote_ts_manual_key_hint'),
                style: TextStyle(fontSize: 11, color: theme.textDim),
              ),
              const SizedBox(height: 8),
              TextField(
                controller: _tsKeyCtrl,
                decoration: InputDecoration(
                  labelText: L10n.t('remote_ts_auth_key_label'),
                  hintText: 'tskey-auth-...',
                  prefixIcon: const Icon(Icons.vpn_key_outlined, size: 20),
                  isDense: true,
                ),
                style: TextStyle(
                  fontFamily: 'JetBrainsMono',
                  fontSize: 13,
                  color: theme.textMain,
                ),
              ),
              const SizedBox(height: 10),
            ],
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _tsHostCtrl,
                    decoration: InputDecoration(
                      labelText: L10n.t('remote_ts_device_name_label'),
                      hintText: 'memo',
                      isDense: true,
                    ),
                    style: TextStyle(fontSize: 13, color: theme.textMain),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: SwitchListTile(
                    title: Text(
                      L10n.t('remote_ts_funnel_title'),
                      style: TextStyle(fontSize: 12, color: theme.textMain),
                    ),
                    subtitle: Text(
                      L10n.t('remote_ts_funnel_desc'),
                      style: TextStyle(fontSize: 10, color: theme.textDim),
                    ),
                    value: _tsFunnel,
                    onChanged: _onFunnelChanged,
                    dense: true,
                    contentPadding: EdgeInsets.zero,
                    activeThumbColor: MemoTheme.accent,
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
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.power_settings_new_rounded, size: 18),
              label: Text(
                _tsBusy
                    ? L10n.t('remote_ts_starting')
                    : L10n.t('remote_ts_start_btn'),
              ),
              style: FilledButton.styleFrom(
                backgroundColor: MemoTheme.accent,
                foregroundColor: theme.textInverse,
                padding: const EdgeInsets.symmetric(
                  horizontal: 20,
                  vertical: 12,
                ),
              ),
            ),
          ] else
            FilledButton.tonalIcon(
              onPressed: _tsBusy ? null : _disableTailscale,
              icon: _tsBusy
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.power_off_rounded, size: 18),
              label: Text(L10n.t('remote_ts_stop_btn')),
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
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              L10n.t('remote_action_failed', {
                'e': FriendlyError.describeGeneric(e),
              }),
            ),
          ),
        );
      }
    }
  }

  void _enable() async {
    final token = _ngrokTokenCtrl.text.trim();
    if (token.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('remote_enter_token_first'))),
      );
      return;
    }
    setState(() => _enabling = true);
    try {
      await ref
          .read(apiClientProvider)
          .setRemoteAccess(
            true,
            _listenPort,
            ngrokMode: true,
            ngrokToken: token,
          );
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
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              L10n.t('remote_action_failed', {
                'e': FriendlyError.describeGeneric(e),
              }),
            ),
          ),
        );
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
          SnackBar(
            content: Text(
              L10n.t('remote_action_failed', {
                'e': FriendlyError.describeGeneric(e),
              }),
            ),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _enabling = false);
    }
  }

  Widget _label(String s) {
    return Text(
      s,
      style: TextStyle(
        fontSize: 11,
        fontWeight: FontWeight.w600,
        color: MemoTheme.of(context).textDim,
        letterSpacing: 1.2,
      ),
    );
  }
}
