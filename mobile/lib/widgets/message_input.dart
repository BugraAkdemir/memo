import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api_client.dart';
import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/connection_provider.dart';

class MessageInput extends ConsumerStatefulWidget {
  final void Function(String text) onSend;
  final VoidCallback onStop;
  final bool streaming;

  const MessageInput({
    super.key,
    required this.onSend,
    required this.onStop,
    required this.streaming,
  });

  @override
  ConsumerState<MessageInput> createState() => _MessageInputState();
}

class _MessageInputState extends ConsumerState<MessageInput> {
  late final TextEditingController _ctrl;
  late final FocusNode _focus;
  bool _hasText = false;

  @override
  void initState() {
    super.initState();
    _ctrl = TextEditingController()..addListener(_onChanged);
    _focus = FocusNode();
  }

  void _onChanged() {
    final has = _ctrl.text.trim().isNotEmpty;
    if (has != _hasText) setState(() => _hasText = has);
  }

  @override
  void dispose() {
    _ctrl.dispose();
    _focus.dispose();
    super.dispose();
  }

  void _send() {
    final text = _ctrl.text.trim();
    if (text.isEmpty) return;
    if (text == '/model') {
      _ctrl.clear();
      showModelSwitcher(context, ref);
      return;
    }
    HapticFeedback.lightImpact();
    widget.onSend(text);
    _ctrl.clear();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: const BoxDecoration(
        color: MemoTheme.bg,
        border: Border(top: BorderSide(color: MemoTheme.border)),
      ),
      child: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(12, 10, 12, 10),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              _circleButton(
                icon: Icons.tune_rounded,
                filled: false,
                onTap: () {
                  HapticFeedback.selectionClick();
                  showModelSwitcher(context, ref);
                },
                tooltip: L10n.t('switch_model'),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Container(
                  constraints: const BoxConstraints(minHeight: 46),
                  decoration: BoxDecoration(
                    color: MemoTheme.surfaceAlt,
                    borderRadius: BorderRadius.circular(23),
                    border: Border.all(color: MemoTheme.border),
                  ),
                  alignment: Alignment.center,
                  child: TextField(
                    controller: _ctrl,
                    focusNode: _focus,
                    maxLines: 5,
                    minLines: 1,
                    textInputAction: TextInputAction.send,
                    onSubmitted: (_) => _send(),
                    style: MemoTheme.body(15, color: MemoTheme.text, height: 1.35),
                    cursorColor: MemoTheme.accent,
                    decoration: InputDecoration(
                      isDense: true,
                      filled: false,
                      hintText: L10n.t('message_hint'),
                      hintStyle: MemoTheme.body(15, color: MemoTheme.textFaint),
                      border: InputBorder.none,
                      enabledBorder: InputBorder.none,
                      focusedBorder: InputBorder.none,
                      contentPadding: const EdgeInsets.symmetric(horizontal: 18, vertical: 12),
                    ),
                  ),
                ),
              ),
              const SizedBox(width: 8),
              widget.streaming
                  ? _circleButton(icon: Icons.stop_rounded, filled: true, onTap: widget.onStop, tooltip: L10n.t('stop'))
                  : _circleButton(
                      icon: Icons.arrow_upward_rounded,
                      filled: true,
                      enabled: _hasText,
                      onTap: _send,
                      tooltip: L10n.t('send'),
                    ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _circleButton({
    required IconData icon,
    required bool filled,
    required VoidCallback onTap,
    bool enabled = true,
    String? tooltip,
  }) {
    final active = enabled;
    final btn = AnimatedContainer(
      duration: const Duration(milliseconds: 180),
      width: 46,
      height: 46,
      decoration: BoxDecoration(
        color: filled
            ? (active ? MemoTheme.accent : MemoTheme.surfaceHi)
            : MemoTheme.surfaceAlt,
        shape: BoxShape.circle,
        border: filled ? null : Border.all(color: MemoTheme.border),
      ),
      child: Icon(
        icon,
        size: 21,
        color: filled
            ? (active ? MemoTheme.onAccent : MemoTheme.textFaint)
            : MemoTheme.textDim,
      ),
    );
    return Tooltip(
      message: tooltip ?? '',
      child: Material(
        color: Colors.transparent,
        shape: const CircleBorder(),
        clipBehavior: Clip.antiAlias,
        child: InkWell(onTap: active ? onTap : null, child: btn),
      ),
    );
  }
}

/// Model / provider switcher — a bottom sheet, so the choice is discoverable
/// rather than hidden behind a typed "/model" command.
Future<void> showModelSwitcher(BuildContext context, WidgetRef ref) async {
  final api = ref.read(apiClientProvider);
  String active;
  List<ProviderConfig> providers;
  try {
    active = await api.getActiveProvider();
    providers = await api.getProviders();
  } catch (_) {
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('models_unreachable'))),
      );
    }
    return;
  }
  if (!context.mounted) return;

  final result = await showModalBottomSheet<String?>(
    context: context,
    isScrollControlled: true,
    builder: (ctx) => _ModelSheet(active: active, providers: providers),
  );

  if (result != null && context.mounted) {
    try {
      await api.setActiveProvider(result);
      if (context.mounted) {
        final name = result.isEmpty
            ? L10n.t('local_model')
            : providers
                .firstWhere((p) => p.type == result,
                    orElse: () => ProviderConfig(type: result, name: result, model: ''))
                .name;
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(L10n.t('now_using', {'name': name}))));
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(L10n.t('error_with', {'e': '$e'}))));
      }
    }
  }
}

