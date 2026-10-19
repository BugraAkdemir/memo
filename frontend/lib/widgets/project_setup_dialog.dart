import 'package:flutter/material.dart';

import '../core/l10n.dart';

/// What the person chose when asked to set Memo up in a project folder.
enum ProjectSetupChoice { setUp, notNow, never }

/// Asks whether to set Memo up in [folderName]. Dismissing the dialog counts as
/// "not now" (null is returned), never as consent.
Future<ProjectSetupChoice?> showProjectSetupDialog(BuildContext context, {required String folderName}) {
  return showDialog<ProjectSetupChoice>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: Text(L10n.t('project_setup_title')),
      content: Text(L10n.t('project_setup_body', {'folder': folderName})),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(ctx).pop(ProjectSetupChoice.never),
          child: Text(L10n.t('project_setup_never')),
        ),
        TextButton(
          onPressed: () => Navigator.of(ctx).pop(ProjectSetupChoice.notNow),
          child: Text(L10n.t('project_setup_later')),
        ),
        FilledButton(
          onPressed: () => Navigator.of(ctx).pop(ProjectSetupChoice.setUp),
          child: Text(L10n.t('project_setup_yes')),
        ),
      ],
    ),
  );
}
