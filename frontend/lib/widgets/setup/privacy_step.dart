import 'package:flutter/material.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../settings/tabs/backup_restore_tab.dart';
import 'setup_widgets.dart';
import 'telemetry_toggle.dart';

/// Optional step: what Memo may share and be allowed to do. Usage sharing,
/// the OS notification permission (only where the platform has one) and the
/// encrypted cloud backup, which is the Settings tab itself in a fixed-height
/// folded panel.
class SetupPrivacyStep extends StatelessWidget {
  final ThemeColors color;

  /// Null when this platform has no notification permission to ask for.
  final bool? notificationsGranted;
  final bool notificationsAsking;
  final VoidCallback onAskNotifications;

  const SetupPrivacyStep({
    super.key,
    required this.color,
    required this.notificationsGranted,
    required this.notificationsAsking,
    required this.onAskNotifications,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SetupTelemetryToggle(),
        if (notificationsGranted != null) ...[
          const SizedBox(height: 10),
          SetupPreferenceToggle(
            icon: Icons.notifications_active_outlined,
            title: L10n.t('notif_permission_title'),
            desc: L10n.t('notif_permission_desc'),
            value: notificationsGranted!,
            color: color,
            onChanged: notificationsAsking ? null : (_) => onAskNotifications(),
          ),
        ],
        const SizedBox(height: 12),
        SetupCard(
          color: color,
          child: Material(
            type: MaterialType.transparency,
            child: Theme(
              data: ThemeData(dividerColor: Colors.transparent),
              child: ExpansionTile(
                key: const Key('setup_backup_panel'),
                tilePadding: EdgeInsets.zero,
                childrenPadding: EdgeInsets.zero,
                leading: Icon(Icons.cloud_outlined, color: color.textDim),
                title: Text(
                  L10n.t('setup_backup_title'),
                  style: TextStyle(
                    color: color.textMain,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                subtitle: Text(
                  L10n.t('setup_backup_desc'),
                  style: TextStyle(color: color.textDim, fontSize: 12),
                ),
                children: const [
                  SizedBox(height: 560, child: BackupRestoreTab()),
                ],
              ),
            ),
          ),
        ),
      ],
    );
  }
}
