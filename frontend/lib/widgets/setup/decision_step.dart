import 'package:flutter/material.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../settings/decision_model_section.dart';
import 'setup_widgets.dart';

/// Optional step: the small, fast model that makes yes/no and scoring decisions
/// (mail risk review, memory and agent gating). It reuses the Settings section,
/// so what is chosen here is what Settings shows.
class SetupDecisionStep extends StatelessWidget {
  final ThemeColors color;
  final bool mailConnected;

  const SetupDecisionStep({
    super.key,
    required this.color,
    required this.mailConnected,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SetupInfoTip(
          color: color,
          message: L10n.t(
            mailConnected
                ? 'setup_decision_tip_mail'
                : 'setup_decision_tip_plain',
          ),
        ),
        const SizedBox(height: 12),
        SetupCard(color: color, child: const DecisionModelSection()),
      ],
    );
  }
}
