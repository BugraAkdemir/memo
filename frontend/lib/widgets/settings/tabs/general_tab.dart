import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/settings_provider.dart';

class GeneralTab extends ConsumerWidget {
  GeneralTab();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final locale = ref.watch(localeProvider);
    final memoryEnabledAsync = ref.watch(memoryEnabledProvider);

    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('general'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 32),

        // Language Selection
        Text(
          L10n.t('language'),
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Container(
          padding: EdgeInsets.symmetric(horizontal: 16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<MemoLocale>(
              value: locale,
              isExpanded: true,
              dropdownColor: MemoTheme.of(context).bgPanel,
              icon: Icon(
                Icons.arrow_drop_down,
                color: MemoTheme.of(context).textDim,
              ),
              items: const [
                DropdownMenuItem(value: MemoLocale.tr, child: Text('Türkçe')),
                DropdownMenuItem(value: MemoLocale.en, child: Text('English')),
              ],
              onChanged: (val) {
                if (val != null) {
                  ref.read(localeProvider.notifier).setLocale(val);
                }
              },
            ),
          ),
        ),

        SizedBox(height: 32),

        // Theme Selection
        Text(
          'Tema',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Container(
          padding: EdgeInsets.symmetric(horizontal: 16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<String>(
              value: ref.watch(themeModeProvider),
              isExpanded: true,
              dropdownColor: MemoTheme.of(context).bgPanel,
              icon: Icon(
                Icons.arrow_drop_down,
                color: MemoTheme.of(context).textDim,
              ),
              items: const [
                DropdownMenuItem(
                  value: 'system',
                  child: Text('Sistem Varsayılanı'),
                ),
                DropdownMenuItem(value: 'light', child: Text('Açık')),
                DropdownMenuItem(value: 'dark', child: Text('Koyu')),
              ],
              onChanged: (val) {
                if (val != null) {
                  ref.read(themeModeProvider.notifier).setMode(val);
                }
              },
            ),
          ),
        ),

        SizedBox(height: 32),

        // Streaming Toggle
        Text(
          'Anlık Gösterim',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Container(
          padding: EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      ref.watch(streamingEnabledProvider) ? 'Açık' : 'Kapalı',
                      style: TextStyle(
                        fontSize: 14,
                        color: MemoTheme.of(context).textMain,
                      ),
                    ),
                    SizedBox(height: 4),
                    Text(
                      'Kapalıyken yanıt tamamlandığında tek seferde gösterilir.',
                      style: TextStyle(
                        fontSize: 12,
                        color: MemoTheme.of(context).textDim,
                      ),
                    ),
                  ],
                ),
              ),
              Switch(
                value: ref.watch(streamingEnabledProvider),
                activeColor: MemoTheme.accent,
                onChanged: (_) {
                  ref.read(streamingEnabledProvider.notifier).toggle();
                },
              ),
            ],
          ),
        ),

        SizedBox(height: 32),

        // Memory Toggle
        Text(
          L10n.t('memory_section'),
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Container(
          padding: EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: memoryEnabledAsync.when(
            loading: () => SizedBox(
              height: 24,
              child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
            ),
            error: (e, _) => Text('${L10n.t('error')}: $e'),
            data: (enabled) => Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        enabled ? L10n.t('memory_active') : L10n.t('memory_disabled'),
                        style: TextStyle(
                          fontSize: 14,
                          color: MemoTheme.of(context).textMain,
                        ),
                      ),
                      SizedBox(height: 4),
                      Text(
                        L10n.t('memory_toggle_desc'),
                        style: TextStyle(
                          fontSize: 12,
                          color: MemoTheme.of(context).textDim,
                        ),
                      ),
                    ],
                  ),
                ),
                Switch(
                  value: enabled,
                  activeColor: MemoTheme.accent,
                  onChanged: (_) async {
                    try {
                      await ref.read(memoryEnabledProvider.notifier).toggle();
                    } catch (e) {
                      if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('${L10n.t('error')}: $e')),
                        );
                      }
                    }
                  },
                ),
              ],
            ),
          ),
        ),

        SizedBox(height: 32),


        // Reset Setup Wizard
        Text(
          'Kurulum',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Container(
          padding: EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Kurulumu Sıfırla',
                style: TextStyle(
                  fontSize: 14,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              OutlinedButton(
                onPressed: () {
                  ref.read(setupCompleteProvider.notifier).resetSetup();
                  Navigator.of(context).pop();
                },
                child: Text(L10n.t('reset')),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                L10n.t('settings_reset_tour'),
                style: TextStyle(
                  fontSize: 14,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              OutlinedButton(
                onPressed: () {
                  ref.read(tourSeenProvider.notifier).resetTour();
                  Navigator.of(context).pop();
                },
                child: Text(L10n.t('reset')),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                L10n.t('settings_reset_launchpad'),
                style: TextStyle(
                  fontSize: 14,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              OutlinedButton(
                onPressed: () {
                  ref.read(launchpadSeenProvider.notifier).reset();
                  Navigator.of(context).pop();
                },
                child: Text(L10n.t('reset')),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
