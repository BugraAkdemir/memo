import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../providers/telemetry_provider.dart';
import '../telemetry_details.dart';
import 'setup_widgets.dart';

/// One background preference the user can decide on right away instead of
/// discovering it later — title, plain-language explanation, and a switch
/// that applies immediately (same provider Settings itself uses, so nothing
/// is lost if the wizard is closed before the final "Start" tap).
/// The setup wizard's switch for the anonymous daily summary: on by default in
/// an official build, switched off right here or later in Settings › Privacy.
/// Hidden while the backend cannot be asked (null status) - a switch that cannot
/// save would lie about its state.
class SetupTelemetryToggle extends ConsumerWidget {
  const SetupTelemetryToggle({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final tel = ref.watch(telemetryStatusProvider).valueOrNull;
    if (tel == null) return const SizedBox.shrink();
    final c = MemoTheme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: 10),
        SetupPreferenceToggle(
          icon: Icons.insights_outlined,
          title: L10n.t('setup_pref_telemetry_title'),
          desc: L10n.t('setup_pref_telemetry_desc'),
          value: tel.sharing,
          color: c,
          onChanged: tel.envOff
              ? null
              : (v) async {
                  try {
                    await ref.read(telemetryActionsProvider).answer(v);
                  } catch (e) {
                    debugPrint('setup_wizard: telemetry answer: $e');
                  }
                },
        ),
        TextButton(
          key: const Key('setupTelemetryDetails'),
          onPressed: () => showDialog<void>(
            context: context,
            builder: (dialogContext) => AlertDialog(
              title: Text(L10n.t('setup_pref_telemetry_title')),
              content: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 520),
                child: const SingleChildScrollView(child: TelemetryDetails()),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.of(dialogContext).pop(),
                  child: Text(L10n.t('close')),
                ),
              ],
            ),
          ),
          child: Text(L10n.t('setup_pref_telemetry_details')),
        ),
      ],
    );
  }
}
