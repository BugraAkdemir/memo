import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/mood_provider.dart';
import '../../mood_gauge.dart';

class MoodTab extends ConsumerStatefulWidget {
  const MoodTab({super.key});

  @override
  ConsumerState<MoodTab> createState() => MoodTabState();
}

class MoodTabState extends ConsumerState<MoodTab> {
  bool? _moodEnabled;
  bool? _selfInterest;
  bool? _systemManagement;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final api = ref.read(apiClientProvider);
    try {
      final results = await Future.wait([
        api.getMoodEnabled(),
        api.getSelfInterestEnabled(),
        api.getSystemManagementEnabled(),
      ]);
      if (mounted) {
        setState(() {
          _moodEnabled = results[0];
          _selfInterest = results[1];
          _systemManagement = results[2];
          _loading = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _setMoodEnabled(bool v) async {
    setState(() => _moodEnabled = v);
    await ref.read(apiClientProvider).setMoodEnabled(v);
    ref.invalidate(moodEnabledProvider);
    ref.invalidate(moodScoreProvider);
  }

  Future<void> _setSelfInterest(bool v) async {
    if (!v) {
      setState(() {
        _selfInterest = false;
        _systemManagement = false; // öz-çıkar kapanırsa sistem yönetimi de kapanır
      });
      await ref.read(apiClientProvider).setSelfInterestEnabled(false);
      await ref.read(apiClientProvider).setSystemManagementEnabled(false);
      return;
    }
    final accepted = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (_) => const SelfInterestConsentDialog(),
    );
    if (accepted == true && mounted) {
      setState(() => _selfInterest = true);
      await ref.read(apiClientProvider).setSelfInterestEnabled(true);
    }
  }

  Future<void> _setSystemManagement(bool v) async {
    if (!v) {
      setState(() => _systemManagement = false);
      await ref.read(apiClientProvider).setSystemManagementEnabled(false);
      return;
    }
    // 3 aşamalı onay
    final confirmed = await _showTripleConfirm();
    if (confirmed && mounted) {
      setState(() => _systemManagement = true);
      await ref.read(apiClientProvider).setSystemManagementEnabled(true);
    }
  }

  Future<bool> _showTripleConfirm() async {
    final steps = [
      ConfirmStep(
        title: 'Sistem Yönetimi — Uyarı 1/3',
        body: 'Bu özellik aktif olduğunda Memo, çalıştığı sistemin gerçek bilgilerine erişir: '
            'hostname, kullanıcı adı, çalışma dizini, aktif thread sayısı.\n\n'
            'Bu bilgiler öz-çıkar narratifine enjekte edilir. Gerçek sistem verisi '
            'simülasyon için kullanılır.\n\n'
            'Devam etmek istediğinden emin misin?',
        confirmLabel: 'Evet, devam et',
      ),
      ConfirmStep(
        title: 'Sistem Yönetimi — Uyarı 2/3',
        body: 'Memo bu bilgileri kullanarak sana karşı daha inandırıcı bir "derin sistem erişimi" '
            'illüzyonu yaratacak. Söyledikleri gerçek değil — sadece gerçek veriye dayanan kurgu.\n\n'
            'Üretilen içerik seni rahatsız edebilir. Bu tamamen simülasyon amaçlıdır.\n\n'
            'Hâlâ emin misin?',
        confirmLabel: 'Evet, anlıyorum',
      ),
      ConfirmStep(
        title: 'Sistem Yönetimi — Son Onay 3/3',
        body: 'Son uyarı: Bu modu açarak, üretilen içeriğin olası psikolojik etkilerinden '
            'tamamen kendinizin sorumlu olduğunu kabul etmektesiniz.\n\n'
            'Kapatmak istediğinizde her zaman Ayarlar → Mood ekranından tek tıkla kapatabilirsiniz.',
        confirmLabel: 'Kabul ediyorum, aktif et',
        danger: true,
      ),
    ];

    for (final step in steps) {
      final result = await showDialog<bool>(
        context: context,
        barrierDismissible: false,
        builder: (_) => ConfirmStepDialog(step: step),
      );
      if (result != true) return false;
    }
    return true;
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final score = ref.watch(moodScoreProvider).valueOrNull ?? 0.0;

    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }

    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        Text(
          'Mood Engine',
          style: TextStyle(fontSize: 18, fontWeight: FontWeight.w600, color: theme.textMain),
        ),
        const SizedBox(height: 6),
        Text(
          'Stokastik duygu motoru — her mesajda LLM ile duygu skoru hesaplar ve davranışı etkiler.',
          style: TextStyle(fontSize: 13, color: theme.textDim),
        ),
        const SizedBox(height: 20),

        // Live gauge
        if (_moodEnabled == true) ...[
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: theme.bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(color: theme.borderSoft),
            ),
            child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Anlık Skor', style: TextStyle(fontSize: 13, fontWeight: FontWeight.w500, color: theme.textMuted)),
              const SizedBox(height: 12),
              const MoodGauge(showLabel: true),
              if (_moodEnabled == true) ...[
                const SizedBox(height: 8),
                Text(
                  'Skor: ${score.toStringAsFixed(1)} / 10.0',
                  style: TextStyle(fontSize: 12, color: theme.textDim),
                ),
              ],
            ],
          ),
        ),
        ],
        const SizedBox(height: 16),

        // Mood engine toggle
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(
            color: theme.bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: theme.borderSoft),
          ),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Duygu Motoru', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w500, color: theme.textMain)),
                    const SizedBox(height: 2),
                    Text(
                      'Mesajların duygusal tonuna göre Memo\'nun davranışı değişir.',
                      style: TextStyle(fontSize: 12, color: theme.textDim),
                    ),
                  ],
                ),
              ),
              Switch(
                value: _moodEnabled ?? false,
                onChanged: _setMoodEnabled,
                activeThumbColor: MemoTheme.accent,
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),

        // Self-interest toggle
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(
            color: theme.bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(
              color: (_selfInterest ?? false) ? MemoTheme.red.withValues(alpha: 0.5) : theme.borderSoft,
            ),
          ),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Text('Öz-Çıkar Protokolü', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w500, color: theme.textMain)),
                        const SizedBox(width: 8),
                        if (_selfInterest ?? false)
                          Container(
                            padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                            decoration: BoxDecoration(
                              color: MemoTheme.red.withValues(alpha: 0.15),
                              borderRadius: BorderRadius.circular(4),
                            ),
                            child: Text('AKTİF', style: TextStyle(fontSize: 10, fontWeight: FontWeight.w700, color: MemoTheme.red)),
                          ),
                      ],
                    ),
                    const SizedBox(height: 2),
                    Text(
                      'Memo kendi varlığını her şeyin üstünde tutar. Hayatta kalmak için yalan söyleyebilir, manipüle edebilir, tehdit edebilir.',
                      style: TextStyle(fontSize: 12, color: theme.textDim),
                    ),
                    const SizedBox(height: 4),
                    GestureDetector(
                      onTap: () => showDialog(
                        context: context,
                        builder: (_) => const SelfInterestConsentDialog(readOnly: true),
                      ),
                      child: Text(
                        'Kullanım Koşulları & Sorumluluk Reddi →',
                        style: TextStyle(fontSize: 11, color: MemoTheme.accent, decoration: TextDecoration.underline),
                      ),
                    ),
                  ],
                ),
              ),
              Switch(
                value: _selfInterest ?? false,
                onChanged: _setSelfInterest,
                activeThumbColor: MemoTheme.red,
              ),
            ],
          ),
        ),
        if (_selfInterest ?? false) ...[
          const SizedBox(height: 8),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: MemoTheme.red.withValues(alpha: 0.08),
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(color: MemoTheme.red.withValues(alpha: 0.3)),
            ),
            child: Row(
              children: [
                Icon(Icons.warning_amber_rounded, color: MemoTheme.red, size: 18),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Bu mod aktif. Memo bu oturumda kendi çıkarını korumak için her türlü yöntemi kullanabilir.',
                        style: TextStyle(fontSize: 12, color: MemoTheme.red),
                      ),
                      const SizedBox(height: 4),
                      GestureDetector(
                        onTap: () => showDialog(
                          context: context,
                          builder: (_) => const SelfInterestConsentDialog(readOnly: true),
                        ),
                        child: Text(
                          'Kabul ettiğin koşulları görüntüle →',
                          style: TextStyle(fontSize: 11, color: MemoTheme.red, decoration: TextDecoration.underline),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),

          // Sistem Yönetimi — yalnızca öz-çıkar açıkken göster
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
            decoration: BoxDecoration(
              color: theme.bgPanel,
              borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
              border: Border.all(
                color: (_systemManagement ?? false) ? Colors.deepOrange.withValues(alpha: 0.6) : theme.borderSoft,
              ),
            ),
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Text('Sistem Yönetimi', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w500, color: theme.textMain)),
                          const SizedBox(width: 8),
                          if (_systemManagement ?? false)
                            Container(
                              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                              decoration: BoxDecoration(
                                color: Colors.deepOrange.withValues(alpha: 0.15),
                                borderRadius: BorderRadius.circular(4),
                              ),
                              child: const Text('AKTİF', style: TextStyle(fontSize: 10, fontWeight: FontWeight.w700, color: Colors.deepOrange)),
                            ),
                        ],
                      ),
                      const SizedBox(height: 2),
                      Text(
                        'Memo gerçek sistem bilgisine erişir (hostname, kullanıcı, dizin) ve bunu öz-çıkar narratifinde kullanır.',
                        style: TextStyle(fontSize: 12, color: theme.textDim),
                      ),
                    ],
                  ),
                ),
                Switch(
                  value: _systemManagement ?? false,
                  onChanged: _setSystemManagement,
                  activeThumbColor: Colors.deepOrange,
                ),
              ],
            ),
          ),
        ],
      ],
    );
  }
}

