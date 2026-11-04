import 'package:flutter/material.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../settings/tabs/telegram_tab.dart';
import '../settings/tabs/whatsapp_tab.dart';
import 'setup_widgets.dart';

/// Optional step: pair WhatsApp and Telegram. Each channel is the Settings tab
/// itself, folded away until opened and given a fixed height because the tabs
/// scroll on their own (they cannot sit inside the wizard's scrolling page).
class SetupChannelsStep extends StatelessWidget {
  final ThemeColors color;

  const SetupChannelsStep({super.key, required this.color});

  Widget _channel({
    required Key key,
    required IconData icon,
    required String title,
    required Widget tab,
  }) {
    return SetupCard(
      color: color,
      child: Material(
        type: MaterialType.transparency,
        child: Theme(
          data: ThemeData(dividerColor: Colors.transparent),
          child: ExpansionTile(
            key: key,
            tilePadding: EdgeInsets.zero,
            childrenPadding: EdgeInsets.zero,
            leading: Icon(icon, color: color.textDim),
            title: Text(
              title,
              style: TextStyle(
                color: color.textMain,
                fontWeight: FontWeight.w600,
              ),
            ),
            children: [SizedBox(height: 520, child: tab)],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SetupNote(color: color, message: L10n.t('setup_channels_tip')),
        const SizedBox(height: 12),
        _channel(
          key: const Key('setup_channel_whatsapp'),
          icon: Icons.chat_outlined,
          title: L10n.t('setup_channels_whatsapp'),
          tab: const WhatsAppTab(),
        ),
        const SizedBox(height: 12),
        _channel(
          key: const Key('setup_channel_telegram'),
          icon: Icons.send_outlined,
          title: L10n.t('setup_channels_telegram'),
          tab: const TelegramTab(),
        ),
      ],
    );
  }
}
