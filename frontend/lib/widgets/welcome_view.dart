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
      child: SingleChildScrollView(
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
                    label: L10n.t('suggest_review_label'),
                    hint: L10n.t('suggest_review_hint'),
                    starter: L10n.t('quick_review_starter'),
                  ),
                  _Suggestion(
                    icon: Icons.menu_book_outlined,
                    label: L10n.t('suggest_explain_label'),
                    hint: L10n.t('suggest_explain_hint'),
                    starter: L10n.t('quick_explain_starter'),
                  ),
                  _Suggestion(
                    icon: Icons.checklist_rounded,
                    label: L10n.t('suggest_plan_label'),
                    hint: L10n.t('suggest_plan_hint'),
                    starter: L10n.t('quick_plan_starter'),
                  ),
                  _Suggestion(
                    icon: Icons.lightbulb_outline_rounded,
                    label: L10n.t('suggest_ideate_label'),
                    hint: L10n.t('suggest_ideate_hint'),
                    starter: L10n.t('quick_ideate_starter'),
                  ),
                ],
              ),
            ),

            const SizedBox(height: 32),

            // ─── Tip ─────────────────────────────────
            _FadeIn(
              delayMs: 340,
              child: Container(
                // Explicit full-width rather than mainAxisSize.min on the Row
                // below: min-sizing a Row that also contains a Flexible child
                // is a contradiction Flutter doesn't resolve the way you'd
                // expect — confirmed live, the Flexible alone did not stop
                // this from overflowing. Filling the available width outright
                // removes the ambiguity, and this container already read as
                // full-width visually (it has its own background/border).
                width: double.infinity,
                padding:
                    const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                decoration: BoxDecoration(
                  color: c.bgPanel,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  border: Border.all(color: c.borderSoft),
                ),
                child: Row(
                  children: [
                    Icon(Icons.lightbulb_outline_rounded,
                        size: 15, color: c.textDim),
                    const SizedBox(width: 8),
                    // At a phone width the SingleChildScrollView above only
                    // leaves this row ~177px, and the localized tip text
                    // alone needs more — a 23px RenderFlex overflow,
                    // confirmed live. Wrapping to a second line reads fine;
                    // it's a tip, not a label that needs to stay on one line.
                    Expanded(
                      child: Text(
                        L10n.t('tip_slash'),
                        style: TextStyle(fontSize: 12, color: c.textDim),
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
