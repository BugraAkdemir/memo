import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/auth_gate_provider.dart';
import '../providers/chat_provider.dart';
import '../providers/settings_provider.dart';

/// App-wide gate: shows the first-run setup screen or the login screen
/// whenever the backend requires a credential the app doesn't have yet.
/// Rendered above BackendUnreachableOverlay in app_shell's Stack — a 401
/// is "need credentials", not "no backend".
class AuthGateOverlay extends ConsumerWidget {
  const AuthGateOverlay({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final info = ref.watch(authGateProvider).valueOrNull;
    if (info == null) return const SizedBox.shrink();
    return switch (info.state) {
      AuthGateState.setupNeeded => const _GateScaffold(child: _SetupGateView()),
      AuthGateState.loginNeeded => _GateScaffold(child: _LoginGateView(mode: info.authMode)),
      AuthGateState.ok => const SizedBox.shrink(),
    };
  }
}

/// Full-screen container matching the BackendUnreachableView look: Memo
/// background, centered card, branded title.
class _GateScaffold extends StatelessWidget {
  const _GateScaffold({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Container(
      color: theme.bgApp,
      alignment: Alignment.center,
      padding: const EdgeInsets.all(24),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 480),
        child: child,
      ),
    );
  }
}

/// Shared card wrapper for the two gate forms.
class _GateCard extends StatelessWidget {
  const _GateCard({required this.title, required this.child});

  final String title;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Material(
      color: Colors.transparent,
      child: Container(
        padding: const EdgeInsets.all(28),
        decoration: BoxDecoration(
          color: theme.bgPanel,
          borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
          border: Border.all(color: theme.borderSoft),
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
            Text(
              title,
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.bold,
                color: theme.textMain,
              ),
            ),
            const SizedBox(height: 8),
            child,
          ],
        ),
      ),
    );
  }
}

/// First-run gate: asks whether other devices will access this Memo; if yes,
/// a 3-way sign-in method picker (password / password+token / token-only)
/// drives the backend setup calls.
class _SetupGateView extends ConsumerStatefulWidget {
  const _SetupGateView();

  @override
  ConsumerState<_SetupGateView> createState() => _SetupGateViewState();
}

class _SetupGateViewState extends ConsumerState<_SetupGateView> {
  int _step = 0; // 0: başka cihaz sorusu, 1: yöntem + form, 2: token kartı
  String _method = 'password'; // password | token_password | token
  final _username = TextEditingController();
  final _password = TextEditingController();
  final _confirm = TextEditingController();
  final _tokenInput = TextEditingController();
  bool _busy = false;
  bool _copied = false;
  String? _error;
  String? _generatedToken;

  Future<void> _decline() async {
    final prefs = ref.read(prefsProvider);
    await prefs.setBool(authSetupDoneKey, true);
    ref.invalidate(authGateProvider);
  }