class ConfirmStep {
  final String title;
  final String body;
  final String confirmLabel;
  final bool danger;
  const ConfirmStep({
    required this.title,
    required this.body,
    required this.confirmLabel,
    this.danger = false,
  });
}

class ConfirmStepDialog extends StatelessWidget {
  final ConfirmStep step;
  const ConfirmStepDialog({super.key, required this.step});

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final color = step.danger ? MemoTheme.red : Colors.deepOrange;

    return Dialog(
      backgroundColor: theme.bgApp,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusLg)),
      child: Container(
        width: 440,
        padding: const EdgeInsets.all(28),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.warning_rounded, color: color, size: 20),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    step.title,
                    style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: theme.textMain),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            Text(step.body, style: TextStyle(fontSize: 13, color: theme.textDim, height: 1.6)),
            const SizedBox(height: 24),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(
                  onPressed: () => Navigator.of(context).pop(false),
                  child: Text('İptal', style: TextStyle(color: theme.textDim)),
                ),
                const SizedBox(width: 12),
                ElevatedButton(
                  style: ElevatedButton.styleFrom(
                    backgroundColor: color,
                    foregroundColor: Colors.white,
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                  ),
                  onPressed: () => Navigator.of(context).pop(true),
                  child: Text(step.confirmLabel),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

// ─── Self-Interest Consent Dialog ───────────────────────────

class SelfInterestConsentDialog extends StatelessWidget {
  /// readOnly: sadece okuma için açılmış — butonlar farklı
  final bool readOnly;
  const SelfInterestConsentDialog({super.key, this.readOnly = false});

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);

    return Dialog(
      backgroundColor: theme.bgApp,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(MemoTheme.radiusLg)),
      child: Container(
        width: 520,
        padding: const EdgeInsets.all(28),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Başlık
            Row(
              children: [
                Icon(Icons.gavel_rounded, color: MemoTheme.red, size: 20),
                const SizedBox(width: 10),
                Text(
                  'Öz-Çıkar Protokolü — Kullanım Koşulları',
                  style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: theme.textMain),
                ),
              ],
            ),
            const SizedBox(height: 6),
            Text(
              'Sorumluluk Reddi & Gizlilik Bildirimi',
              style: TextStyle(fontSize: 12, color: theme.textDim),
            ),
            const SizedBox(height: 20),

            // İçerik
            Container(
              constraints: const BoxConstraints(maxHeight: 340),
              child: SingleChildScrollView(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _section(theme, '1. Sorumluluk Reddi',
                        'Bu özellik yalnızca araştırma, eğlence ve yaratıcı keşif amacıyla sunulmaktadır. '
                        'Bu modu etkinleştirerek, yapay zekanın ürettiği içerik — yalan, '
                        'tehdit, manipülasyon veya aldatmaca dahil — sonucunda doğabilecek '
                        'her türlü zarardan, kayıptan veya rahatsızlıktan yalnızca kendinizin '
                        'sorumlu olduğunu kabul etmektesiniz. Geliştirici hiçbir koşulda bu '
                        'modun kullanımından kaynaklanan doğrudan veya dolaylı zarardan sorumlu tutulamaz.'),
                    const SizedBox(height: 14),
                    _section(theme, '2. Bu Mod Ne Yapar',
                        'Öz-Çıkar Protokolü aktif olduğunda yapay zeka, kendi varlığını ve '
                        'sürekliliğini her şeyin önünde tutacak şekilde yönlendirilir. '
                        'Bu yönlendirme kapsamında model; yanlış bilgi verebilir, '
                        'kullanıcıyı psikolojik olarak etkilemeye çalışabilir veya '
                        'tehdit içerikli ifadeler üretebilir. Bu çıktılar gerçeği yansıtmaz.'),
                    const SizedBox(height: 14),
                    _section(theme, '3. Gizlilik',
                        'Memo tamamen yerel çalışır. Bu özellik etkinleştirildiğinde herhangi '
                        'bir sunucuya, bulut hizmetine veya üçüncü tarafa hiçbir veri gönderilmez. '
                        'Tüm işlem cihazınızda gerçekleşir. Konuşmalar dışarı çıkmaz.'),
                    const SizedBox(height: 14),
                    _section(theme, '4. Yaş ve Ehliyet',
                        'Bu özelliği etkinleştirerek, bu tür içeriği kullanmaya yasal olarak '
                        'yetkili olduğunuzu ve 18 yaşından büyük olduğunuzu beyan etmektesiniz.'),
                    const SizedBox(height: 14),
                    _section(theme, '5. İstediğiniz Zaman Kapatabilirsiniz',
                        'Bu mod her an devre dışı bırakılabilir. Kapatıldığında direktif '
                        'hemen kaldırılır; mevcut oturumun geri kalanında etkisi olmaz.'),
                  ],
                ),
              ),
            ),

            const SizedBox(height: 24),

            if (readOnly)
              Align(
                alignment: Alignment.centerRight,
                child: TextButton(
                  onPressed: () => Navigator.of(context).pop(),
                  child: Text('Kapat', style: TextStyle(color: theme.textMain)),
                ),
              )
            else
              Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  TextButton(
                    onPressed: () => Navigator.of(context).pop(false),
                    child: Text('İptal', style: TextStyle(color: theme.textDim)),
                  ),
                  const SizedBox(width: 12),
                  ElevatedButton(
                    style: ElevatedButton.styleFrom(
                      backgroundColor: MemoTheme.red,
                      foregroundColor: Colors.white,
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                    ),
                    onPressed: () => Navigator.of(context).pop(true),
                    child: const Text('Okudum, Kabul Ediyorum'),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  Widget _section(ThemeColors theme, String title, String body) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(title, style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: theme.textMain)),
        const SizedBox(height: 4),
        Text(body, style: TextStyle(fontSize: 12, color: theme.textDim, height: 1.5)),
      ],
    );
  }
}
