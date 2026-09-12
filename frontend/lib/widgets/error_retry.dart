import 'package:flutter/material.dart';

import '../core/friendly_error.dart';
import '../core/l10n.dart';
import '../core/theme.dart';

/// A short, friendly error line with a retry action — the shape
/// agent_screen.dart already used for messagesProvider's error state, factored
/// out so the many settings-tab `.when(error: ...)` branches that used to
/// render `FriendlyError.describeGeneric(e)` as static, unrecoverable text
/// can offer the same way out (M6, stability audit): reload the one provider
/// that failed instead of leaving the user stuck until they close and reopen
/// the whole tab/dialog.
class ErrorRetryLine extends StatelessWidget {
  const ErrorRetryLine({
    super.key,
    required this.error,
    required this.onRetry,
    this.textAlign,
  });

  final Object error;
  final VoidCallback onRetry;
  final TextAlign? textAlign;

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Wrap(
      crossAxisAlignment: WrapCrossAlignment.center,
      spacing: 8,
      runSpacing: 4,
      children: [
        Text(
          FriendlyError.describeGeneric(error),
          textAlign: textAlign,
          style: TextStyle(color: c.textDim, fontSize: 13),
        ),
        TextButton(
          onPressed: onRetry,
          style: TextButton.styleFrom(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
            minimumSize: Size.zero,
            tapTargetSize: MaterialTapTargetSize.shrinkWrap,
          ),
          child: Text(L10n.t('retry')),
        ),
      ],
    );
  }
}
