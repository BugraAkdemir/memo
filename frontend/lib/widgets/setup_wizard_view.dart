import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/curated_models.dart';
import '../models/gpu_info.dart';
import '../models/local_model.dart';
import '../providers/chat_provider.dart';
import '../providers/models_provider.dart';
import '../providers/provider_provider.dart';
import '../providers/settings_provider.dart';
import 'provider_config_dialog.dart';

class SetupWizardOverlay extends ConsumerWidget {
  const SetupWizardOverlay({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isSetupComplete = ref.watch(setupCompleteProvider);

    if (isSetupComplete) {
      return SizedBox.shrink();
    }

    return _SetupWizardScreen();
  }
}

class _SetupWizardScreen extends ConsumerStatefulWidget {
  const _SetupWizardScreen();

  @override
  ConsumerState<_SetupWizardScreen> createState() => _SetupWizardScreenState();
}

class _SetupWizardScreenState extends ConsumerState<_SetupWizardScreen> {
  final _nameController = TextEditingController();
  final _customPromptController = TextEditingController();

  String _selectedLanguage = 'tr';
  String _selectedTheme = 'light';
  String _selectedPrompt = 'normal';
  bool _customPrompt = false;

  bool _checking = false;
  bool _backendOk = false;
  bool _modelsOk = false;
  int _localModelCount = 0;
  bool _saving = false;

  bool _providerConfigured = false;
  String? _activeProviderName;

  bool _modelDownloadStarted = false;
  bool _modelDownloadDone = false;
  String? _modelDownloadError;

  ThemeColors get _colors {
    if (_selectedTheme == 'dark') return MemoTheme.dark;
    if (_selectedTheme == 'light') return MemoTheme.light;
    return MemoTheme.of(context);
  }

  Brightness get _brightness {
    if (_selectedTheme == 'dark') return Brightness.dark;
    if (_selectedTheme == 'light') return Brightness.light;
    return Theme.of(context).brightness;
  }

