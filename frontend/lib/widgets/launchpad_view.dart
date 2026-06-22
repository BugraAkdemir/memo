import 'package:flutter/material.dart';

import '../core/l10n.dart';
import '../core/theme.dart';

class LaunchpadView extends StatelessWidget {
  final VoidCallback onStartChat;
  final VoidCallback onNavigateAgent;
  final VoidCallback onNavigateWhatsApp;
  final VoidCallback onNavigateCalendar;
  final VoidCallback onNavigateModels;
  final bool whatsAppConnected;

  const LaunchpadView({
    super.key,
    required this.onStartChat,
    required this.onNavigateAgent,
    required this.onNavigateWhatsApp,
    required this.onNavigateCalendar,
    required this.onNavigateModels,
    this.whatsAppConnected = false,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);

    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(48),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(22),
              child: Image.asset(
                'lib/icon/memo.png',
                width: 64,
                height: 64,
                fit: BoxFit.cover,
              ),
            ),
            const SizedBox(height: 20),
            Text(
              L10n.t('launchpad_title'),
              style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.w700,
                    color: c.textMain,
                  ),
            ),
            const SizedBox(height: 6),
            Text(
              L10n.t('launchpad_subtitle'),
              style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                    color: c.textDim,
                  ),
            ),
            const SizedBox(height: 36),
            Wrap(
              spacing: 16,
              runSpacing: 16,
              alignment: WrapAlignment.center,
              children: [
                _FeatureCard(
                  icon: Icons.chat_bubble_rounded,
                  title: L10n.t('launchpad_chat_title'),
                  description: L10n.t('launchpad_chat_desc'),
                  actionLabel: L10n.t('launchpad_start_chat'),
                  onTap: onStartChat,
                ),
                _FeatureCard(
                  icon: Icons.smart_toy_rounded,
                  title: L10n.t('launchpad_agent_title'),
                  description: L10n.t('launchpad_agent_desc'),
                  actionLabel: L10n.t('launchpad_start_chat'),
                  onTap: onNavigateAgent,
                ),
                _FeatureCard(
                  icon: Icons.diversity_3_rounded,
                  title: L10n.t('launchpad_orchestra_title'),
                  description: L10n.t('launchpad_orchestra_desc'),
                  actionLabel: L10n.t('launchpad_start_chat'),
                  onTap: onNavigateModels,
                ),
                _FeatureCard(
                  icon: Icons.message_rounded,
                  title: L10n.t('launchpad_whatsapp_title'),
                  description: L10n.t('launchpad_whatsapp_desc'),
                  actionLabel: whatsAppConnected
                      ? L10n.t('launchpad_start_chat')
                      : L10n.t('launchpad_connect_wa'),
                  onTap: onNavigateWhatsApp,
                ),
                _FeatureCard(
                  icon: Icons.calendar_month_rounded,
                  title: L10n.t('launchpad_calendar_title'),
                  description: L10n.t('launchpad_calendar_desc'),
                  actionLabel: L10n.t('launchpad_start_chat'),
                  onTap: onNavigateCalendar,
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _FeatureCard extends StatefulWidget {
  final IconData icon;
  final String title;
  final String description;
  final String actionLabel;
  final VoidCallback onTap;

  const _FeatureCard({
    required this.icon,
    required this.title,
    required this.description,
    required this.actionLabel,
    required this.onTap,
  });

  @override
  State<_FeatureCard> createState() => _FeatureCardState();
}

class _FeatureCardState extends State<_FeatureCard> {
  bool _hovering = false;

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);

    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _hovering = true),
      onExit: (_) => setState(() => _hovering = false),
      child: GestureDetector(
        onTap: widget.onTap,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          width: 200,
          padding: const EdgeInsets.all(20),
          decoration: BoxDecoration(
            color: _hovering ? c.bgElement : c.bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(
              color: _hovering
                  ? MemoTheme.accent.withValues(alpha: 0.5)
                  : c.borderSoft,
            ),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  color: MemoTheme.accent.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Icon(widget.icon, size: 24, color: MemoTheme.accent),
              ),
              const SizedBox(height: 14),
              Text(
                widget.title,
                style: TextStyle(
                  fontSize: 15,
                  fontWeight: FontWeight.w600,
                  color: c.textMain,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 6),
              Text(
                widget.description,
                style: TextStyle(fontSize: 12, color: c.textDim, height: 1.4),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 16),
              Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                decoration: BoxDecoration(
                  color: _hovering
                      ? MemoTheme.accent.withValues(alpha: 0.15)
                      : c.bgHover,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Text(
                  widget.actionLabel,
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                    color: _hovering ? MemoTheme.accent : c.textSecondary,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
