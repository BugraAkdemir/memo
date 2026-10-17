import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/app_update.dart';
import '../providers/update_provider.dart';
import 'update_dialog.dart';

/// Opens the update window when Memo starts (and again if an even newer version
/// appears later). Renders nothing itself; mount it once in the app shell.
///
/// One prompt per version per session: closing it with "Later" does not bring
/// it back until the next launch, and "Skip this version" is remembered across
/// launches — except for a required update, which is offered every time.
class UpdatePrompt extends ConsumerStatefulWidget {
  const UpdatePrompt({super.key});

  @override
  ConsumerState<UpdatePrompt> createState() => _UpdatePromptState();
}

class _UpdatePromptState extends ConsumerState<UpdatePrompt> {
  String _promptedApp = '';
  String _promptedComponents = '';
  bool _showing = false;

  @override
  void initState() {
    super.initState();
    // The first answer may already be there when this widget mounts.
    WidgetsBinding.instance.addPostFrameCallback((_) => _consider(ref.read(updateInfoProvider).valueOrNull));
  }

  void _consider(AppUpdateInfo? info) {
    if (!mounted || _showing || info == null) return;
    final skipped = ref.read(skippedUpdateProvider);

    if (shouldPromptApp(info, skipped) && _promptedApp != info.latest) {
      _promptedApp = info.latest;
      _show(AppUpdateDialog(info: info));
      return;
    }
    final pending = info.pendingComponents;
    if (!info.available && pending.isNotEmpty) {
      final key = pending.map((c) => '${c.name}@${c.latest}').join(',');
      if (_promptedComponents != key) {
        _promptedComponents = key;
        _show(ComponentUpdateDialog(components: pending));
      }
    }
  }

  void _show(Widget dialog) {
    _showing = true;
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      if (!mounted) {
        _showing = false;
        return;
      }
      await showDialog<void>(
        context: context,
        barrierDismissible: false,
        builder: (_) => dialog,
      );
      _showing = false;
      ref.read(updateFlowProvider.notifier).reset();
    });
  }

  @override
  Widget build(BuildContext context) {
    ref.listen<AsyncValue<AppUpdateInfo?>>(updateInfoProvider, (_, next) => _consider(next.valueOrNull));
    return const SizedBox.shrink();
  }
}

/// Opens the application-update window on demand (the banner and Settings use
/// it), for an update the person is already looking at.
Future<void> showAppUpdateDialog(BuildContext context, AppUpdateInfo info) {
  return showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (_) => AppUpdateDialog(info: info),
  );
}