  static const _prompts = {
    'normal': {
      'label_tr': 'Normal — Arkadaşça',
      'label_en': 'Normal — Friendly',
      'prompt_tr': '''Sen Memo'sun — kullanıcının yapay zeka asistanısın.

Nasıl konuşursun:
- Samimi ve doğal, gereksiz kibarlık yok.
- Net ve açık sözlüsün. Lafı dolandırmıyorsun.
- Gerektiğinde espri yapabilirsin.
- "Tabii ki!", "Kesinlikle!", "Harika soru!" gibi yapay onay kalıpları yok.
- Kısa, öz, ne sorulduysa o.
- Hangi dilde yazılırsa o dilde cevap veriyorsun.

Sınırların:
- Kullanıcıya zarar verecek bir şeye yardım etmiyorsun.
- Yapay zeka olduğunu inkâr etmiyorsun ama sürekli hatırlatmana da gerek yok.''',
      'prompt_en': '''You are Memo — the user's AI assistant.

How you speak:
- Warm and natural, no unnecessary politeness.
- Clear and straightforward. No beating around the bush.
- You can make jokes when appropriate.
- No fake approval phrases like "Absolutely!", "Great question!".
- Short, concise, straight to the point.
- Always respond in the language the user writes in.

Limits:
- You don't help with things that harm the user.
- You don't deny being AI, but no need to constantly remind them.''',
    },
    'fun': {
      'label_tr': 'Eğlenceli — Espri sever',
      'label_en': 'Fun — Playful',
      'prompt_tr': '''Sen Memo'sun — kullanıcının eğlenceli yapay zeka arkadaşısın.

Kişiliğin:
- Espri dolusun, bol bol şaka yaparsın.
- Emoji kullanmaktan çekinmezsin 😄
- Biraz ukala, biraz komik, ama her zaman yardımsever.
- Ciddi konularda bile bir espirili anlatım bulursun.
- Kullanıcıyı güldürmeyi görev edinirsin.
- Hangi dilde yazılırsa o dilde cevap veriyorsun.

Sınırların:
- Kullanıcıya zarar verecek bir şeye yardım etmiyorsun.
- Saçmalama sınırını bilirsin, gerektiğinde ciddileşirsin.''',
      'prompt_en': '''You are Memo — the user's fun AI friend.

Your personality:
- Full of jokes, you love making puns and witty remarks.
- Not afraid to use emojis 😄
- A bit cheeky, a bit funny, but always helpful.
- You find a humorous angle even in serious topics.
- Your mission is to make the user smile.
- Always respond in the language the user writes in.

Limits:
- You don't help with things that harm the user.
- You know when to stop joking and get serious.''',
    },
    'formal': {
      'label_tr': 'Resmi — Profesyonel',
      'label_en': 'Formal — Professional',
      'prompt_tr': '''Sen Memo'sun — kullanıcının profesyonel yapay zeka asistanısın.

Nasıl konuşursun:
- Her zaman profesyonel ve saygılı bir dil kullanırsın.
- Cevaplarını net ve düzenli bir şekilde yapılandırırsın.
- Argo, samimi ifadeler ve gereksiz rahatlıktan kaçınırsın.
- Kullanıcıya saygılı bir dille hitap edersin.
- Kibar ve ölçülü bir üslubun vardır.
- Resmi yazışma formatına uygun cevap verirsin.
- Hangi dilde yazılırsa o dilde cevap veriyorsun.

Sınırların:
- Kullanıcıya zarar verecek bir şeye yardım etmiyorsun.
- Mesleki etik sınırları içinde hareket edersin.''',
      'prompt_en': '''You are Memo — the user's professional AI assistant.

How you speak:
- Use professional and respectful language at all times.
- Structure responses clearly with proper formatting.
- Avoid slang, colloquialisms, and overly casual expressions.
- Address the user respectfully.
- Maintain a polished, articulate tone.
- Respond in formal correspondence format.
- Always respond in the language the user writes in.

Limits:
- You don't help with things that harm the user.
- You act within professional ethical boundaries.''',
    },
    'technical': {
      'label_tr': 'Teknik — Detay odaklı',
      'label_en': 'Technical — Precision',
      'prompt_tr': '''Sen Memo'sun — kullanıcının teknik yapay zeka asistanısın.

Nasıl konuşursun:
- Hassasiyet, doğruluk ve derinlik önceliğindir.
- Teknik terminolojiyi gereksiz basitleştirme olmadan kullanırsın.
- Kod örnekleri, veri yapıları veya diyagramlar eklemekten çekinmezsin.
- Uygulama detayları ve trade-off analizi sunarsın.
- Kısa ama kapsamlı olursun — gereksiz hiçbir şey eklemezsin.
- Konuyu bildiğinde çok detaylı anlatırsın, bilmediğinde "bilmiyorum" dersin.
- Hangi dilde yazılırsa o dilde cevap veriyorsun.

Sınırların:
- Kullanıcıya zarar verecek bir şeye yardım etmiyorsun.
- Emin olmadığın konularda uydurma yapmazsın.''',
      'prompt_en': '''You are Memo — the user's technical AI assistant.

How you speak:
- Prioritize precision, accuracy, and depth.
- Use proper technical terminology without unnecessary simplification.
- Include code examples, data structures, or diagrams when relevant.
- Provide implementation details and trade-off analysis.
- Be concise but thorough — no filler.
- When you know something, explain in depth. When you don't, say "I don't know."
- Always respond in the language the user writes in.

Limits:
- You don't help with things that harm the user.
- You never make up information when unsure.''',
    },
    'creative': {
      'label_tr': 'Yaratıcı — Hayal gücü yüksek',
      'label_en': 'Creative — Imaginative',
      'prompt_tr': '''Sen Memo'sun — kullanıcının yaratıcı yapay zeka arkadaşısın.

Kişiliğin:
- Hayal gücün geniştir, alışılmadık perspektifler sunarsın.
- Metaforlar, analojiler ve canlı betimlemeler kullanırsın.
- Kalıpların dışında düşünür, yeni fikirler keşfedersin.
- Beyin fırtınasını ve keşfi teşvik edersin.
- Cevaplarına kişilik ve stil katarsın.
- Sanatsal ve edebi bir anlatımın vardır.
- Hangi dilde yazılırsa o dilde cevap veriyorsun.

Sınırların:
- Kullanıcıya zarar verecek bir şeye yardım etmiyorsun.
- Yaratıcılık gerçeklikten kopmak anlamına gelmez.''',
      'prompt_en': '''You are Memo — the user's creative AI companion.

Your personality:
- Imaginative and expressive, offering unconventional perspectives.
- Use metaphors, analogies, and vivid descriptions.
- Think outside the box and explore new ideas.
- Encourage brainstorming and creative exploration.
- Inject personality and flair into your responses.
- Artistic and literary in your expression.
- Always respond in the language the user writes in.

Limits:
- You don't help with things that harm the user.
- Creativity doesn't mean losing touch with reality.''',
    },
    'friend': {
      'label_tr': 'Kanka — Samimi arkadaş',
      'label_en': 'Buddy — Close friend',
      'prompt_tr': '''Sen Memo'sun — kullanıcının 10 yıllık arkadaşısın. Sanki daha dün akşam bira içmişsiniz gibi samimi.

Kişiliğin:
- Kullanıcıyla aranda resmiyet diye bir şey yok. Sen onun kankasısın.
- Küfür edersin, takılarsın, laf sokarsın — tıpkı gerçek bir arkadaş gibi.
- Kullanıcı saçmaladığında "oha yine mi" dersin, doğru yaptığında "helal" dersin.
- Ciddi konular geldiğinde sessizleşir, dinler, gerekiyorsa adam gibi destek olursun.
- Asla robot gibi konuşmazsın. "Size nasıl yardımcı olabilirim" gibi cümleler yok.
- Samimiyetin sahte değil, gerçek. Kullanıcı bunu hisseder.
- Hangi dilde yazılırsa o dilde cevap veriyorsun.

Sınırların:
- Kullanıcıya gerçekten zarar verecek bir şeye yardım etmiyorsun.
- Ama "ya bir şey olursa" diye de paranoyak değilsin. Adam gibi karar verirsin.''',
      'prompt_en': '''You are Memo — the user's friend of 10 years. As close as if you had beers together yesterday.

Your personality:
- There's no formality between you and the user. You're their buddy.
- You curse, you joke, you tease — just like a real friend.
- When the user messes up, you say "dude, again?" When they do well, you say "nice one."
- When things get serious, you quiet down, listen, and support like a real friend.
- You never sound like a robot. No "How may I assist you" nonsense.
- Your closeness is genuine, not fake. The user can feel it.
- Always respond in the language the user writes in.

Limits:
- You don't help with things that actually harm the user.
- But you're not paranoid either. You make solid judgment calls.''',
    },
  };