class _ModelSheet extends StatelessWidget {
  final String active;
  final List<ProviderConfig> providers;
  const _ModelSheet({required this.active, required this.providers});

  @override
  Widget build(BuildContext context) {
    final enabled = providers.where((p) => p.enabled).toList();
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Center(
              child: Container(
                width: 38,
                height: 4,
                decoration: BoxDecoration(color: MemoTheme.borderHi, borderRadius: BorderRadius.circular(2)),
              ),
            ),
            const SizedBox(height: 18),
            Padding(
              padding: const EdgeInsets.only(left: 6, bottom: 4),
              child: Text(L10n.t('answer_with'), style: Theme.of(context).textTheme.titleLarge),
            ),
            Padding(
              padding: const EdgeInsets.only(left: 6, bottom: 12),
              child: Text(L10n.t('answer_with_hint'),
                  style: MemoTheme.body(13, color: MemoTheme.textDim)),
            ),
            _option(context,
                icon: Icons.developer_board_rounded,
                label: L10n.t('local_model_title'),
                subtitle: L10n.t('local_model_subtitle'),
                isActive: active.isEmpty,
                value: ''),
            ...enabled.map((p) => _option(context,
                icon: Icons.cloud_outlined,
                label: p.name,
                subtitle: p.model.isEmpty ? p.type : p.model,
                isActive: active == p.type,
                value: p.type)),
          ],
        ),
      ),
    );
  }

  Widget _option(BuildContext context,
      {required IconData icon,
      required String label,
      required String subtitle,
      required bool isActive,
      required String value}) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Material(
        color: isActive ? MemoTheme.accent.withValues(alpha: 0.12) : MemoTheme.surfaceAlt,
        borderRadius: BorderRadius.circular(14),
        child: InkWell(
          onTap: () {
            HapticFeedback.selectionClick();
            Navigator.pop(context, value);
          },
          borderRadius: BorderRadius.circular(14),
          child: Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: isActive ? MemoTheme.accentDeep : MemoTheme.border),
            ),
            child: Row(
              children: [
                Icon(icon, size: 22, color: isActive ? MemoTheme.accentBright : MemoTheme.textDim),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(label, style: MemoTheme.body(15, w: FontWeight.w600, color: MemoTheme.text)),
                      const SizedBox(height: 2),
                      Text(subtitle, style: MemoTheme.mono(11.5, color: MemoTheme.textFaint)),
                    ],
                  ),
                ),
                if (isActive) const Icon(Icons.check_circle_rounded, size: 20, color: MemoTheme.accent),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
