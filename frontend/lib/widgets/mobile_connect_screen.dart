import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/backend_url.dart';
import '../core/l10n.dart';
import '../core/platform_capabilities.dart';
import '../core/scope_restart.dart';
import '../core/server_probe.dart';
import '../core/theme.dart';
import '../providers/settings_provider.dart';

/// Phone-only: the very first screen of a fresh install, before anything
/// else in the app. A phone never runs Memo's backend, so until the user has
/// told it where one lives the rest of the app has nothing to talk to.
///
/// Two steps, then the existing auth gate takes over:
///  1. the user types an IP or domain (port optional — see
///     [normalizeBackendUrl]: none means 8090, a typed port is respected);
///  2. the server is asked for its unauthenticated setup status. Success saves
///     the address and rebuilds the provider scope ([softRestartScope]) so the
///     whole app starts against it; the auth gate then shows the first-run
///     setup, or the password / token / password+token sign-in with "remember
///     me", whichever the server is configured for. Failure stays here and
///     says why.
///
/// Desktop and web never show this: [shouldShowMobileConnect] is false there.
class MobileConnectOverlay extends ConsumerWidget {
  const MobileConnectOverlay({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final url = ref.watch(backendUrlProvider);
    if (!shouldShowMobileConnect(isMobile: isMobilePlatform, savedUrl: url)) {
      return const SizedBox.shrink();
    }
    return const MobileConnectScreen();
  }
}

/// A saved address means one was reached at least once (the screen only saves
/// after a successful probe), so the screen is needed exactly when a phone has
/// none.
bool shouldShowMobileConnect({required bool isMobile, required String savedUrl}) =>
    isMobile && savedUrl.trim().isEmpty;

typedef ServerProbe = Future<ServerProbeResult> Function(String baseUrl);

class MobileConnectScreen extends ConsumerStatefulWidget {
  const MobileConnectScreen({super.key, this.probe = _defaultProbe});

  /// Test seam; production probes with a throwaway [MemoApiClient].
  final ServerProbe probe;

  static Future<ServerProbeResult> _defaultProbe(String url) => probeServer(url);

  @override
  ConsumerState<MobileConnectScreen> createState() => _MobileConnectScreenState();
}

class _MobileConnectScreenState extends ConsumerState<MobileConnectScreen> {
  final _controller = TextEditingController();
  bool _checking = false;
  ServerProbeResult? _failure;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  /// What the typed text will be turned into, or '' when it is empty.
  String get _resolved {
    final text = _controller.text.trim();
    return text.isEmpty ? '' : normalizeBackendUrl(text);
  }

  Future<void> _next() async {
    final url = _resolved;
    if (url.isEmpty || _checking) return;
    setState(() {
      _checking = true;
      _failure = null;
    });
    final result = await widget.probe(url);
    if (!mounted) return;
    if (!result.ok) {
      setState(() {
        _checking = false;
        _failure = result;
      });
      return;
    }
    await ref.read(backendUrlProvider.notifier).save(url);
    // Every provider that grabbed the (address-less) client at start has to be
    // rebuilt against the saved address; this replaces this very screen, so
    // nothing may touch state after it.
    softRestartScope();
  }

  String _reason(ServerProbeFailure f) => L10n.t(switch (f) {
        ServerProbeFailure.dns => 'connect_fail_dns',
        ServerProbeFailure.refused => 'connect_fail_refused',
        ServerProbeFailure.timeout => 'connect_fail_timeout',
        ServerProbeFailure.tls => 'connect_fail_tls',
        ServerProbeFailure.unreachable => 'connect_fail_unreachable',
        ServerProbeFailure.notMemo => 'connect_fail_not_memo',
        ServerProbeFailure.unknown => 'connect_fail_unknown',
      });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final resolved = _resolved;
    final failure = _failure;
    return Material(
      color: theme.bgApp,
      child: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 480),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Icon(Icons.dns_outlined, size: 40, color: MemoTheme.accent),
                  const SizedBox(height: 16),
                  Text(
                    L10n.t('connect_title'),
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 22,
                      fontWeight: FontWeight.bold,
                      color: theme.textMain,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    L10n.t('connect_subtitle'),
                    textAlign: TextAlign.center,
                    style: TextStyle(color: theme.textDim, height: 1.5, fontSize: 13),
                  ),
                  const SizedBox(height: 24),
                  TextField(
                    controller: _controller,
                    enabled: !_checking,
                    autofocus: true,
                    keyboardType: TextInputType.url,
                    autocorrect: false,
                    enableSuggestions: false,
                    textInputAction: TextInputAction.go,
                    onChanged: (_) => setState(() => _failure = null),
                    onSubmitted: (_) => _next(),
                    style: const TextStyle(fontFamily: 'JetBrainsMono', fontSize: 14),
                    decoration: InputDecoration(
                      labelText: L10n.t('connect_address_label'),
                      // An example address, not translatable prose.
                      hintText: '192.168.1.50',
                      prefixIcon: const Icon(Icons.link, size: 18),
                    ),
                  ),
                  if (resolved.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    Text(
                      L10n.t('connect_will_use', {'url': resolved}),
                      style: TextStyle(
                        color: theme.textDim,
                        fontSize: 12,
                        fontFamily: 'JetBrainsMono',
                      ),
                    ),
                  ],
                  if (failure != null) ...[
                    const SizedBox(height: 16),
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: MemoTheme.red.withValues(alpha: 0.08),
                        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                        border: Border.all(
                          color: MemoTheme.red.withValues(alpha: 0.35),
                        ),
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            L10n.t('connect_fail_title'),
                            style: TextStyle(
                              color: MemoTheme.red,
                              fontWeight: FontWeight.w600,
                              fontSize: 13,
                            ),
                          ),
                          const SizedBox(height: 4),
                          Text(
                            _reason(failure.failure!),
                            style: TextStyle(
                              color: theme.textMain,
                              fontSize: 13,
                              height: 1.45,
                            ),
                          ),
                          if (failure.detail.isNotEmpty) ...[
                            const SizedBox(height: 6),
                            Text(
                              failure.detail,
                              style: TextStyle(
                                color: theme.textDim,
                                fontSize: 11,
                                fontFamily: 'JetBrainsMono',
                              ),
                            ),
                          ],
                        ],
                      ),
                    ),
                  ],
                  const SizedBox(height: 20),
                  SizedBox(
                    height: 48,
                    child: _checking
                        ? Row(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              const SizedBox(
                                width: 20,
                                height: 20,
                                child: CircularProgressIndicator(strokeWidth: 2.5),
                              ),
                              const SizedBox(width: 12),
                              Text(
                                L10n.t('connect_checking'),
                                style: TextStyle(color: theme.textDim),
                              ),
                            ],
                          )
                        : ElevatedButton(
                            onPressed: resolved.isEmpty ? null : _next,
                            style: ElevatedButton.styleFrom(
                              backgroundColor: MemoTheme.accent,
                              foregroundColor: theme.textInverse,
                              shape: RoundedRectangleBorder(
                                borderRadius:
                                    BorderRadius.circular(MemoTheme.radiusMd),
                              ),
                            ),
                            child: Text(
                              L10n.t(failure == null ? 'next' : 'retry'),
                              style: const TextStyle(fontWeight: FontWeight.w600),
                            ),
                          ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
