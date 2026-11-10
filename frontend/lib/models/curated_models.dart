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
  /// The model's native context window in tokens (0 = not a chat model or
  /// unknown). Below 8k a chat leaves little room, so such a model is only
  /// recommended when nothing else fits (see [recommendedChatModel]).
  final int contextTokens;

  const CuratedModel({
    required this.repoId,
    required this.name,
    required this.descTr,
    required this.descEn,
    required this.approxBytes,
    required this.kind,
    this.supportsTools = false,
    this.brandAuthor,
    this.contextTokens = 0,
  });

  String get desc => L10n.locale == MemoLocale.tr ? descTr : descEn;

  /// The author used for avatar/logo display.
  String get displayAuthor =>
      brandAuthor ?? (repoId.contains('/') ? repoId.split('/').first : repoId);

  String get sizeFormatted {
    if (approxBytes >= 1024 * 1024 * 1024) {
      return '${(approxBytes / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
    }
    return '${(approxBytes / (1024 * 1024)).toStringAsFixed(0)} MB';
  }
}

/// Curated GGUF repos — only official repos from the original model creators.
/// Filenames are resolved at download time via `getModelFiles`.
const curatedModels = <CuratedModel>[
  // ── Google's official Gemma 3 GGUF releases are deliberately NOT
  // curated here: huggingface.co/api/models/{repoId} reports
  // "gated": "manual" for them (verified directly against HF's API, not
  // guessed) — anonymous, unauthenticated access (all Memo ever does; no
  // HF login/token support exists) gets a 401 on every actual file
  // (README, GGUF download) even though the repo's metadata and file
  // *listing* both resolve fine, which is exactly what made this
  // confusing to spot: the Discover detail panel looked normal right up
  // until the user actually pressed Download. A curated "recommended"
  // model that can never actually be downloaded is worse than not
  // recommending one at all.
  // ── Qwen / Alibaba — official GGUF releases ──────────────────────
  // Sizes are the single-file Q4_K_M (or, where Qwen publishes only that,
  // Q8_0) from each repo's own file listing, checked 2026-11-10. Qwen 2.5 7B
  // and 14B were dropped then: their Q4_K_M is split into parts, which
  // pickRecommendedFile skips, so 7B fell back to a q3 file and 14B had no
  // downloadable file at all. Qwen 3 4B/8B/14B replace them.
  CuratedModel(
    repoId: 'Qwen/Qwen3-14B-GGUF',
    name: 'Qwen 3 14B',
    descTr: 'Güçlü donanım için büyük model · Akıl yürütme + araç desteği',
    descEn: 'Large model for strong hardware · Reasoning + tool use',
    approxBytes: 9001752960,
    kind: ModelKind.chat,
    supportsTools: true,
    contextTokens: 32768,
  ),
  CuratedModel(
    repoId: 'Qwen/Qwen3-8B-GGUF',
    name: 'Qwen 3 8B',
    descTr: 'Dengeli · Akıl yürütme + araç desteği',
    descEn: 'Balanced · Reasoning + tool use',
    approxBytes: 5027783488,
    kind: ModelKind.chat,
    supportsTools: true,
    contextTokens: 32768,
  ),
  CuratedModel(
    repoId: 'Qwen/Qwen2.5-Coder-7B-Instruct-GGUF',
    name: 'Qwen 2.5 Coder 7B',
    descTr: 'Kod yazımı için özelleştirilmiş model',
    descEn: 'Specialist coding model',
    approxBytes: 4680000000,
    kind: ModelKind.chat,
    supportsTools: true,
    contextTokens: 32768,
  ),
  CuratedModel(
    repoId: 'Qwen/Qwen3-4B-GGUF',
    name: 'Qwen 3 4B',
    descTr: 'Orta donanım için hafif ve yetenekli · Araç desteği',
    descEn: 'Light and capable for mid-range hardware · Tool use',
    approxBytes: 2497280256,
    kind: ModelKind.chat,
    supportsTools: true,
    contextTokens: 32768,
  ),
  CuratedModel(
    repoId: 'Qwen/Qwen2.5-3B-Instruct-GGUF',
    name: 'Qwen 2.5 3B Instruct',
    descTr: 'Zayıf donanım için hafif model · Araç desteği',
    descEn: 'Lightweight model for modest hardware · Tool use',
    approxBytes: 2104932768,
    kind: ModelKind.chat,
    supportsTools: true,
    contextTokens: 32768,
  ),
  CuratedModel(
    repoId: 'Qwen/Qwen3-1.7B-GGUF',
    name: 'Qwen 3 1.7B',
    descTr: 'Çok zayıf donanım için en küçük model · Araç desteği',
    descEn: 'Smallest model for very modest hardware · Tool use',
    approxBytes: 1834426016,
    kind: ModelKind.chat,
    supportsTools: true,
    contextTokens: 32768,
  ),
  // ── Microsoft — official GGUF releases ──────────────────────────
  CuratedModel(
    repoId: 'microsoft/Phi-3-mini-4k-instruct-gguf',
    name: 'Phi-3 mini 4K Instruct',
    descTr: 'Microsoft · Hafif ve hızlı · 3.8B · 4K bağlam',
    descEn: 'Microsoft · Lightweight and fast · 3.8B · 4K context',
    approxBytes: 2390000000,
    kind: ModelKind.chat,
    supportsTools: true,
    contextTokens: 4096,
  ),
  // ── OpenBMB — official GGUF release, non-gated (verified) ────────
  CuratedModel(
    repoId: 'openbmb/MiniCPM-V-2_6-gguf',
    name: 'MiniCPM-V 2.6',
    descTr: 'Görüntü + sohbet · OpenBMB, 8B',
    descEn: 'Vision + chat · OpenBMB, 8B',
    approxBytes: 4681089344, // Q4_K_M, verified against the repo's own file listing
    kind: ModelKind.vision,
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

/// A chat window below this is a poor default: the conversation has little
/// room (Memo runs such models in its compact mode).
const kRoomyContextTokens = 8192;

/// Picks the best curated chat model for the given hardware: among models
/// with a roomy context window ([kRoomyContextTokens]), the largest that gets
/// a "good" fit, then the largest with an "ok" fit; only then the same over
/// every chat model; finally the smallest model overall so there's always a
/// usable recommendation even on weak hardware. The specialist coder model is
/// never the general default.
CuratedModel recommendedChatModel(GPUInfo gpu) {
  final chatModels = curatedModels
      .where((m) => m.kind == ModelKind.chat && !m.repoId.contains('Coder'))
      .toList()
    ..sort((a, b) => b.approxBytes.compareTo(a.approxBytes));
  final roomy = chatModels.where((m) => m.contextTokens >= kRoomyContextTokens).toList();
  for (final pool in [roomy, chatModels]) {
    for (final level in [FitLevel.good, FitLevel.ok]) {
      for (final m in pool) {
        if (hardwareFit(m.approxBytes, gpu).level == level) return m;
      }
    }
  }
  return chatModels.last;
}

/// Whether Memo knows enough about this machine to judge a model's fit.
bool hardwareKnown(GPUInfo gpu) => (gpu.hasGpu && gpu.vramMb > 0) || gpu.ramTotalMb > 0;

/// The Discover home list for this machine: only official models that fit it,
/// the recommended chat model first. Models that would not run well here
/// ([FitLevel.warn]) are left out — except the recommended one, so a weak
/// machine still gets one suggestion. The memory model always stays. With the
/// hardware unknown the full list comes back unchanged and nothing is marked.
({List<CuratedModel> models, CuratedModel? recommended}) curatedForSystem(GPUInfo gpu) {
  if (!hardwareKnown(gpu)) return (models: curatedModels, recommended: null);
  final rec = recommendedChatModel(gpu);
  int rank(CuratedModel m) => switch (m.kind) {
        ModelKind.chat => 0,
        ModelKind.vision => 1,
        ModelKind.memory => 2,
      };
  final fits = curatedModels.where((m) {
    if (m == rec || m.kind == ModelKind.memory) return true;
    return hardwareFit(m.approxBytes, gpu).level != FitLevel.warn;
  }).toList()
    ..sort((a, b) {
      if (a == rec) return -1;
      if (b == rec) return 1;
      final k = rank(a).compareTo(rank(b));
      if (k != 0) return k;
      final fa = hardwareFit(a.approxBytes, gpu).level.index;
      final fb = hardwareFit(b.approxBytes, gpu).level.index;
      if (fa != fb) return fa.compareTo(fb);
      return b.approxBytes.compareTo(a.approxBytes);
    });
  return (models: fits, recommended: rec);
}

/// The curated embedding model backing RAG memory — recommended alongside a
/// chat model regardless of hardware, since memory search needs it either way.
CuratedModel get recommendedMemoryModel =>
    curatedModels.firstWhere((m) => m.kind == ModelKind.memory);

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
