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
  /// Whether this model supports OpenAI-style tool/function calling.
  /// Required for Agent mode to work with a local model.
  final bool supportsTools;
  /// Overrides the author extracted from [repoId] for avatar/logo lookup.
  /// Set when the uploader org differs from the original model creator.
  final String? brandAuthor;

  const CuratedModel({
    required this.repoId,
    required this.name,
    required this.descTr,
    required this.descEn,
    required this.approxBytes,
    required this.kind,
    this.supportsTools = false,
    this.brandAuthor,
  });

  String get desc => L10n.locale == MemoLocale.tr ? descTr : descEn;

  /// The author used for avatar/logo display.
  String get displayAuthor =>
      brandAuthor ?? (repoId.contains('/') ? repoId.split('/').first : repoId);
}

/// Curated GGUF repos — only official repos from the original model creators.
/// Filenames are resolved at download time via `getModelFiles`.
const curatedModels = <CuratedModel>[
  // ── Google — official QAT GGUF releases ─────────────────────────
  CuratedModel(
    repoId: 'google/gemma-3-4b-it-qat-q4_0-gguf',
    name: 'Gemma 3 4B Instruct',
    descTr: 'Google\'dan hafif ve güçlü · Araç desteği',
    descEn: 'Lightweight powerhouse from Google · Tool use',
    approxBytes: 2530000000,
    kind: ModelKind.chat,
    supportsTools: true,
  ),
  CuratedModel(
    repoId: 'google/gemma-3-12b-it-qat-q4_0-gguf',
    name: 'Gemma 3 12B Instruct',
    descTr: 'Görüntü + sohbet + araç · Google',
    descEn: 'Vision + chat + tools · Google',
    approxBytes: 7600000000,
    kind: ModelKind.vision,
    supportsTools: true,
  ),
  // ── Qwen / Alibaba — official GGUF releases ──────────────────────
  CuratedModel(
    repoId: 'Qwen/Qwen3-8B-GGUF',
    name: 'Qwen 3 8B',
    descTr: 'En yeni Qwen · Akıl yürütme + araç desteği',
    descEn: 'Latest Qwen · Reasoning + tool use',
    approxBytes: 5200000000,
    kind: ModelKind.chat,
    supportsTools: true,
  ),
  CuratedModel(
    repoId: 'Qwen/Qwen2.5-7B-Instruct-GGUF',
    name: 'Qwen 2.5 7B Instruct',
    descTr: 'Kod + sohbet, dengeli · Araç desteği',
    descEn: 'Balanced code + chat · Tool use',
    approxBytes: 4680000000,
    kind: ModelKind.chat,
    supportsTools: true,
  ),
  CuratedModel(
    repoId: 'Qwen/Qwen2.5-14B-Instruct-GGUF',
    name: 'Qwen 2.5 14B Instruct',
    descTr: 'Büyük model · Kod + akıl yürütme',
    descEn: 'Larger model · Code + reasoning',
    approxBytes: 8730000000,
    kind: ModelKind.chat,
    supportsTools: true,
  ),
  CuratedModel(
    repoId: 'Qwen/Qwen2.5-Coder-7B-Instruct-GGUF',
    name: 'Qwen 2.5 Coder 7B',
    descTr: 'Kod yazımı için özelleştirilmiş model',
    descEn: 'Specialist coding model',
    approxBytes: 4680000000,
    kind: ModelKind.chat,
    supportsTools: true,
  ),
  // ── Microsoft — official GGUF releases ──────────────────────────
  CuratedModel(
    repoId: 'microsoft/Phi-3-mini-4k-instruct-gguf',
    name: 'Phi-3 mini 4K Instruct',
    descTr: 'Microsoft · Hafif ve hızlı · 3.8B',
    descEn: 'Microsoft · Lightweight and fast · 3.8B',
    approxBytes: 2390000000,
    kind: ModelKind.chat,
    supportsTools: true,
  ),
  // ── Embedding / Memory ──────────────────────────────────────────
  CuratedModel(
    repoId: 'nomic-ai/nomic-embed-text-v1.5-GGUF',
    name: 'Nomic Embed v1.5',
    descTr: 'Hafıza için vektör modeli',
    descEn: 'Vector model for memory',
    approxBytes: 84000000,
    kind: ModelKind.memory,
    supportsTools: false,
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
  if (f.contains('q4_0')) return QuantInfo(L10n.t('quant_balanced'), 95);
  if (f.contains('q4_1')) return QuantInfo(L10n.t('quant_small'), 78);
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