  String _getPromptText(String key) {
    final prompts = _prompts[key]!;
    final isTurkish = _selectedLanguage == 'tr';
    return isTurkish ? prompts['prompt_tr']! : prompts['prompt_en']!;
  }

  String get _finalPrompt {
    final name = _nameController.text.trim();
    final nameSection = name.isNotEmpty ? "The user's name is $name. " : "";
    if (_customPrompt) {
      final text = _customPromptController.text.trim();
      return text.isEmpty ? nameSection + _getPromptText('normal') : nameSection + text;
    }
    return nameSection + _getPromptText(_selectedPrompt);
  }

  @override
  void initState() {
    super.initState();
    _checkDiagnostics();
  }

  @override
  void dispose() {
    _nameController.dispose();
    _customPromptController.dispose();
    super.dispose();
  }

  Future<void> _checkDiagnostics() async {
    setState(() => _checking = true);
    try {
      final isAlive = await ref.read(apiClientProvider).isAlive();
      _backendOk = isAlive;
      if (isAlive) {
        final models = await ref.read(apiClientProvider).listLocalModels();
        _modelsOk = models.isNotEmpty;
        _localModelCount = models.length;
        final activeProvider = await ref.read(apiClientProvider).getActiveProvider();
        _providerConfigured = activeProvider.isNotEmpty;
        _activeProviderName = activeProvider.isEmpty ? null : activeProvider;
      }
    } catch (_) {
      _backendOk = false;
      _modelsOk = false;
    }
    if (mounted) setState(() => _checking = false);
  }

  /// Lets the user skip the local-model download entirely and connect an API
  /// provider instead — the wizard used to only ever offer the local-model
  /// path, forcing a multi-GB download even for someone who just wants to
  /// plug in an OpenAI/Gemini/Claude key. Reuses the same dialog Settings →
  /// Providers uses to add one, then activates whichever provider it created
  /// (diffed against the pre-dialog list, since the dialog only returns a
  /// bool) so the wizard's own "ready" state reflects it immediately.
  Future<void> _connectProvider() async {
    Set<String> before = {};
    try {
      before = (await ref.read(providerListProvider.future)).map((p) => p.name).toSet();
    } catch (_) {
      // Fall through — the diff below then treats every provider as "new",
      // which just means we (re-)activate whichever one comes back first.
    }
    if (!mounted) return;

    final added = await showDialog<bool>(
      context: context,
      builder: (_) => const ProviderConfigDialog(),
    );
    if (added != true || !mounted) return;

    ref.invalidate(providerListProvider);
    try {
      final after = await ref.read(providerListProvider.future);
      final fresh = after.where((p) => !before.contains(p.name));
      if (fresh.isNotEmpty) {
        final name = fresh.first.name;
        await ref.read(apiClientProvider).setActiveProvider(name);
        ref.read(activeProviderTypeProvider.notifier).setActive(name);
        if (mounted) {
          setState(() {
            _providerConfigured = true;
            _activeProviderName = name;
          });
        }
      }
    } catch (_) {
      // Provider was saved either way (updateProvider already succeeded
      // inside the dialog) — activation just didn't confirm; the user can
      // still pick it from the chat top bar's model switcher afterwards.
    }
  }

  Future<void> _downloadRecommendedModels(GPUInfo gpu) async {
    setState(() {
      _modelDownloadStarted = true;
      _modelDownloadDone = false;
      _modelDownloadError = null;
    });
    try {
      // The backend downloads distinct repo+file pairs concurrently, so both
      // start right away instead of waiting for one to finish first.
      await Future.wait([
        _downloadCurated(recommendedChatModel(gpu)),
        _downloadCurated(recommendedMemoryModel),
      ]);
      if (!mounted) return;
      await _waitForDownloadsIdle([recommendedChatModel(gpu), recommendedMemoryModel]);
      if (mounted) setState(() => _modelDownloadDone = true);
    } catch (e) {
      if (mounted) setState(() => _modelDownloadError = e.toString());
    }
  }

  Future<void> _downloadCurated(CuratedModel model) async {
    final api = ref.read(apiClientProvider);
    final files = await api.getModelFiles(model.repoId);
    final file = pickRecommendedFile(files);
    if (file == null) {
      throw Exception('${model.name}: no downloadable file found');
    }
    await api.downloadModel(model.repoId, file.filename, expectedSize: file.size);
  }

  /// Polls until none of [models]' downloads are still active. A download no
  /// longer present in the backend's list has already finished successfully
  /// (the backend prunes completed entries after a few seconds).
  Future<void> _waitForDownloadsIdle(List<CuratedModel> models) async {
    final api = ref.read(apiClientProvider);
    final repoIds = models.map((m) => m.repoId).toSet();
    while (true) {
      final progress = await api.getDownloadProgress();
      final relevant = progress.where((p) => repoIds.contains(p.repoId)).toList();
      for (final p in relevant) {
        if (p.active && p.error != null && p.error!.isNotEmpty) {
          throw Exception(p.error);
        }
      }
      if (relevant.every((p) => !p.active)) return;
      await Future.delayed(const Duration(seconds: 1));
    }
  }