  Future<void> _submit() async {
    final api = ref.read(apiClientProvider);
    final prefs = ref.read(prefsProvider);
    if (_method != 'token') {
      if (_password.text != _confirm.text) {
        setState(() => _error = L10n.t('auth_gate_error_password_mismatch'));
        return;
      }
      if (_username.text.trim().isEmpty || _password.text.isEmpty) {
        setState(() => _error = L10n.t('auth_gate_error_generic', {'err': ''}));
        return;
      }
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      if (_method == 'token') {
        // Sadece token: mode=token + device token üret; kullanıcı token'ı
        // sonraki adımda alana yapıştırır.
        await api.setRemoteAuthConfig('token');
        final plain = await api.createRemoteDevice('Auth setup');
        await prefs.setBool(authSetupDoneKey, true);
        if (!mounted) return;
        setState(() {
          _generatedToken = plain;
          _step = 2;
          _busy = false;
        });
        return;
      }
      final token = await api.setupCreateAdmin(
        _username.text.trim(),
        _password.text,
      );
      api.setSessionToken(token);
      if (_method == 'token_password') {
        await api.setRemoteAuthConfig(
          'token_password',
          username: _username.text.trim(),
          password: _password.text,
        );
        final plain = await api.createRemoteDevice('Auth setup');
        await prefs.setBool(authSetupDoneKey, true);
        if (!mounted) return;
        setState(() {
          _generatedToken = plain;
          _step = 2;
          _busy = false;
        });
        return;
      }
      await prefs.setBool(authSetupDoneKey, true);
      ref.invalidate(authGateProvider);
    } on DioException catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = e.response?.statusCode == 403
            ? L10n.t('auth_gate_error_create_failed')
            : L10n.t('auth_gate_error_generic', {'err': '$e'});
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = L10n.t('auth_gate_error_generic', {'err': '$e'});
      });
    }
  }

  Future<void> _enterToken() async {
    final api = ref.read(apiClientProvider);
    api.setSessionToken(_tokenInput.text.trim());
    ref.invalidate(authGateProvider);
  }

  Future<void> _copyToken() async {
    await Clipboard.setData(ClipboardData(text: _generatedToken ?? ''));
    setState(() => _copied = true);
  }

  @override
  void dispose() {
    _username.dispose();
    _password.dispose();
    _confirm.dispose();
    _tokenInput.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return switch (_step) {
      0 => _GateCard(
          title: L10n.t('auth_gate_setup_title'),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                L10n.t('auth_gate_privacy_note'),
                style: TextStyle(color: theme.textDim, height: 1.5, fontSize: 13),
              ),
              const SizedBox(height: 24),
              Text(
                L10n.t('auth_gate_other_devices_question'),
                style: TextStyle(
                  color: theme.textMain,
                  fontSize: 15,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 12),
              _GateChoiceButton(
                label: L10n.t('auth_gate_other_devices_yes'),
                icon: Icons.phone_android_rounded,
                onPressed: () => setState(() => _step = 1),
              ),
              const SizedBox(height: 8),
              _GateChoiceButton(
                label: L10n.t('auth_gate_other_devices_no'),
                icon: Icons.desktop_windows_rounded,
                onPressed: _decline,
              ),
            ],
          ),
        ),
      1 => _GateCard(
          title: L10n.t('auth_gate_setup_title'),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                L10n.t('auth_gate_method_label'),
                style: TextStyle(
                  color: theme.textMain,
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 8),
              _MethodOption(
                title: L10n.t('auth_gate_method_password'),
                desc: L10n.t('auth_gate_method_password_desc'),
                selected: _method == 'password',
                onTap: () => setState(() => _method = 'password'),
              ),
              const SizedBox(height: 4),
              _MethodOption(
                title: L10n.t('auth_gate_method_token_password'),
                desc: L10n.t('auth_gate_method_token_password_desc'),
                selected: _method == 'token_password',
                onTap: () => setState(() => _method = 'token_password'),
              ),
              const SizedBox(height: 4),
              _MethodOption(
                title: L10n.t('auth_gate_method_token'),
                desc: L10n.t('auth_gate_method_token_desc'),
                selected: _method == 'token',
                onTap: () => setState(() => _method = 'token'),
              ),
              const SizedBox(height: 16),
              if (_method != 'token') ...[
                _Field(
                  controller: _username,
                  label: L10n.t('auth_gate_username'),
                  icon: Icons.person_outline_rounded,
                ),
                const SizedBox(height: 10),
                _Field(
                  controller: _password,
                  label: L10n.t('auth_gate_password'),
                  icon: Icons.lock_outline_rounded,
                  obscure: true,
                ),
                const SizedBox(height: 10),
                _Field(
                  controller: _confirm,
                  label: L10n.t('auth_gate_confirm_password'),
                  icon: Icons.lock_outline_rounded,
                  obscure: true,
                ),
              ],
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(
                  _error!,
                  style: TextStyle(color: Colors.redAccent, fontSize: 13),
                  textAlign: TextAlign.center,
                ),
              ],
              const SizedBox(height: 16),
              SizedBox(
                height: 44,
                child: _busy
                    ? const Center(
                        child: SizedBox(
                          width: 22,
                          height: 22,
                          child: CircularProgressIndicator(strokeWidth: 2.5),
                        ),
                      )
                    : ElevatedButton(
                        onPressed: _submit,
                        style: ElevatedButton.styleFrom(
                          backgroundColor: MemoTheme.accent,
                          foregroundColor: theme.textInverse,
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                          ),
                        ),
                        child: Text(
                          _method == 'token'
                              ? L10n.t('auth_gate_generate_token')
                              : L10n.t('auth_gate_create'),
                          style: const TextStyle(fontWeight: FontWeight.w600),
                        ),
                      ),
              ),
            ],
          ),
        ),
      2 => _GateCard(
          title: L10n.t('auth_gate_token_generated_title'),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                L10n.t('auth_gate_token_generated_body'),
                style: TextStyle(color: theme.textDim, height: 1.5, fontSize: 13),
              ),
              const SizedBox(height: 16),
              Container(
                padding: const EdgeInsets.all(14),
                decoration: BoxDecoration(
                  color: theme.bgApp,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                  border: Border.all(color: theme.borderSoft),
                ),
                child: SelectableText(
                  _generatedToken ?? '',
                  style: const TextStyle(fontFamily: 'JetBrainsMono', fontSize: 12),
                  textAlign: TextAlign.center,
                ),
              ),
              const SizedBox(height: 10),
              OutlinedButton(
                onPressed: _copyToken,
                child: Text(
                  _copied ? L10n.t('auth_gate_token_copied') : L10n.t('auth_gate_token_copy'),
                ),
              ),
              const SizedBox(height: 20),
              _Field(
                controller: _tokenInput,
                label: L10n.t('auth_gate_enter_token'),
                icon: Icons.vpn_key_outlined,
                hint: L10n.t('auth_gate_token_enter_hint'),
              ),
              const SizedBox(height: 14),
              SizedBox(
                height: 44,
                child: ElevatedButton(
                  onPressed: _enterToken,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: MemoTheme.accent,
                    foregroundColor: theme.textInverse,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    ),
                  ),
                  child: Text(
                    L10n.t('auth_gate_sign_in'),
                    style: const TextStyle(fontWeight: FontWeight.w600),
                  ),
                ),
              ),
            ],
          ),
        ),
      _ => const SizedBox.shrink(),
    };
  }
}

