import 'package:flutter/material.dart';

import '../core/l10n.dart';
import '../core/theme.dart';

/// Welcome view shown when there are no messages in the active chat.
class WelcomeView extends StatelessWidget {
  const WelcomeView({super.key});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(40),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // ─── Logo ────────────────────────────────
            TweenAnimationBuilder<double>(
              tween: Tween(begin: 0, end: 1),
              duration: const Duration(milliseconds: 600),
              curve: Curves.easeOut,
              builder: (context, value, child) {
                return Opacity(
                  opacity: value,
                  child: Transform.translate(
                    offset: Offset(0, 20 * (1 - value)),
                    child: child,
                  ),
                );
              },
              child: Container(
                width: 80,
                height: 80,
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                    colors: [MemoTheme.accentPale, MemoTheme.accentMuted],
                  ),
                  borderRadius: BorderRadius.circular(24),
                  border: Border.all(
                    color: MemoTheme.accent.withValues(alpha: 0.4),
                    width: 2,
                  ),
                  boxShadow: [
                    BoxShadow(
                      color: MemoTheme.accent.withValues(alpha: 0.15),
                      blurRadius: 32,
                      offset: const Offset(0, 8),
                    ),
                  ],
                ),
                child: const Center(
                  child: Text(
                    'M',
                    style: TextStyle(
                      fontSize: 36,
                      fontWeight: FontWeight.bold,
                      color: MemoTheme.accent,
                    ),
                  ),
                ),
              ),
            ),

            const SizedBox(height: 28),

            // ─── Title ───────────────────────────────
            TweenAnimationBuilder<double>(
              tween: Tween(begin: 0, end: 1),
              duration: const Duration(milliseconds: 600),
              curve: Curves.easeOut,
              builder: (context, value, child) {
                return Opacity(
                  opacity: value,
                  child: Transform.translate(
                    offset: Offset(0, 12 * (1 - value)),
                    child: child,
                  ),
                );
              },
              child: Text(
                L10n.t('welcome_title'),
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.bold,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
            ),

            const SizedBox(height: 8),

            TweenAnimationBuilder<double>(
              tween: Tween(begin: 0, end: 1),
              duration: const Duration(milliseconds: 700),
              curve: Curves.easeOut,
              builder: (context, value, child) {
                return Opacity(opacity: value, child: child);
              },
              child: Text(
                L10n.t('welcome_subtitle'),
                style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                  color: MemoTheme.of(context).textDim,
                ),
              ),
            ),

            const SizedBox(height: 40),

            // ─── Quick Actions ───────────────────────
            TweenAnimationBuilder<double>(
              tween: Tween(begin: 0, end: 1),
              duration: const Duration(milliseconds: 800),
              curve: Curves.easeOut,
              builder: (context, value, child) {
                return Opacity(
                  opacity: value,
                  child: Transform.translate(
                    offset: Offset(0, 8 * (1 - value)),
                    child: child,
                  ),
                );
              },
              child: Wrap(
                spacing: 10,
                runSpacing: 10,
                alignment: WrapAlignment.center,
                children: const [
                  _QuickAction(
                    icon: '💻',
                    label: 'Kod incele',
                    hint: 'Kodunuzu yapıştırın',
                  ),
                  _QuickAction(
                    icon: '📖',
                    label: 'Kavram açıkla',
                    hint: 'Bir konu sorun',
                  ),
                  _QuickAction(
                    icon: '🗺️',
                    label: 'Plan oluştur',
                    hint: 'Bir görev tanımlayın',
                  ),
                  _QuickAction(
                    icon: '💡',
                    label: 'Fikir üret',
                    hint: 'Beyin fırtınası yapın',
                  ),
                ],
              ),
            ),

            const SizedBox(height: 32),

            // ─── Tip ─────────────────────────────────
            TweenAnimationBuilder<double>(
              tween: Tween(begin: 0, end: 1),
              duration: const Duration(milliseconds: 900),
              curve: Curves.easeOut,
              builder: (context, value, child) {
                return Opacity(opacity: value, child: child);
              },
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 10,
                ),
                decoration: BoxDecoration(
                  color: MemoTheme.of(context).bgPanel,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  border: Border.all(color: MemoTheme.of(context).borderSoft),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text('💡', style: TextStyle(fontSize: 14)),
                    const SizedBox(width: 8),
                    Text(
                      'İpucu: "/" yazarak hızlı şablonlara ulaşabilirsiniz',
                      style: TextStyle(
                        fontSize: 12,
                        color: MemoTheme.of(context).textDim,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _QuickAction extends StatefulWidget {
  final String icon;
  final String label;
  final String hint;

  const _QuickAction({
    required this.icon,
    required this.label,
    required this.hint,
  });

  @override
  State<_QuickAction> createState() => _QuickActionState();
}

class _QuickActionState extends State<_QuickAction> {
  bool _hovering = false;

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      onEnter: (_) => setState(() => _hovering = true),
      onExit: (_) => setState(() => _hovering = false),
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 150),
        width: 140,
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: _hovering
              ? MemoTheme.of(context).bgElement
              : MemoTheme.of(context).bgPanel,
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          border: Border.all(
            color: _hovering
                ? MemoTheme.accent.withValues(alpha: 0.3)
                : MemoTheme.of(context).borderSoft,
          ),
        ),
        child: Column(
          children: [
            Text(widget.icon, style: const TextStyle(fontSize: 24)),
            const SizedBox(height: 8),
            Text(
              widget.label,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w500,
                color: MemoTheme.of(context).textMain,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 2),
            Text(
              widget.hint,
              style: TextStyle(
                fontSize: 11,
                color: MemoTheme.of(context).textDim,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}
