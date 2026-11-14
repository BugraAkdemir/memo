import 'package:dio/dio.dart';
import 'package:flutter/material.dart';

import '../../core/theme.dart';

/// Configuration payload emitted when the user completes the KorteksOS
/// First-Boot OOBE Wizard (POST /api/os/oobe/complete).
class KorteksOobeConfig {
  const KorteksOobeConfig({
    required this.ringMode,
    required this.apuBoost,
    required this.selectedModel,
    required this.distroboxStacks,
    required this.voiceEnabled,
  });

  final String ringMode;
  final bool apuBoost;
  final String selectedModel;
  final List<String> distroboxStacks;
  final bool voiceEnabled;

  Map<String, dynamic> toJson() => {
        'ring_mode': ringMode,
        'apu_boost': apuBoost,
        'selected_model': selectedModel,
        'distrobox_stacks': distroboxStacks,
        'voice_enabled': voiceEnabled,
      };
}

typedef KorteksOobeSubmit = Future<void> Function(KorteksOobeConfig config);

/// Interactive 5-step First-Boot OOBE Wizard for KorteksOS (KortexLinux),
/// guiding hardware APU detection, local GGUF model selection, 3-Ring Polkit
/// security policy, 1-click Distrobox developer stacks, and voice activation.
class KorteksOobeScreen extends StatefulWidget {
  const KorteksOobeScreen({
    super.key,
    this.backendBaseUrl = 'http://127.0.0.1:8090',
    this.initialHardwareSummary = 'AMD Ryzen 5 APU • 16 GB RAM • Vulkan UMA Ready',
    this.onSubmitConfig,
    this.onCompleted,
  });

  final String backendBaseUrl;
  final String initialHardwareSummary;
  final KorteksOobeSubmit? onSubmitConfig;
  final VoidCallback? onCompleted;

  @override
  State<KorteksOobeScreen> createState() => _KorteksOobeScreenState();
}

class _KorteksOobeScreenState extends State<KorteksOobeScreen> {
  int _step = 0;
  bool _apuBoost = true;
  String _selectedModel = 'Qwen2.5-Coder-7B-Instruct-Q4_K_M';
  String _ringMode = 'hybrid';
  final Set<String> _stacks = {'go', 'python-ai'};
  bool _voiceEnabled = true;
  bool _submitting = false;
  String? _error;

  static const _stepTitles = [
    'Donanım & APU Boost',
    'Yerel AI & Sağlayıcı',
    '3-Ring Yetki & Btrfs',
    'Distrobox Geliştirici Stack',
    'Sesli HUD & Tamamlama',
  ];

  static const _availableStacks = <String, String>{
    'go': 'Go 1.26 Toolchain',
    'rust': 'Rustup + Cargo',
    'python-ai': 'Python 3.12 + PyTorch ROCm/CPU',
    'node': 'Node.js LTS + pnpm',
    'flutter': 'Flutter Linux Desktop SDK',
  };

