import 'package:flutter/material.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/persona_presets.dart';

/// A card-grid picker for the six built-in [PersonaPreset]s plus a "write
/// your own" option — shared between the Setup Wizard and Settings → System
/// Prompt, so both surfaces offer the same friendly starting point instead
/// of the wizard being the only place a non-technical user can discover
/// these presets. Deliberately "dumb" (StatelessWidget driven entirely by
/// the parent's state): the wizard and the settings tab want different
/// things to happen when a persona is picked (wizard: hold it until the
/// final "Start" save; settings tab: populate the existing raw-text editor
/// for further tweaking), so the picker itself has no opinion about that —
/// it just reports selection changes upward.
class PersonaPicker extends StatelessWidget {
  /// A [PersonaPreset.key], or 'custom' for the free-text option.
  final String selectedKey;
  final TextEditingController nameController;
  final ValueChanged<String> onSelect;
  final ValueChanged<String> onNameChanged;

  /// Whether to offer a "write your own" card plus its own text box and the
  /// preset preview box below the name field. The Setup Wizard needs this
  /// (it has no other place to author or preview a prompt); Settings →
  /// System Prompt passes false, since that screen already has its own
  /// full-size raw-text editor right below this picker — a second "custom"
  /// text box there would just duplicate it, and picking a preset already
  /// populates that editor directly for a live preview.
  final bool includeCustomOption;
  final TextEditingController? customController;
  final ValueChanged<String>? onCustomTextChanged;

  const PersonaPicker({
    super.key,
    required this.selectedKey,
    required this.nameController,
    required this.onSelect,
    required this.onNameChanged,
    this.includeCustomOption = true,
    this.customController,
    this.onCustomTextChanged,
  }) : assert(
         !includeCustomOption || customController != null,
         'customController is required when includeCustomOption is true',
       );

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final isCustom = includeCustomOption && selectedKey == 'custom';
    PersonaPreset? selectedPreset;
    for (final p in personaPresets) {
      if (p.key == selectedKey) {
        selectedPreset = p;
        break;
      }
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            for (final p in personaPresets)
              _PersonaCard(
                icon: p.icon,
                label: p.label,
                desc: p.desc,
                selected: selectedKey == p.key,
                color: c,
                onTap: () => onSelect(p.key),
              ),
            if (includeCustomOption)
              _PersonaCard(
                icon: Icons.edit_note_rounded,
                label: L10n.t('persona_picker_custom_label'),
                desc: L10n.t('setup_persona_custom_desc'),
                selected: isCustom,
                color: c,
                onTap: () => onSelect('custom'),
              ),
          ],
        ),
        SizedBox(height: 12),
        TextField(
          controller: nameController,
          onChanged: onNameChanged,
          decoration: InputDecoration(
            hintText: L10n.t('persona_picker_name_hint'),
            labelText: L10n.t('persona_picker_name_label'),
            filled: true,
            fillColor: c.bgElement,
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              borderSide: BorderSide.none,
            ),
          ),
          style: TextStyle(fontSize: 14, color: c.textMain),
        ),
        if (includeCustomOption) ...[
          SizedBox(height: 12),
          Container(
            decoration: BoxDecoration(
              color: c.bgApp,
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(color: c.borderSoft),
            ),
            child: isCustom
                ? TextField(
                    controller: customController,
                    onChanged: onCustomTextChanged,
                    maxLines: 6,
                    decoration: InputDecoration(
                      hintText: L10n.t('persona_picker_custom_hint'),
                      border: InputBorder.none,
                      contentPadding: EdgeInsets.all(16),
                    ),
                    style: TextStyle(fontSize: 12, height: 1.5, color: c.textMain),
                  )
                : Container(
                    width: double.infinity,
                    padding: EdgeInsets.all(16),
                    child: Text(
                      selectedPreset?.prompt ?? '',
                      style: TextStyle(fontSize: 12, height: 1.5, color: c.textDim),
                    ),
                  ),
          ),
        ],
      ],
    );
  }
}

/// Matches the card design already shipped in the Setup Wizard
/// (_PersonaCard, setup_wizard_view.dart) exactly, so a persona looks the
/// same whether picked there or from Settings → System Prompt.
class _PersonaCard extends StatelessWidget {
  final IconData icon;
  final String label;
  final String desc;
  final bool selected;
  final ThemeColors color;
  final VoidCallback onTap;

  const _PersonaCard({
    required this.icon,
    required this.label,
    required this.desc,
    required this.selected,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 150),
        width: 156,
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: selected ? MemoTheme.accent.withValues(alpha: 0.10) : color.bgElement,
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          border: Border.all(
            color: selected ? MemoTheme.accent : color.borderSoft,
            width: selected ? 1.5 : 1,
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 28,
              height: 28,
              decoration: BoxDecoration(
                color: (selected ? MemoTheme.accent : color.textDim).withValues(alpha: 0.14),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Icon(icon, size: 15, color: selected ? MemoTheme.accent : color.textSecondary),
            ),
            const SizedBox(height: 8),
            Text(
              label,
              style: TextStyle(fontSize: 12.5, fontWeight: FontWeight.w600, color: color.textMain),
            ),
            const SizedBox(height: 2),
            Text(
              desc,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(fontSize: 10.5, height: 1.3, color: color.textDim),
            ),
          ],
        ),
      ),
    );
  }
}
