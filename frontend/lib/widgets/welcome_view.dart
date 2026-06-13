import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';

/// Welcome view shown when there are no messages in the active chat.
/// Suggestions are real starters — tapping one fills the composer.
class WelcomeView extends ConsumerWidget {
  const WelcomeView({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(40),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // ─── Logo ────────────────────────────────
            _FadeIn(
              child: ClipRRect(
                borderRadius: BorderRadius.circular(22),
                child: Image.asset(
                  'lib/icon/memo.png',
                  width: 76,
                  height: 76,
                  fit: BoxFit.cover,
                ),
              ),
            ),

            const SizedBox(height: 28),

            _FadeIn(
              delayMs: 100,
              child: Text(
                L10n.t('welcome_title'),
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.w600,
                      color: c.textMain,
                    ),
              ),
            ),

            const SizedBox(height: 8),

            _FadeIn(
              delayMs: 180,
              child: Text(
                L10n.t('welcome_subtitle'),
                style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                      color: c.textDim,
                    ),
              ),
            ),

            const SizedBox(height: 40),

            // ─── Suggestion starters ─────────────────
            _FadeIn(
              delayMs: 260,
              child: Wrap(
                spacing: 12,
                runSpacing: 12,
                alignment: WrapAlignment.center,
                children: [
                  _Suggestion(
                    icon: Icons.code_rounded,
                    label: 'Kod incele',
                    hint: 'Kodunu yapıştır',
                    starter: 'Şu kodu inceler misin:\n',
                  ),
                  _Suggestion(
                    icon: Icons.menu_book_outlined,
                    label: 'Kavram açıkla',
                    hint: 'Bir konu sor',
                    starter: 'Şunu basitçe açıklar mısın: ',
                  ),
                  _Suggestion(
                    icon: Icons.checklist_rounded,
                    label: 'Plan oluştur',
                    hint: 'Bir görev tanımla',
                    starter: 'Şunun için adım adım bir plan oluştur: ',
                  ),
                  _Suggestion(
                    icon: Icons.lightbulb_outline_rounded,
                    label: 'Fikir üret',
                    hint: 'Beyin fırtınası',
                    starter: 'Şu konuda bana fikir üret: ',
                  ),
                ],
              ),
            ),

            const SizedBox(height: 32),

            // ─── Tip ─────────────────────────────────
            _FadeIn(
              delayMs: 340,
              child: Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                decoration: BoxDecoration(
                  color: c.bgPanel,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  border: Border.all(color: c.borderSoft),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.lightbulb_outline_rounded,
                        size: 15, color: c.textDim),
                    const SizedBox(width: 8),
                    Text(
                      'İpucu: "/" yazarak hızlı şablonlara ulaşabilirsin',
                      style: TextStyle(fontSize: 12, color: c.textDim),
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

/// Subtle one-shot fade+rise entrance.
class _FadeIn extends StatelessWidget {
  final Widget child;
  final int delayMs;
  const _FadeIn({required this.child, this.delayMs = 0});

  @override
  Widget build(BuildContext context) {
    return TweenAnimationBuilder<double>(
      tween: Tween(begin: 0, end: 1),
      duration: Duration(milliseconds: 500 + delayMs),
      curve: Curves.easeOut,
      builder: (context, value, child) => Opacity(
        opacity: value.clamp(0, 1),
        child: Transform.translate(
          offset: Offset(0, 10 * (1 - value)),
          child: child,
        ),
      ),
      child: child,
    );
  }
}

class _Suggestion extends ConsumerStatefulWidget {
  final IconData icon;
  final String label;
  final String hint;
  final String starter;

  const _Suggestion({
    required this.icon,
    required this.label,
    required this.hint,
    required this.starter,
  });

  @override
  ConsumerState<_Suggestion> createState() => _SuggestionState();
}

class _SuggestionState extends ConsumerState<_Suggestion> {
  bool _hovering = false;

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _hovering = true),
      onExit: (_) => setState(() => _hovering = false),
      child: GestureDetector(
        onTap: () =>
            ref.read(composerDraftProvider.notifier).state = widget.starter,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 150),
          width: 150,
          padding: const EdgeInsets.all(14),
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
            children: [
              Icon(widget.icon, size: 22, color: MemoTheme.accent),
              const SizedBox(height: 10),
              Text(
                widget.label,
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: c.textMain,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 2),
              Text(
                widget.hint,
                style: TextStyle(fontSize: 11, color: c.textDim),
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