  Future<void> _submit() async {
    setState(() {
      _submitting = true;
      _error = null;
    });
    final cfg = KorteksOobeConfig(
      ringMode: _ringMode,
      apuBoost: _apuBoost,
      selectedModel: _selectedModel,
      distroboxStacks: _stacks.toList()..sort(),
      voiceEnabled: _voiceEnabled,
    );
    try {
      if (widget.onSubmitConfig != null) {
        await widget.onSubmitConfig!(cfg);
      } else {
        final dio = Dio(BaseOptions(baseUrl: widget.backendBaseUrl));
        await dio.post('/api/os/oobe/complete', data: cfg.toJson());
      }
      widget.onCompleted?.call();
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = 'Kurulum kaydedilemedi: $e';
        });
      }
    } finally {
      if (mounted) {
        setState(() {
          _submitting = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.dark;

    return Scaffold(
      backgroundColor: c.bgApp,
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 760),
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Material(
                color: c.bgPanel,
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
                  side: BorderSide(color: c.borderHover),
                ),
                clipBehavior: Clip.antiAlias,
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      _buildHeader(c),
                      const SizedBox(height: 16),
                      _buildStepPills(c),
                      const SizedBox(height: 20),
                      Expanded(
                        child: SingleChildScrollView(
                          child: _buildStepBody(c),
                        ),
                      ),
                      if (_error != null) ...[
                        const SizedBox(height: 12),
                        Text(
                          _error!,
                          style: const TextStyle(color: MemoTheme.red, fontSize: 13),
                        ),
                      ],
                      const SizedBox(height: 16),
                      _buildFooter(c),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildHeader(ThemeColors c) {
    return Row(
      children: [
        Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            color: MemoTheme.accentMuted,
            borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
            border: Border.all(color: MemoTheme.accent),
          ),
          child: const Icon(Icons.memory, color: MemoTheme.accentLight),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'KorteksOS İlk Kurulum Sihirbazı (OOBE)',
                style: TextStyle(
                  color: c.textMain,
                  fontSize: 20,
                  fontWeight: FontWeight.w600,
                ),
              ),
              Text(
                'Ring-1 İşletim Sistemi Çekirdeği • Memo v3 Embedded',
                style: TextStyle(color: c.textMuted, fontSize: 13),
              ),
            ],
          ),
        ),
        Text(
          'Adım ${_step + 1}/${_stepTitles.length}',
          style: const TextStyle(
            color: MemoTheme.accentLight,
            fontWeight: FontWeight.w600,
          ),
        ),
      ],
    );
  }

  Widget _buildStepPills(ThemeColors c) {
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: List.generate(_stepTitles.length, (idx) {
        final active = idx == _step;
        final done = idx < _step;
        return InkWell(
          onTap: () => setState(() => _step = idx),
          borderRadius: BorderRadius.circular(20),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            decoration: BoxDecoration(
              color: active
                  ? MemoTheme.accentMuted
                  : done
                      ? c.bgElement
                      : Colors.transparent,
              borderRadius: BorderRadius.circular(20),
              border: Border.all(
                color: active ? MemoTheme.accent : c.borderSoft,
              ),
            ),
            child: Text(
              '${idx + 1}. ${_stepTitles[idx]}',
              style: TextStyle(
                color: active ? MemoTheme.accentLight : c.textSecondary,
                fontSize: 12,
                fontWeight: active ? FontWeight.w600 : FontWeight.w400,
              ),
            ),
          ),
        );
      }),
    );
  }

  Widget _buildStepBody(ThemeColors c) {
    switch (_step) {
      case 0:
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _infoBanner(
              c,
              icon: Icons.developer_board,
              title: 'Algılanan Donanım Profili',
              subtitle: widget.initialHardwareSummary,
            ),
            const SizedBox(height: 16),
            SwitchListTile.adaptive(
              key: const Key('oobe_apu_boost_switch'),
              contentPadding: EdgeInsets.zero,
              title: Text(
                'korteks-apu-boost (Vulkan UMA Zero-Copy + 10 GB GTT)',
                style: TextStyle(color: c.textMain, fontWeight: FontWeight.w500),
              ),
              subtitle: Text(
                'Ryzen APU ve 16 GB RAM sistemlerde paylaşımlı VRAM havuzunu dinamik optimize eder.',
                style: TextStyle(color: c.textMuted, fontSize: 13),
              ),
              value: _apuBoost,
              activeThumbColor: MemoTheme.accent,
              onChanged: (v) => setState(() => _apuBoost = v),
            ),
          ],
        );

      case 1:
        const models = [
          (
            'Qwen2.5-Coder-7B-Instruct-Q4_K_M',
            'Qwen 2.5 Coder 7B (Q4_K_M • ~4.7 GB)',
            'Kodlama, Linux yönetimi ve Türkçe/İngilizce ajan görevleri için önerilen.'
          ),
          (
            'Llama-3.2-3B-Instruct-Q4_K_M',
            'Llama 3.2 3B (Q4_K_M • ~2.1 GB)',
            'Minimum RAM kullanımı ve düşük gecikmeli hızlı yanıtlar için.'
          ),
          (
            'cloud-only',
            'Yalnızca Bulut Sağlayıcı (OpenRouter / Anthropic / Gemini)',
            'Yerel VRAM ayırmadan harici API anahtarıyla çalıştırır.'
          ),
        ];
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Varsayılan Beyin Modelini Seçin',
              style: TextStyle(
                color: c.textMain,
                fontSize: 16,
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: 12),
            RadioGroup<String>(
              groupValue: _selectedModel,
              onChanged: (v) {
                if (v != null) setState(() => _selectedModel = v);
              },
              child: Column(
                children: models
                    .map((m) => RadioListTile<String>(
                          key: Key('oobe_model_${m.$1}'),
                          contentPadding: EdgeInsets.zero,
                          activeColor: MemoTheme.accent,
                          title: Text(m.$2, style: TextStyle(color: c.textMain)),
                          subtitle: Text(m.$3,
                              style:
                                  TextStyle(color: c.textMuted, fontSize: 12)),
                          value: m.$1,
                        ))
                    .toList(),
              ),
            ),
          ],
        );

      case 2:
        const modes = [
          (
            'locked',
            'Locked (Sadece Gözlem)',
            'Memo yalnızca Ring-0 okuma ve masaüstü düzenleme yapar; sistem değişikliği yapmaz.'
          ),
          (
            'hybrid',
            'Hybrid (Önerilen — Pre-Snapshot + Onay)',
            'Ring-1/Ring-2 işlemlerden önce otomatik @snapshots Btrfs CoW yedeği alır ve onay ister.'
          ),
          (
            'autonomous',
            'Autonomous (Tam Otonom Ring-1)',
            'Servis/paket güncellemelerini pre-snapshot güvencesiyle doğrudan yürütür (@memo-vault her zaman korunur).'
          ),
        ];
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _infoBanner(
              c,
              icon: Icons.security,
              title: 'Btrfs @memo-vault İzolasyonu Aktif',
              subtitle:
                  '/var/lib/korteks/vault altındaki anılarınız ve modelleriniz OS geri yüklemelerinde asla silinmez.',
            ),
            const SizedBox(height: 12),
            RadioGroup<String>(
              groupValue: _ringMode,
              onChanged: (v) {
                if (v != null) setState(() => _ringMode = v);
              },
              child: Column(
                children: modes
                    .map((m) => RadioListTile<String>(
                          key: Key('oobe_ring_${m.$1}'),
                          contentPadding: EdgeInsets.zero,
                          activeColor: MemoTheme.accent,
                          title: Text(m.$2, style: TextStyle(color: c.textMain)),
                          subtitle: Text(m.$3,
                              style:
                                  TextStyle(color: c.textMuted, fontSize: 12)),
                          value: m.$1,
                        ))
                    .toList(),
              ),
            ),
          ],
        );

      case 3:
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '1-Tıkla Distrobox Geliştirici Konteynerleri',
              style: TextStyle(
                color: c.textMain,
                fontSize: 16,
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: 6),
            Text(
              'Immutable host sisteminizi kirletmeden izole konteyner geliştirme ortamları oluşturun:',
              style: TextStyle(color: c.textMuted, fontSize: 13),
            ),
            const SizedBox(height: 16),
            Wrap(
              spacing: 10,
              runSpacing: 10,
              children: _availableStacks.entries.map((e) {
                final selected = _stacks.contains(e.key);
                return FilterChip(
                  key: Key('oobe_stack_${e.key}'),
                  label: Text(e.value),
                  selected: selected,
                  selectedColor: MemoTheme.accentMuted,
                  checkmarkColor: MemoTheme.accentLight,
                  labelStyle: TextStyle(
                    color: selected ? MemoTheme.accentLight : c.textSecondary,
                  ),
                  side: BorderSide(
                    color: selected ? MemoTheme.accent : c.borderSoft,
                  ),
                  onSelected: (val) {
                    setState(() {
                      if (val) {
                        _stacks.add(e.key);
                      } else {
                        _stacks.remove(e.key);
                      }
                    });
                  },
                );
              }).toList(),
            ),
          ],
        );

      default:
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SwitchListTile.adaptive(
              key: const Key('oobe_voice_switch'),
              contentPadding: EdgeInsets.zero,
              title: Text(
                'Super+M Küresel Sesli HUD (whisper.cpp + Piper TTS)',
                style: TextStyle(color: c.textMain, fontWeight: FontWeight.w500),
              ),
              subtitle: Text(
                'Herhangi bir pencere üzerinde Super+M kısayoluyla sesli komut ve anlık sistem kontrolü açar.',
                style: TextStyle(color: c.textMuted, fontSize: 13),
              ),
              value: _voiceEnabled,
              activeThumbColor: MemoTheme.accent,
              onChanged: (v) => setState(() => _voiceEnabled = v),
            ),
            const SizedBox(height: 16),
            _infoBanner(
              c,
              icon: Icons.check_circle_outline,
              title: 'Özet Yapılandırma',
              subtitle:
                  'Yetki Modu: $_ringMode • APU Boost: ${_apuBoost ? "Aktif" : "Kapalı"} • Model: $_selectedModel • Stack: ${_stacks.isEmpty ? "yok" : _stacks.join(", ")}',
            ),
          ],
        );
    }
  }

  Widget _infoBanner(
    ThemeColors c, {
    required IconData icon,
    required String title,
    required String subtitle,
  }) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: c.bgElement,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: c.borderSoft),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, color: MemoTheme.accentLight, size: 22),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: TextStyle(
                    color: c.textMain,
                    fontWeight: FontWeight.w600,
                    fontSize: 14,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  subtitle,
                  style: TextStyle(color: c.textMuted, fontSize: 13),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildFooter(ThemeColors c) {
    final isLast = _step == _stepTitles.length - 1;
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        if (_step > 0)
          OutlinedButton(
            key: const Key('oobe_back_btn'),
            onPressed: _submitting ? null : () => setState(() => _step--),
            child: const Text('Geri'),
          )
        else
          const SizedBox.shrink(),
        ElevatedButton.icon(
          key: Key(isLast ? 'oobe_finish_btn' : 'oobe_next_btn'),
          style: ElevatedButton.styleFrom(
            backgroundColor: MemoTheme.accent,
            foregroundColor: c.textInverse,
            padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
          ),
          onPressed: _submitting
              ? null
              : () {
                  if (!isLast) {
                    setState(() => _step++);
                  } else {
                    _submit();
                  }
                },
          icon: Icon(isLast ? Icons.rocket_launch : Icons.arrow_forward),
          label: Text(isLast ? 'KorteksOS\'i Başlat' : 'İleri'),
        ),
      ],
    );
  }
}