/// Login gate: renders a form matching the configured auth mode — password,
/// token, or password+token with a segment switch.
class _LoginGateView extends ConsumerStatefulWidget {
  const _LoginGateView({required this.mode});

  final String mode;

  @override
  ConsumerState<_LoginGateView> createState() => _LoginGateViewState();
}

class _LoginGateViewState extends ConsumerState<_LoginGateView> {
  final _username = TextEditingController();
  final _password = TextEditingController();
  final _tokenInput = TextEditingController();
  bool _busy = false;
  String? _error;
  var _tab = 0; // 0 şifre, 1 token (yalnız token_password'de görünür)

  Future<void> _loginPassword() async {
    final api = ref.read(apiClientProvider);
    final prefs = ref.read(prefsProvider);
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final res = await api.login(_username.text.trim(), _password.text);
      if (res.sessionToken.isEmpty) throw Exception('empty token');
      api.setSessionToken(res.sessionToken);
      await prefs.setString('memo_session_role', res.role);
      ref.invalidate(authGateProvider);
    } on DioException catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = switch (e.response?.statusCode) {
          401 => L10n.t('auth_gate_error_invalid_credentials'),
          429 => L10n.t('auth_gate_error_locked'),
          _ => L10n.t('auth_gate_error_generic', {'err': '$e'}),
        };
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = L10n.t('auth_gate_error_generic', {'err': '$e'});
      });
    }
  }

  Future<void> _loginToken() async {
    final api = ref.read(apiClientProvider);
    api.setSessionToken(_tokenInput.text.trim());
    ref.invalidate(authGateProvider);
  }

  @override
  void dispose() {
    _username.dispose();
    _password.dispose();
    _tokenInput.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return _GateCard(
      title: L10n.t('auth_gate_sign_in'),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (widget.mode == 'token_password') ...[
            SegmentedButton<int>(
              segments: [
                ButtonSegment(
                  value: 0,
                  label: Text(L10n.t('auth_gate_login_tab_password')),
                ),
                ButtonSegment(
                  value: 1,
                  label: Text(L10n.t('auth_gate_login_tab_token')),
                ),
              ],
              selected: {_tab},
              onSelectionChanged: (s) => setState(() {
                _tab = s.first;
                _error = null;
              }),
            ),
            const SizedBox(height: 16),
          ],
          if (widget.mode == 'token' || (_tab == 1 && widget.mode == 'token_password')) ...[
            _Field(
              controller: _tokenInput,
              label: L10n.t('auth_gate_enter_token'),
              icon: Icons.vpn_key_outlined,
              hint: L10n.t('auth_gate_token_enter_hint'),
            ),
          ] else ...[
            _Field(
              controller: _username,
              label: L10n.t('auth_gate_username'),
              icon: Icons.person_outline_rounded,
            ),
            const SizedBox(height: 10),
            _Field(
              controller: _password,
              label: L10n.t('auth_gate_password'),
              icon: Icons.lock_outline_rounded,
              obscure: true,
            ),
          ],
          if (widget.mode == 'password') ...[
            const SizedBox(height: 10),
            Text(
              L10n.t('auth_gate_token_hint_password_mode'),
              style: TextStyle(color: theme.textDim, fontSize: 12),
            ),
          ],
          if (_error != null) ...[
            const SizedBox(height: 12),
            Text(
              _error!,
              style: TextStyle(color: Colors.redAccent, fontSize: 13),
              textAlign: TextAlign.center,
            ),
          ],
          const SizedBox(height: 16),
          SizedBox(
            height: 44,
            child: _busy
                ? const Center(
                    child: SizedBox(
                      width: 22,
                      height: 22,
                      child: CircularProgressIndicator(strokeWidth: 2.5),
                    ),
                  )
                : ElevatedButton(
                    onPressed: widget.mode == 'token' || _tab == 1
                        ? _loginToken
                        : _loginPassword,
                    style: ElevatedButton.styleFrom(
                      backgroundColor: MemoTheme.accent,
                      foregroundColor: theme.textInverse,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                      ),
                    ),
                    child: Text(
                      L10n.t('auth_gate_sign_in'),
                      style: const TextStyle(fontWeight: FontWeight.w600),
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}

class _GateChoiceButton extends StatelessWidget {
  const _GateChoiceButton({
    required this.label,
    required this.icon,
    required this.onPressed,
  });

  final String label;
  final IconData icon;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return OutlinedButton.icon(
      onPressed: onPressed,
      icon: Icon(icon, size: 20, color: MemoTheme.accent),
      label: Text(label),
      style: OutlinedButton.styleFrom(
        foregroundColor: theme.textMain,
        side: BorderSide(color: theme.borderSoft),
        padding: const EdgeInsets.symmetric(vertical: 14),
        alignment: Alignment.centerLeft,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        ),
      ),
    );
  }
}

class _MethodOption extends StatelessWidget {
  const _MethodOption({
    required this.title,
    required this.desc,
    required this.selected,
    required this.onTap,
  });

  final String title;
  final String desc;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        decoration: BoxDecoration(
          color: selected ? MemoTheme.accentPale.withValues(alpha: 0.35) : null,
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          border: Border.all(
            color: selected ? MemoTheme.accent : theme.borderSoft,
          ),
        ),
        child: Row(
          children: [
            Icon(
              selected ? Icons.radio_button_checked : Icons.radio_button_off,
              size: 18,
              color: selected ? MemoTheme.accent : theme.textDim,
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    style: TextStyle(
                      color: theme.textMain,
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  Text(
                    desc,
                    style: TextStyle(color: theme.textDim, fontSize: 12),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Field extends StatelessWidget {
  const _Field({
    required this.controller,
    required this.label,
    required this.icon,
    this.obscure = false,
    this.hint,
  });

  final TextEditingController controller;
  final String label;
  final IconData icon;
  final bool obscure;
  final String? hint;

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return TextField(
      controller: controller,
      obscureText: obscure,
      decoration: InputDecoration(
        labelText: label,
        hintText: hint,
        prefixIcon: Icon(icon, size: 18),
        filled: true,
        fillColor: theme.bgApp,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          borderSide: BorderSide(color: theme.borderSoft),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          borderSide: BorderSide(color: theme.borderSoft),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          borderSide: const BorderSide(color: MemoTheme.accent),
        ),
      ),
    );
  }
}