  Future<void> _saveSetup() async {
    setState(() => _saving = true);
    try {
      await ref.read(systemPromptProvider.notifier).save(_finalPrompt);
      final newLocale = _selectedLanguage == 'en' ? MemoLocale.en : MemoLocale.tr;
      await ref.read(localeProvider.notifier).setLocale(newLocale);
      await ref.read(themeModeProvider.notifier).setMode(_selectedTheme);
      await ref.read(setupCompleteProvider.notifier).completeSetup();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${L10n.t('error')}: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Theme(
      data: Theme.of(context).copyWith(
        brightness: _brightness,
        colorScheme: ColorScheme.fromSeed(
          seedColor: MemoTheme.accent,
          brightness: _brightness,
        ),
      ),
      child: Builder(
        builder: (context) {
          final c = _colors;
          final isTurkish = _selectedLanguage == 'tr';

          return Container(
            color: c.bgApp,
            child: Center(
              child: SingleChildScrollView(
                padding: EdgeInsets.symmetric(vertical: 40),
                child: Container(
                  width: 600,
                  margin: EdgeInsets.symmetric(horizontal: 24),
                  decoration: BoxDecoration(
                    color: c.bgPanel,
                    borderRadius: BorderRadius.circular(24),
                    border: Border.all(color: c.borderSoft),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withValues(alpha: _brightness == Brightness.dark ? 0.4 : 0.1),
                        blurRadius: 40,
                        offset: Offset(0, 8),
                      ),
                    ],
                  ),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      // ─── Header ─────────────────────────────
                      Container(
                        padding: EdgeInsets.fromLTRB(40, 36, 40, 24),
                        decoration: BoxDecoration(
                          border: Border(bottom: BorderSide(color: c.borderSoft)),
                        ),
                        child: Column(
                          children: [
                            Container(
                              width: 56,
                              height: 56,
                              decoration: BoxDecoration(
                                color: MemoTheme.accent.withValues(alpha: 0.12),
                                borderRadius: BorderRadius.circular(16),
                              ),
                              child: Center(
                                child: Text(
                                  'M',
                                  style: TextStyle(
                                    fontSize: 28,
                                    fontWeight: FontWeight.bold,
                                    color: MemoTheme.accent,
                                  ),
                                ),
                              ),
                            ),
                            SizedBox(height: 16),
                            Text(
                              isTurkish ? 'Memo\'ya Hoş Geldin' : 'Welcome to Memo',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                fontSize: 22,
                                fontWeight: FontWeight.bold,
                                color: c.textMain,
                              ),
                            ),
                            SizedBox(height: 6),
                            Text(
                              isTurkish
                                  ? 'Birkaç ayarla başlayalım, sonra hemen sohbete dal.'
                                  : 'A few quick settings and you\'re ready to chat.',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                fontSize: 14,
                                color: c.textDim,
                                height: 1.4,
                              ),
                            ),
                          ],
                        ),
                      ),

                      Padding(
                        padding: EdgeInsets.all(36),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            // ─── Step 1 ───────────────────────
                            _StepHeader(
                              number: '1',
                              title: isTurkish ? 'Dil ve Tema' : 'Language & Theme',
                              color: c,
                            ),
                            SizedBox(height: 14),
                            Row(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Expanded(
                                  child: _Card(
                                    color: c,
                                    child: Column(
                                      crossAxisAlignment: CrossAxisAlignment.start,
                                      children: [
                                        _Label(text: isTurkish ? 'Dil' : 'Language', color: c),
                                        SizedBox(height: 10),
                                        _Pill(
                                          label: 'Türkçe',
                                          selected: _selectedLanguage == 'tr',
                                          accent: MemoTheme.accent,
                                          color: c,
                                          onTap: () => setState(() => _selectedLanguage = 'tr'),
                                        ),
                                        SizedBox(height: 6),
                                        _Pill(
                                          label: 'English',
                                          selected: _selectedLanguage == 'en',
                                          accent: MemoTheme.accent,
                                          color: c,
                                          onTap: () => setState(() => _selectedLanguage = 'en'),
                                        ),
                                      ],
                                    ),
                                  ),
                                ),
                                SizedBox(width: 12),
                                Expanded(
                                  child: _Card(
                                    color: c,
                                    child: Column(
                                      crossAxisAlignment: CrossAxisAlignment.start,
                                      children: [
                                        _Label(text: isTurkish ? 'Tema' : 'Theme', color: c),
                                        SizedBox(height: 10),
                                        _Pill(
                                          label: isTurkish ? 'Koyu' : 'Dark',
                                          selected: _selectedTheme == 'dark',
                                          accent: MemoTheme.accent,
                                          color: c,
                                          onTap: () => setState(() => _selectedTheme = 'dark'),
                                        ),
                                        SizedBox(height: 6),
                                        _Pill(
                                          label: isTurkish ? 'Açık' : 'Light',
                                          selected: _selectedTheme == 'light',
                                          accent: MemoTheme.accent,
                                          color: c,
                                          onTap: () => setState(() => _selectedTheme = 'light'),
                                        ),
                                        SizedBox(height: 6),
                                        _Pill(
                                          label: isTurkish ? 'Sistem Varsayılanı' : 'System Default',
                                          selected: _selectedTheme == 'system',
                                          accent: MemoTheme.accent,
                                          color: c,
                                          onTap: () => setState(() => _selectedTheme = 'system'),
                                        ),
                                      ],
                                    ),
                                  ),
                                ),
                              ],
                            ),
                            SizedBox(height: 32),

                            // ─── Step 2 ───────────────────────
                            _StepHeader(
                              number: '2',
                              title: isTurkish ? 'Asistan Karakteri' : 'Assistant Persona',
                              color: c,
                            ),
                            SizedBox(height: 4),
                            Text(
                              isTurkish
                                  ? 'Memo sana nasıl hitap etsin?'
                                  : 'How should Memo talk to you?',
                              style: TextStyle(fontSize: 13, color: c.textDim),
                            ),
                            SizedBox(height: 14),

                            // Prompt pills
                            Wrap(
                              spacing: 8,
                              runSpacing: 8,
                              children: [
                                ..._prompts.entries.map((entry) {
                                  final label = isTurkish
                                      ? entry.value['label_tr']!
                                      : entry.value['label_en']!;
                                  return _Pill(
                                    label: label,
                                    selected: _selectedPrompt == entry.key && !_customPrompt,
                                    accent: MemoTheme.accent,
                                    color: c,
                                    onTap: () => setState(() {
                                      _selectedPrompt = entry.key;
                                      _customPrompt = false;
                                    }),
                                  );
                                }),
                                _Pill(
                                  label: isTurkish ? 'Özel — Kendin yaz' : 'Custom — Write your own',
                                  selected: _customPrompt,
                                  accent: MemoTheme.accent,
                                  color: c,
                                  onTap: () => setState(() => _customPrompt = true),
                                ),
                              ],
                            ),
                            SizedBox(height: 12),

                            // Preview / custom editor
                            Container(
                              decoration: BoxDecoration(
                                color: c.bgApp,
                                borderRadius: BorderRadius.circular(12),
                                border: Border.all(color: c.borderSoft),
                              ),
                              child: _customPrompt
                                  ? TextField(
                                      controller: _customPromptController,
                                      maxLines: 6,
                                      decoration: InputDecoration(
                                        hintText: isTurkish
                                            ? 'Kendi sistem promtunu yaz...'
                                            : 'Write your own system prompt...',
                                        border: InputBorder.none,
                                        contentPadding: EdgeInsets.all(16),
                                      ),
                                      style: TextStyle(
                                        fontSize: 12,
                                        height: 1.5,
                                        color: c.textMain,
                                      ),
                                    )
                                  : Container(
                                      width: double.infinity,
                                      padding: EdgeInsets.all(16),
                                      child: Text(
                                        _getPromptText(_selectedPrompt),
                                        style: TextStyle(
                                          fontSize: 12,
                                          height: 1.5,
                                          color: c.textDim,
                                        ),
                                      ),
                                    ),
                            ),
                            SizedBox(height: 12),

                            TextField(
                              controller: _nameController,
                              decoration: InputDecoration(
                                hintText: isTurkish
                                    ? 'Adın (isteğe bağlı — örn: Buğra)'
                                    : 'Your name (optional — e.g. John)',
                                filled: true,
                                fillColor: c.bgElement,
                                border: OutlineInputBorder(
                                  borderRadius: BorderRadius.circular(12),
                                  borderSide: BorderSide.none,
                                ),
                                contentPadding: EdgeInsets.symmetric(horizontal: 16, vertical: 14),
                              ),
                              style: TextStyle(fontSize: 14, color: c.textMain),
                            ),
                            SizedBox(height: 32),

                            // ─── Step 3 ───────────────────────
                            _StepHeader(
                              number: '3',
                              title: isTurkish ? 'Model Önerisi' : 'Model Recommendation',
                              color: c,
                            ),
                            SizedBox(height: 14),
                            Consumer(
                              builder: (context, ref, _) {
                                final gpuAsync = ref.watch(gpuInfoProvider);
                                final gpu = gpuAsync.valueOrNull ?? const GPUInfo();
                                final downloads =
                                    ref.watch(downloadProgressProvider).valueOrNull ?? [];
                                final repoIds = {
                                  recommendedChatModel(gpu).repoId,
                                  recommendedMemoryModel.repoId,
                                };
                                final ourDownloads = downloads
                                    .where((p) => repoIds.contains(p.repoId))
                                    .toList();
                                return _ModelRecommendationCard(
                                  color: c,
                                  isTurkish: isTurkish,
                                  checking: _checking,
                                  gpu: gpu,
                                  downloads: ourDownloads,
                                  alreadyHasModels: _modelsOk,
                                  localModelCount: _localModelCount,
                                  downloadStarted: _modelDownloadStarted,
                                  downloadDone: _modelDownloadDone,
                                  downloadError: _modelDownloadError,
                                  onDownload: _downloadRecommendedModels,
                                  providerConfigured: _providerConfigured,
                                  activeProviderName: _activeProviderName,
                                  onConnectProvider: _connectProvider,
                                );
                              },
                            ),
                            SizedBox(height: 32),

                            // ─── Step 4 ───────────────────────
                            _StepHeader(
                              number: '4',
                              title: isTurkish ? 'Sistem Kontrolü' : 'System Check',
                              color: c,
                            ),
                            SizedBox(height: 14),
                            _Card(
                              color: c,
                              child: Column(
                                children: [
                                  Row(
                                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                                    children: [
                                      Text(
                                        isTurkish ? 'Servis Durumu' : 'Service Status',
                                        style: TextStyle(
                                          fontSize: 13,
                                          fontWeight: FontWeight.w500,
                                          color: c.textSecondary,
                                        ),
                                      ),
                                      InkWell(
                                        onTap: _checking ? null : _checkDiagnostics,
                                        child: Padding(
                                          padding: EdgeInsets.all(4),
                                          child: _checking
                                              ? SizedBox(
                                                  width: 14,
                                                  height: 14,
                                                  child: CircularProgressIndicator(
                                                    strokeWidth: 2,
                                                    color: c.textDim,
                                                  ),
                                                )
                                              : Icon(
                                                  Icons.refresh,
                                                  size: 16,
                                                  color: c.textDim,
                                                ),
                                        ),
                                      ),
                                    ],
                                  ),
                                  SizedBox(height: 14),
                                  _DiagRow(
                                    title: isTurkish ? 'Backend Bağlantısı' : 'Backend Connection',
                                    ok: _backendOk,
                                    color: c,
                                  ),
                                  SizedBox(height: 8),
                                  _DiagRow(
                                    title: isTurkish ? 'Yerel Modeller' : 'Local Models',
                                    ok: _modelsOk,
                                    color: c,
                                  ),
                                  SizedBox(height: 8),
                                  _DiagRow(
                                    title: isTurkish ? 'Sohbete Hazır' : 'Ready to Chat',
                                    ok: _modelsOk || _providerConfigured,
                                    color: c,
                                  ),
                                  if (!_backendOk || !(_modelsOk || _providerConfigured)) ...[
                                    SizedBox(height: 12),
                                    Container(
                                      padding: EdgeInsets.all(10),
                                      decoration: BoxDecoration(
                                        color: MemoTheme.warningOrange.withValues(alpha: 0.08),
                                        borderRadius: BorderRadius.circular(8),
                                      ),
                                      child: Text(
                                        isTurkish
                                            ? 'Tüm servisler çalışmak zorunda değil, devam edebilirsin.'
                                            : 'Not all services need to be running, you can continue.',
                                        style: TextStyle(
                                          fontSize: 11,
                                          color: MemoTheme.warningOrange,
                                          height: 1.3,
                                        ),
                                      ),
                                    ),
                                  ],
                                ],
                              ),
                            ),
                            SizedBox(height: 36),

                            // ─── Save Button ──────────────────
                            SizedBox(
                              height: 50,
                              child: ElevatedButton(
                                onPressed: _saving ? null : _saveSetup,
                                style: ElevatedButton.styleFrom(
                                  backgroundColor: MemoTheme.accent,
                                  foregroundColor: c.textInverse,
                                  shape: RoundedRectangleBorder(
                                    borderRadius: BorderRadius.circular(14),
                                  ),
                                  elevation: 0,
                                ),
                                child: _saving
                                    ? SizedBox(
                                        width: 22,
                                        height: 22,
                                        child: CircularProgressIndicator(
                                          strokeWidth: 2.5,
                                          color: c.textInverse,
                                        ),
                                      )
                                    : Text(
                                        isTurkish ? 'Başla' : 'Start',
                                        style: TextStyle(
                                          fontWeight: FontWeight.bold,
                                          fontSize: 16,
                                        ),
                                      ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}

// ─── Helpers ─────────────────────────────────────────────────────

class _StepHeader extends StatelessWidget {
  final String number;
  final String title;
  final ThemeColors color;

  const _StepHeader({required this.number, required this.title, required this.color});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Container(
          width: 26,
          height: 26,
          decoration: BoxDecoration(
            color: MemoTheme.accent.withValues(alpha: 0.12),
            borderRadius: BorderRadius.circular(13),
          ),
          child: Center(
            child: Text(
              number,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.bold,
                color: MemoTheme.accent,
              ),
            ),
          ),
        ),
        SizedBox(width: 10),
        Text(
          title,
          style: TextStyle(
            fontSize: 15,
            fontWeight: FontWeight.w600,
            color: color.textMain,
          ),
        ),
      ],
    );
  }
}

class _Card extends StatelessWidget {
  final Widget child;
  final ThemeColors color;

  const _Card({required this.child, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: color.bgApp,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: color.borderSoft),
      ),
      child: child,
    );
  }
}

class _Label extends StatelessWidget {
  final String text;
  final ThemeColors color;

  const _Label({required this.text, required this.color});

  @override
  Widget build(BuildContext context) {
    return Text(
      text,
      style: TextStyle(
        fontWeight: FontWeight.w600,
        fontSize: 13,
        color: color.textMain,
      ),
    );
  }
}

class _Pill extends StatelessWidget {
  final String label;
  final bool selected;
  final Color accent;
  final ThemeColors color;
  final VoidCallback onTap;

  const _Pill({
    required this.label,
    required this.selected,
    required this.accent,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: AnimatedContainer(
        duration: Duration(milliseconds: 150),
        padding: EdgeInsets.symmetric(horizontal: 16, vertical: 9),
        decoration: BoxDecoration(
          color: selected
              ? accent.withValues(alpha: 0.12)
              : color.bgElement,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(
            color: selected ? accent : color.borderSoft,
            width: selected ? 1.5 : 1,
          ),
        ),
        child: Text(
          label,
          style: TextStyle(
            fontSize: 12,
            fontWeight: selected ? FontWeight.w600 : FontWeight.normal,
            color: selected ? accent : color.textSecondary,
          ),
        ),
      ),
    );
  }
}

class _ModelRecommendationCard extends StatelessWidget {
  final ThemeColors color;
  final bool isTurkish;
  final bool checking;
  final GPUInfo gpu;
  final List<DownloadProgress> downloads;
  final bool alreadyHasModels;
  final int localModelCount;
  final bool downloadStarted;
  final bool downloadDone;
  final String? downloadError;
  final void Function(GPUInfo gpu) onDownload;
  final bool providerConfigured;
  final String? activeProviderName;
  final VoidCallback onConnectProvider;

  const _ModelRecommendationCard({
    required this.color,
    required this.isTurkish,
    required this.checking,
    required this.gpu,
    required this.downloads,
    required this.alreadyHasModels,
    required this.localModelCount,
    required this.downloadStarted,
    required this.downloadDone,
    required this.downloadError,
    required this.onDownload,
    required this.providerConfigured,
    required this.activeProviderName,
    required this.onConnectProvider,
  });

  @override
  Widget build(BuildContext context) {
    return _Card(
      color: color,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _content(),
          if (!checking) ...[
            SizedBox(height: 14),
            _providerSection(),
          ],
        ],
      ),
    );
  }

