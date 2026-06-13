import '../core/l10n.dart';
import 'gpu_info.dart';
import 'local_model.dart';

/// What a model is for, in plain user terms.
enum ModelKind { chat, memory, vision }

/// A hand-picked, known-good model surfaced on the Discover tab so newcomers
/// don't have to wade through raw HuggingFace search and cryptic quant names.
class CuratedModel {
  final String repoId;
  final String name;
  final String descTr;
  final String descEn;
  final int approxBytes; // recommended (Q4_K_M) file size, for display + fit hint
  final ModelKind kind;

  const CuratedModel({
    required this.repoId,
    required this.name,
    required this.descTr,
    required this.descEn,
    required this.approxBytes,
    required this.kind,
  });

  String get desc => L10n.locale == MemoLocale.tr ? descTr : descEn;
}

/// Stable bartowski/nomic GGUF repos. Exact filenames are resolved at download
/// time via `getModelFiles`, so only the repo ids need to stay valid.
const curatedModels = <CuratedModel>[
  CuratedModel(
    repoId: 'bartowski/Meta-Llama-3.1-8B-Instruct-GGUF',
    name: 'Llama 3.1 8B Instruct',
    descTr: 'Genel sohbet · Türkçe iyi',
    descEn: 'General chat · good Turkish',
    approxBytes: 4920000000,
    kind: ModelKind.chat,
  ),
  CuratedModel(
    repoId: 'bartowski/Qwen2.5-7B-Instruct-GGUF',
    name: 'Qwen 2.5 7B Instruct',
    descTr: 'Kod + sohbet, dengeli',
    descEn: 'Balanced code + chat',
    approxBytes: 4680000000,
    kind: ModelKind.chat,
  ),
  CuratedModel(
    repoId: 'bartowski/Phi-3.5-mini-instruct-GGUF',
    name: 'Phi-3.5 mini',
    descTr: 'Hafif ve hızlı',
    descEn: 'Lightweight and fast',
    approxBytes: 2390000000,
    kind: ModelKind.chat,
  ),
  CuratedModel(
    repoId: 'nomic-ai/nomic-embed-text-v1.5-GGUF',
    name: 'Nomic Embed v1.5',
    descTr: 'Hafıza için küçük model',
    descEn: 'Small model for memory',
    approxBytes: 84000000,
    kind: ModelKind.memory,
  ),
];

// ─── Hardware fit ───────────────────────────────────────────────

enum FitLevel { good, ok, warn }

class HardwareFit {
  final FitLevel level;
  final String label;
  const HardwareFit(this.level, this.label);
}

/// Conservative, hardware-aware verdict on whether a model of [sizeBytes] is a
/// good match. Prefers VRAM when a GPU is present, falls back to system RAM,
/// then to absolute-size tiers when neither is known. Language is deliberately
/// hedged ("may be insufficient") because the heuristic is approximate.
HardwareFit hardwareFit(int sizeBytes, GPUInfo gpu) {
  final gb = sizeBytes / 1e9;

  if (gpu.hasGpu && gpu.vramMb > 0) {
    final vramGb = gpu.vramMb / 1024;
    if (gb < 0.85 * vramGb) return HardwareFit(FitLevel.good, L10n.t('fit_gpu_fast'));
    if (gb < 1.4 * vramGb) return HardwareFit(FitLevel.ok, L10n.t('fit_gpu_cpu'));
  }

  if (gpu.ramTotalMb > 0) {
    final ramGb = gpu.ramTotalMb / 1024;
    if (gb < 0.5 * ramGb) return HardwareFit(FitLevel.good, L10n.t('fit_good'));
    if (gb < 0.75 * ramGb) return HardwareFit(FitLevel.ok, L10n.t('fit_cpu_ok'));
    return HardwareFit(FitLevel.warn, L10n.t('fit_insufficient'));
  }

  if (gb < 3) return HardwareFit(FitLevel.ok, L10n.t('fit_most'));
  if (gb < 6) return HardwareFit(FitLevel.warn, L10n.t('fit_strong'));
  return HardwareFit(FitLevel.warn, L10n.t('fit_heavy'));
}

// ─── Quantization → plain language ──────────────────────────────

class QuantInfo {
  final String label;
  final int priority; // higher = preferred default
  const QuantInfo(this.label, this.priority);
}

/// Translates a GGUF quant tag in [filename] into a plain-language quality hint.
QuantInfo quantInfo(String filename) {
  final f = filename.toLowerCase();
  if (f.contains('q4_k_m')) return QuantInfo(L10n.t('quant_balanced'), 100);
  if (f.contains('q5_k_m')) return QuantInfo(L10n.t('quant_high'), 90);
  if (f.contains('q5_k_s')) return QuantInfo(L10n.t('quant_high'), 85);
  if (f.contains('q4_k_s')) return QuantInfo(L10n.t('quant_small'), 80);
  if (f.contains('q6_k')) return QuantInfo(L10n.t('quant_very_high'), 75);
  if (f.contains('q8_0')) return QuantInfo(L10n.t('quant_highest'), 70);
  if (f.contains('q3_k')) return QuantInfo(L10n.t('quant_smallest'), 60);
  if (f.contains('q2_k')) return QuantInfo(L10n.t('quant_smallest'), 55);
  if (f.contains('fp16') || f.contains('f16')) return QuantInfo(L10n.t('quant_full'), 50);
  return QuantInfo(L10n.t('quant_standard'), 40);
}

/// Picks the best default GGUF from a repo's file list: prefer the balanced
/// Q4_K_M, otherwise the highest-priority quant. Skips multi-part split files
/// (those are left to advanced users). Returns null if no single-file GGUF.
GGUFFile? pickRecommendedFile(List<GGUFFile> files) {
  final ggufs = files
      .where((f) => f.filename.toLowerCase().endsWith('.gguf'))
      .where((f) => !RegExp(r'-\d{5}-of-\d{5}').hasMatch(f.filename.toLowerCase()))
      .toList();
  if (ggufs.isEmpty) return null;
  for (final f in ggufs) {
    if (f.filename.toLowerCase().contains('q4_k_m')) return f;
  }
  ggufs.sort((a, b) => quantInfo(b.filename).priority.compareTo(quantInfo(a.filename).priority));
  return ggufs.first;
}
