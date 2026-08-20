import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/whatsapp_provider.dart';
import 'whatsapp_screen.dart';

/// Lists every messaging/integration connection Memo supports. Currently
/// just WhatsApp — this screen exists so the nav's old dedicated "WhatsApp"
/// slot can host a second connection type later without needing another
/// top-level nav item. Tapping a connection pushes its own screen on top.
class ConnectionsScreen extends ConsumerWidget {
  const ConnectionsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final waStatus = ref.watch(whatsAppStatusProvider).valueOrNull;
    final waConnected = waStatus?.connected == true && waStatus?.loggedIn == true;

    return Container(
      color: c.bgApp,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            height: 52,
            padding: const EdgeInsets.symmetric(horizontal: 16),
            decoration: BoxDecoration(
              color: c.bgPanel,
              border: Border(bottom: BorderSide(color: c.borderSoft)),
            ),
            alignment: Alignment.centerLeft,
            child: Text(
              L10n.t('connections_title'),
              style: TextStyle(
                fontSize: 15,
                fontWeight: FontWeight.w700,
                color: c.textMain,
              ),
            ),
          ),
          Expanded(
            child: ListView(
              padding: const EdgeInsets.all(16),
              children: [
                _ConnectionTile(
                  icon: Icons.message_rounded,
                  title: 'WhatsApp',
                  subtitle: waConnected
                      ? L10n.t('connections_status_connected')
                      : L10n.t('connections_status_not_connected'),
                  connected: waConnected,
                  onTap: () => Navigator.of(context).push(
                    MaterialPageRoute(
                      builder: (_) => const _WhatsAppRoute(),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// WhatsAppScreen was built as an always-mounted IndexedStack tab body — no
/// Scaffold, no back button, leaving via a different nav item was the only
/// way out. Pushed as its own route here instead (so it isn't permanently
/// resident once a second connection type exists), it needs that back
/// affordance itself, since the pushed route covers AppShell's NavRail/
/// floating-hamburger entirely. Mirrors TasksScreen's own header pattern
/// (conditional back arrow + title bar) rather than a Material AppBar, to
/// match the rest of the app's custom-header style.
class _WhatsAppRoute extends StatelessWidget {
  const _WhatsAppRoute();

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Scaffold(
      backgroundColor: c.bgApp,
      body: SafeArea(
        child: Column(
          children: [
            Container(
              height: 52,
              padding: const EdgeInsets.symmetric(horizontal: 8),
              decoration: BoxDecoration(
                color: c.bgPanel,
                border: Border(bottom: BorderSide(color: c.borderSoft)),
              ),
              child: Row(
                children: [
                  IconButton(
                    icon: Icon(Icons.arrow_back, color: c.textSecondary),
                    onPressed: () => Navigator.of(context).pop(),
                  ),
                  const Icon(Icons.message_rounded, size: 18, color: MemoTheme.accent),
                  const SizedBox(width: 8),
                  Text(
                    'WhatsApp',
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                      color: c.textMain,
                    ),
                  ),
                ],
              ),
            ),
            const Expanded(child: WhatsAppScreen()),
          ],
        ),
      ),
    );
  }
}

class _ConnectionTile extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final bool connected;
  final VoidCallback onTap;

  const _ConnectionTile({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.connected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          onTap: onTap,
          child: Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: c.bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(color: c.borderSoft),
            ),
            child: Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: MemoTheme.accent.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Icon(icon, color: MemoTheme.accent, size: 20),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        title,
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: c.textMain,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        subtitle,
                        style: TextStyle(fontSize: 12, color: c.textDim),
                      ),
                    ],
                  ),
                ),
                Container(
                  width: 8,
                  height: 8,
                  margin: const EdgeInsets.only(right: 10),
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: connected ? MemoTheme.green : c.textDim,
                  ),
                ),
                Icon(Icons.chevron_right, color: c.textDim, size: 20),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
