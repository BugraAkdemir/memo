import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../core/friendly_error.dart';
import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/model_display.dart';
import '../models/provider_config.dart';
import '../models/provider_models.dart';
import '../providers/chat_provider.dart';
import '../providers/provider_provider.dart';
import 'model_picker.dart';
import 'quota_badge.dart';
import 'server_file_browser_dialog.dart';

/// Both chat model menus use the same saved/account model pool.
List<ModelPickerEntry> chatModelPickerEntries({
  required List<ProviderConfig> providers,
  required List<ProviderModel> subscriptionModels,
  required String activeProvider,
  String cliType = '',
  Color? mutedColor,
}) {
  final cliName = cliType.isEmpty
      ? ''
      : (providers.where((p) => p.type == cliType).firstOrNull?.name ??
            cliType);
  final effective = cliName.isNotEmpty
      ? cliName
      : (activeProvider.isEmpty ? 'local' : activeProvider);
  return [
    ModelPickerEntry.item(
      value: kAutoRouteProviderName,
      title: L10n.t('auto_route'),
      tag: L10n.t('auto_route_tag'),
      leading: Icon(
        Icons.auto_awesome_outlined,
        size: 16,
        color: MemoTheme.accent,
      ),
      active: effective == kAutoRouteProviderName,
    ),
    ModelPickerEntry.item(
      value: 'local',
      title: L10n.t('local_model'),
      leading: Icon(Icons.computer_outlined, size: 16, color: mutedColor),
      active: effective == 'local',
    ),
    for (final p in providers.where(
      (p) =>
          p.enabled &&
          (ProviderDefaults.isCLIType(p.type) || p.chatModels.isNotEmpty),
    ))
      if (p.name == kSubscriptionsProviderName && subscriptionModels.isNotEmpty)
        ...subscriptionModelPickerEntries(
          p,
          subscriptionModels,
          effective == p.name,
        )
      else if (!ProviderDefaults.isCLIType(p.type))
        ...providerModelPickerEntries(p, isActive: effective == p.name)
      else
        ModelPickerEntry.item(
          value: p.name,
          title: p.name,
          leading: providerLogoWidget(p.type, size: 16),
          active: effective == p.name,
        ),
  ];
}

/// Explicit model choices apply to this chat too, even if it was using a CLI.
Future<String> selectNonCLIChatModel(
  MemoApiClient api,
  String selection, {
  String? chatId,
}) async {
  final model = decodeModelSelection(selection);
  final provider = model?.provider ?? (selection == 'local' ? '' : selection);
  if (model != null) {
    await api.setProviderModel(model.provider, model.model, activate: true);
  } else {
    await api.setActiveProvider(provider);
  }
  // Only leave CLI mode once the chosen model/provider was successfully saved.
  if (chatId != null && chatId.isNotEmpty) {
    await api.setChatCLIProvider(chatId, '');
  }
  return provider;
}

/// Show saved IDs and the legacy current model, never the vendor catalog.
/// Subscriptions use their separate live-account model list.
List<ModelPickerEntry> providerModelPickerEntries(
  ProviderConfig provider, {
  bool isActive = false,
}) {
  final ids = <String>{...provider.configuredModels.map((m) => m.id)};
  if (provider.model.isNotEmpty) ids.add(provider.model);
  return [
    ModelPickerEntry.header(
      provider.name,
      leading: providerLogoWidget(provider.type, size: 14),
    ),
    for (final id in ids.where((id) => id.isNotEmpty))
      ModelPickerEntry.item(
        value: encodeModelSelection(provider.name, id),
        title: id,
        leading: providerLogoWidget(provider.type, size: 16),
        active: isActive && id == provider.model,
      ),
  ];
}

List<ModelPickerEntry> subscriptionModelPickerEntries(
  ProviderConfig provider,
  List<ProviderModel> models,
  bool providerIsActive,
) {
  final out = <ModelPickerEntry>[];
  String? lastVendor;
  for (final m in models) {
    if (m.ownedBy != lastVendor) {
      lastVendor = m.ownedBy;
      final label = vendorLabel(m.ownedBy);
      out.add(
        ModelPickerEntry.header(
          label.isEmpty ? provider.name : label,
          leading: vendorLogo(m.ownedBy, size: 14),
        ),
      );
    }
    final d = prettyModelName(m.id);
    out.add(
      ModelPickerEntry.item(
        value: encodeModelSelection(provider.name, m.id),
        title: d.title,
        tag: d.tag,
        leading: modelFamilyLogo(m.id, size: 16),
        trailing: m.remainingPercent == null
            ? null
            : QuotaBadge(model: m, fontSize: 11),
        active: providerIsActive && provider.model == m.id,
      ),
    );
  }
  return out;
}

/// First time a chat is pointed at a CLI provider: warns the user this is
/// a real agent (file/shell access), then asks which folder it should run
/// in. Only runs once per chat — a workdir already set means both of
/// these already happened.
Future<void> setupCLIChat(
  BuildContext context,
  WidgetRef ref,
  String chatId,
) async {
  final api = ref.read(apiClientProvider);
  final existing = await api.getChatCLIWorkdir(chatId);
  if (existing.isNotEmpty || !context.mounted) return;

  final proceed = await showDialog<bool>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: Text(L10n.t('cli_first_use_title')),
      content: Text(L10n.t('cli_first_use_body')),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(dialogContext).pop(false),
          child: Text(L10n.t('cancel')),
        ),
        FilledButton(
          onPressed: () => Navigator.of(dialogContext).pop(true),
          child: Text(L10n.t('cli_first_use_continue')),
        ),
      ],
    ),
  );
  if (proceed != true || !context.mounted) return;

  final dir = await showServerFileBrowserDialog(
    context,
    mode: ServerBrowseMode.directory,
  );
  if (dir == null) return;
  try {
    await api.setChatCLIWorkdir(chatId, dir);
  } catch (e) {
    // Not ScaffoldMessenger.of(context) — the native folder picker above
    // can stay open for a long, unpredictable time, and by the time it
    // resolves the widget this context came from (deep inside the model
    // dropdown's popup menu) may already be gone. errorMessageProvider is
    // a global, context-free sink a stable top-level listener shows as a
    // SnackBar (see _ChatContent's ref.listen) — this codebase's own
    // established way of avoiding exactly this class of crash.
    ref.read(errorMessageProvider.notifier).state = L10n.t('switch_failed', {
      'e': FriendlyError.describeGeneric(e),
    });
  }
}
