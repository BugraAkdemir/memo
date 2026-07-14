import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/settings_provider.dart';

class AboutTab extends ConsumerWidget {
  const AboutTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final versionAsync = ref.watch(appVersionProvider);

    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Row(
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(16),
              child: Image.asset(
                'lib/icon/memo.png',
                width: 64,
                height: 64,
                fit: BoxFit.cover,
              ),
            ),
            SizedBox(width: 24),
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  L10n.t('app_title'),
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                    color: MemoTheme.of(context).textMain,
                  ),
                ),
                versionAsync.when(
                  loading: () => Text('...'),
                  error: (_, _) => SizedBox(),
                  data: (v) => Text(
                    v,
                    style: TextStyle(color: MemoTheme.of(context).textDim),
                  ),
                ),
              ],
            ),
          ],
        ),
        SizedBox(height: 32),
        Text(
          L10n.t('about_vision'),
          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
        ),
        SizedBox(height: 8),
        Text(
          L10n.t('about_vision_body'),
          style: TextStyle(height: 1.6, color: MemoTheme.of(context).textMuted),
        ),
        SizedBox(height: 24),
        Text(
          L10n.t('about_license_title'),
          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
        ),
        SizedBox(height: 8),
        Text(
          L10n.t('about_license_body'),
          style: TextStyle(height: 1.6, color: MemoTheme.of(context).textMuted),
        ),
        SizedBox(height: 24),
        Text(
          L10n.t('about_tech_title'),
          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
        ),
        SizedBox(height: 8),
        Text(
          L10n.t('about_tech_body'),
          style: TextStyle(height: 1.6, color: MemoTheme.of(context).textMuted),
        ),
      ],
    );
  }
}