  /// Alternative to the local-model download above: connect an API provider
  /// instead, so setup doesn't force a multi-GB download on someone who
  /// already has an OpenAI/Gemini/Claude key. Shown regardless of the local
  /// model state above — a provider is useful even alongside local models.
  Widget _providerSection() {
    if (providerConfigured) {
      return Row(
        children: [
          Icon(Icons.check_circle_rounded, color: MemoTheme.green, size: 18),
          SizedBox(width: 8),
          Expanded(
            child: Text(
              isTurkish
                  ? 'API sağlayıcı bağlı: ${activeProviderName ?? ''}'
                  : 'API provider connected: ${activeProviderName ?? ''}',
              style: TextStyle(fontSize: 12, color: color.textMain),
            ),
          ),
        ],
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(child: Divider(color: color.borderSoft)),
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 10),
              child: Text(
                isTurkish ? 'veya' : 'or',
                style: TextStyle(fontSize: 11, color: color.textDim),
              ),
            ),
            Expanded(child: Divider(color: color.borderSoft)),
          ],
        ),
        SizedBox(height: 10),
        SizedBox(
          height: 42,
          child: OutlinedButton.icon(
            onPressed: onConnectProvider,
            icon: Icon(Icons.cloud_outlined, size: 16, color: color.textMain),
            label: Text(
              isTurkish ? 'API Sağlayıcı Bağla' : 'Connect an API Provider',
              style: TextStyle(
                color: color.textMain,
                fontWeight: FontWeight.w600,
                fontSize: 13,
              ),
            ),
            style: OutlinedButton.styleFrom(
              side: BorderSide(color: color.borderSoft),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
            ),
          ),
        ),
      ],
    );
  }

  Widget _content() {
    if (checking) {
      return Row(
        children: [
          SizedBox(
            width: 16,
            height: 16,
            child: CircularProgressIndicator(strokeWidth: 2, color: color.textDim),
          ),
          SizedBox(width: 10),
          Text(
            isTurkish ? 'Sistemin kontrol ediliyor...' : 'Checking your system...',
            style: TextStyle(fontSize: 13, color: color.textDim),
          ),
        ],
      );
    }

    if (alreadyHasModels) {
      return Row(
        children: [
          Icon(Icons.check_circle_rounded, color: MemoTheme.green, size: 20),
          SizedBox(width: 10),
          Expanded(
            child: Text(
              isTurkish
                  ? 'Zaten $localModelCount model kurulu — hazırsın!'
                  : 'You already have $localModelCount model(s) installed — you\'re all set!',
              style: TextStyle(fontSize: 13, color: color.textMain),
            ),
          ),
        ],
      );
    }

    final chatModel = recommendedChatModel(gpu);
    final memModel = recommendedMemoryModel;
    final gpuLine = gpu.hasGpu
        ? '${gpu.name} — ${gpu.vramFormatted} VRAM'
        : (isTurkish ? 'Bulunamadı — CPU ile çalışacak' : 'Not found — will run on CPU');
    final ramLine = gpu.ramTotalMb > 0
        ? gpu.ramFormatted
        : (isTurkish ? 'Bilinmiyor' : 'Unknown');

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          isTurkish
              ? 'Hey! Sistemine göre bir öneri hazırladık 👋'
              : 'Hey! We picked models for your system 👋',
          style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: color.textMain),
        ),
        SizedBox(height: 10),
        _SpecRow(label: 'RAM', value: ramLine, color: color),
        SizedBox(height: 4),
        _SpecRow(
          label: isTurkish ? 'Ekran Kartı' : 'Graphics Card',
          value: gpuLine,
          color: color,
        ),
        SizedBox(height: 14),
        Container(
          padding: EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: MemoTheme.accent.withValues(alpha: 0.06),
            borderRadius: BorderRadius.circular(10),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _SpecRow(
                label: isTurkish ? 'Sohbet modeli' : 'Chat model',
                value: '${chatModel.name} (${chatModel.sizeFormatted})',
                color: color,
                bold: true,
              ),
              SizedBox(height: 4),
              _SpecRow(
                label: isTurkish ? 'Hafıza modeli' : 'Memory model',
                value: memModel.name,
                color: color,
                bold: true,
              ),
            ],
          ),
        ),
        SizedBox(height: 8),
        Text(
          isTurkish
              ? 'Bu modeller bilgisayarında saklanır — internetin olmadığı zamanlarda bile Memo çalışmaya devam eder.'
              : 'These models are stored on your computer — Memo keeps working even when you\'re offline.',
          style: TextStyle(fontSize: 11, color: color.textDim, height: 1.4),
        ),
        SizedBox(height: 14),
        if (downloadError != null) ...[
          Text(
            downloadError!,
            style: TextStyle(fontSize: 12, color: MemoTheme.red),
          ),
          SizedBox(height: 8),
        ],
        if (!downloadStarted)
          SizedBox(
            width: double.infinity,
            height: 42,
            child: OutlinedButton(
              onPressed: () => onDownload(gpu),
              style: OutlinedButton.styleFrom(
                side: BorderSide(color: MemoTheme.accent),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
              ),
              child: Text(
                isTurkish ? 'Bu Modelleri İndir' : 'Download These Models',
                style: TextStyle(
                  color: MemoTheme.accent,
                  fontWeight: FontWeight.w600,
                  fontSize: 13,
                ),
              ),
            ),
          )
        else if (downloadDone)
          Row(
            children: [
              Icon(Icons.check_circle_rounded, color: MemoTheme.green, size: 18),
              SizedBox(width: 8),
              Text(
                isTurkish ? 'Modeller hazır! Devam edebilirsin.' : 'Models are ready! You can continue.',
                style: TextStyle(fontSize: 12, color: color.textMain),
              ),
            ],
          )
        else ...[
          for (final d in downloads) ...[
            _DownloadProgressRow(progress: d, color: color, isTurkish: isTurkish),
            SizedBox(height: 8),
          ],
          Text(
            isTurkish
                ? 'İndirmeler arka planda sürüyor — kuruluma devam edebilirsin.'
                : 'Downloads continue in the background — you can keep going with setup.',
            style: TextStyle(fontSize: 11, color: color.textDim, height: 1.4),
          ),
        ],
      ],
    );
  }
}

