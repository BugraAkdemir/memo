import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';
import '../providers/connection_provider.dart';
import '../screens/settings_screen.dart';
import 'branding.dart';

class SessionDrawer extends ConsumerWidget {
  const SessionDrawer({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(chatProvider);
    final connection = ref.watch(connectionStateProvider);
    final host = Uri.tryParse(connection.baseUrl)?.host ?? connection.baseUrl;

    return Drawer(
      backgroundColor: MemoTheme.surface,
      width: MediaQuery.of(context).size.width * 0.84,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.horizontal(right: Radius.circular(24)),
      ),
      child: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Header: identity + live connection ──
            Padding(
              padding: const EdgeInsets.fromLTRB(18, 18, 18, 14),
              child: Row(
                children: [
                  const MemoLogo(size: 40),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(L10n.t('app_name'), style: Theme.of(context).textTheme.titleLarge),
                        const SizedBox(height: 3),
                        Row(
                          children: [
                            const StatusDot(color: MemoTheme.sage, size: 6, pulse: true),
                            const SizedBox(width: 6),
                            Flexible(
                              child: Text(host,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: MemoTheme.mono(11, color: MemoTheme.textFaint)),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),

            // ── New chat ──
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 14),
              child: SizedBox(
                width: double.infinity,
                child: FilledButton.icon(
                  onPressed: () {
                    HapticFeedback.selectionClick();
                    ref.read(chatProvider.notifier).newChat();
                    Navigator.pop(context);
                  },
                  icon: const Icon(Icons.add_rounded, size: 19),
                  label: Text(L10n.t('new_chat')),
                  style: FilledButton.styleFrom(
                    backgroundColor: MemoTheme.accent,
                    foregroundColor: MemoTheme.onAccent,
                    alignment: Alignment.centerLeft,
                    padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 13),
                    textStyle: MemoTheme.body(14.5, w: FontWeight.w600),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(13)),
                  ),
                ),
              ),
            ),

            const SizedBox(height: 18),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 22),
              child: Text(L10n.t('recent'), style: MemoTheme.mono(10.5, color: MemoTheme.textFaint, ls: 1.4)),
            ),
            const SizedBox(height: 8),

            // ── Sessions ──
            Expanded(
              child: state.sessions.isEmpty
                  ? Center(
                      child: Padding(
                        padding: const EdgeInsets.all(24),
                        child: Text(L10n.t('no_conversations'),
                            textAlign: TextAlign.center,
                            style: MemoTheme.body(13.5, color: MemoTheme.textFaint, height: 1.5)),
                      ),
                    )
                  : ListView.builder(
                      padding: const EdgeInsets.symmetric(horizontal: 10),
                      itemCount: state.sessions.length,
                      itemBuilder: (context, index) {
                        final s = state.sessions[index];
                        final active = s.id == state.activeSessionId;
                        return Padding(
                          padding: const EdgeInsets.only(bottom: 2),
                          child: Material(
                            color: active ? MemoTheme.accent.withValues(alpha: 0.12) : Colors.transparent,
                            borderRadius: BorderRadius.circular(11),
                            child: InkWell(
                              borderRadius: BorderRadius.circular(11),
                              onTap: () {
                                ref.read(chatProvider.notifier).switchChat(s.id);
                                Navigator.pop(context);
                              },
                              child: Container(
                                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 11),
                                child: Row(
                                  children: [
                                    Icon(
                                      active ? Icons.chat_bubble_rounded : Icons.chat_bubble_outline_rounded,
                                      size: 17,
                                      color: active ? MemoTheme.accent : MemoTheme.textFaint,
                                    ),
                                    const SizedBox(width: 12),
                                    Expanded(
                                      child: Column(
                                        crossAxisAlignment: CrossAxisAlignment.start,
                                        children: [
                                          Text(
                                            s.title,
                                            maxLines: 1,
                                            overflow: TextOverflow.ellipsis,
                                            style: MemoTheme.body(14,
                                                w: active ? FontWeight.w600 : FontWeight.w400,
                                                color: active ? MemoTheme.text : MemoTheme.textDim),
                                          ),
                                          const SizedBox(height: 2),
                                          Text(L10n.t('message_count', {'count': '${s.msgCount}'}),
                                              style: MemoTheme.mono(10.5, color: MemoTheme.textFaint)),
                                        ],
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            ),
                          ),
                        );
                      },
                    ),
            ),

            const Divider(height: 1),
            // ── Footer actions ──
            _footerTile(context, Icons.tune_rounded, L10n.t('settings'), MemoTheme.textDim, () {
              Navigator.pop(context);
              Navigator.of(context).push(MaterialPageRoute(builder: (_) => const SettingsScreen()));
            }),
            _footerTile(context, Icons.link_off_rounded, L10n.t('disconnect'), MemoTheme.error, () {
              ref.read(connectionStateProvider.notifier).disconnect();
              Navigator.pop(context);
            }),
            const SizedBox(height: 8),
          ],
        ),
      ),
    );
  }

  Widget _footerTile(BuildContext context, IconData icon, String label, Color color, VoidCallback onTap) {
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 14),
        child: Row(
          children: [
            Icon(icon, size: 19, color: color),
            const SizedBox(width: 14),
            Text(label, style: MemoTheme.body(14.5, color: color)),
          ],
        ),
      ),
    );
  }
}
