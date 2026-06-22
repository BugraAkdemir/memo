import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/settings_provider.dart';

class AboutTab extends ConsumerWidget {
  AboutTab();

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
          'Memo, tamamen yerel bilgisayarınızda çalışan, gizlilik odaklı bir yapay zeka asistanıdır. '
          'Konuşmalarınızı ve tercihlerinizi zamanla öğrenip kalıcı hafızasına kazır. '
          'Üçüncü taraf sunuculara ihtiyaç duymadan, kendi bilgisayarınızda çalışır — '
          'verileriniz tamamen sizde kalır. İsteğe bağlı olarak harici API sağlayıcıları '
          'veya yerel llama.cpp modelleri ile kullanılabilir. '
          'WhatsApp entegrasyonu, RAG hafıza ve E2E şifreli bulut senkronizasyonu destekler.',
          style: TextStyle(height: 1.6, color: MemoTheme.of(context).textMuted),
        ),
        SizedBox(height: 24),
        Text(
          'Lisans',
          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
        ),
        SizedBox(height: 8),
        Text(
          'Bu yazılım GNU Affero Genel Kamu Lisansı v3 (AGPL-3.0) ile lisanslanmıştır. '
          'Geliştirici: Buğra Akdemir. Kaynak kod: github.com/BugraAkdemir/memo',
          style: TextStyle(height: 1.6, color: MemoTheme.of(context).textMuted),
        ),
        SizedBox(height: 24),
        Text(
          'Teknolojiler',
          style: TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
        ),
        SizedBox(height: 8),
        Text(
          'Go 1.25 + Flutter 3.10 | SQLite + sqlite-vec (vektör arama) | '
          'whatsmeow (WhatsApp Web) | llama.cpp | Riverpod | Dio',
          style: TextStyle(height: 1.6, color: MemoTheme.of(context).textMuted),
        ),
      ],
    );
  }
}