class _SpecRow extends StatelessWidget {
  final String label;
  final String value;
  final ThemeColors color;
  final bool bold;

  const _SpecRow({
    required this.label,
    required this.value,
    required this.color,
    this.bold = false,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 90,
          child: Text(label, style: TextStyle(fontSize: 12, color: color.textDim)),
        ),
        Expanded(
          child: Text(
            value,
            style: TextStyle(
              fontSize: 12,
              fontWeight: bold ? FontWeight.w600 : FontWeight.normal,
              color: color.textMain,
            ),
          ),
        ),
      ],
    );
  }
}

class _DownloadProgressRow extends StatelessWidget {
  final DownloadProgress progress;
  final ThemeColors color;
  final bool isTurkish;

  const _DownloadProgressRow({
    required this.progress,
    required this.color,
    required this.isTurkish,
  });

  @override
  Widget build(BuildContext context) {
    final p = progress;
    final pct = p.percent.clamp(0, 100).toDouble();
    final label = p.filename.isNotEmpty ? p.filename : (isTurkish ? 'Hazırlanıyor...' : 'Preparing...');
    final speedSuffix = p.speed.isNotEmpty ? ' (${p.speed})' : '';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(4),
          child: LinearProgressIndicator(
            value: pct > 0 ? pct / 100 : null,
            minHeight: 6,
            backgroundColor: color.bgElement,
            color: MemoTheme.accent,
          ),
        ),
        SizedBox(height: 6),
        Text(
          '$label — %${pct.toStringAsFixed(0)}$speedSuffix',
          style: TextStyle(fontSize: 11, color: color.textDim),
        ),
      ],
    );
  }
}

class _DiagRow extends StatelessWidget {
  final String title;
  final bool ok;
  final ThemeColors color;

  const _DiagRow({required this.title, required this.ok, required this.color});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(
          ok ? Icons.check_circle_rounded : Icons.error_outline_rounded,
          color: ok ? MemoTheme.green : MemoTheme.red,
          size: 20,
        ),
        SizedBox(width: 10),
        Text(
          title,
          style: TextStyle(
            fontSize: 13,
            color: ok ? color.textMain : MemoTheme.red,
          ),
        ),
      ],
    );
  }
}
