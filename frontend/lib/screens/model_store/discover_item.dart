import 'package:flutter/material.dart';

import '../../models/curated_models.dart';
import '../../models/local_model.dart';

// ─── Unified discover item ────────────────────────────────────────

@immutable
class DiscoverItem {
  final String repoId;
  final String author;
  /// Override author used for avatar/logo fetch (e.g. the original model creator
  /// when the uploader is a community quantizer).
  final String? brandAuthor;
  final String displayName;
  final String description;
  final bool supportsTools;
  final bool supportsVision;
  final bool supportsCode;
  final bool isEmbedding;
  final int downloads;
  final int likes;
  final String? lastModified;
  final List<String> tags;
  final int? approxBytes;
  final bool isCurated;

  const DiscoverItem({
    required this.repoId,
    required this.author,
    this.brandAuthor,
    required this.displayName,
    required this.description,
    required this.supportsTools,
    required this.supportsVision,
    required this.supportsCode,
    required this.isEmbedding,
    required this.downloads,
    required this.likes,
    this.lastModified,
    required this.tags,
    this.approxBytes,
    required this.isCurated,
  });

  /// The author slug used to look up the avatar on HuggingFace.
  String get avatarAuthor => brandAuthor ?? author;

  factory DiscoverItem.fromCurated(CuratedModel m) => DiscoverItem(
        repoId: m.repoId,
        author: m.repoId.contains('/') ? m.repoId.split('/').first : m.repoId,
        brandAuthor: m.brandAuthor,
        displayName: m.name,
        description: m.desc,
        supportsTools: m.supportsTools,
        supportsVision: m.kind == ModelKind.vision,
        supportsCode: false,
        isEmbedding: m.kind == ModelKind.memory,
        downloads: 0,
        likes: 0,
        tags: const [],
        approxBytes: m.approxBytes,
        isCurated: true,
      );

  factory DiscoverItem.fromHF(HFModelResult r) {
    final lowerTags = r.tags.map((t) => t.toLowerCase()).toSet();
    final isEmbed = lowerTags.intersection({
      'feature-extraction',
      'sentence-similarity',
      'sentence-transformers',
      'text-embeddings-inference',
    }).isNotEmpty;
    return DiscoverItem(
      repoId: r.id,
      author: r.author,
      displayName: humanizeName(r.id),
      description: '',
      supportsTools: r.supportsTools,
      supportsVision: r.supportsVision,
      supportsCode: r.supportsCode,
      isEmbedding: isEmbed,
      downloads: r.downloads,
      likes: r.likes,
      lastModified: r.lastModified,
      tags: r.tags,
      isCurated: false,
    );
  }

  String? get paramCount => _extractParams(displayName.isNotEmpty ? displayName : repoId);
  String? get arch => _detectArch(tags);

  /// True if this model likely supports tool/function calling.
  /// HF search tags rarely include 'function-calling' on GGUF repos, so we
  /// fall back to checking the model name against known tool-capable families.
  bool get likelySupportsTools {
    if (supportsTools) return true;
    if (isEmbedding) return false;
    final lower = '${displayName.toLowerCase()} ${repoId.toLowerCase()}';
    const families = [
      'llama-3', 'llama3', 'llama 3',
      'qwen2', 'qwen 2', 'qwen2.5', 'qwen3', 'qwen 3',
      'mistral', 'mixtral',
      'hermes', 'functionary', 'nexusraven', 'gorilla',
      'phi-3', 'phi-4', 'phi3', 'phi4', 'phi 3', 'phi 4',
      'gemma-2', 'gemma2', 'gemma 2', 'gemma-3', 'gemma3', 'gemma 3',
      'command-r', 'deepseek', 'internlm',
      'smollm', 'dolphin', 'openhermes',
    ];
    final hasFamily = families.any((f) => lower.contains(f));
    final isInstruct =
        lower.contains('instruct') || lower.contains('chat') || lower.contains('-it');
    return hasFamily && isInstruct;
  }

  /// True if this model likely supports vision/image input.
  bool get likelySupportsVision {
    if (supportsVision) return true;
    final lower = '${displayName.toLowerCase()} ${repoId.toLowerCase()}';
    const families = [
      'llava', 'bakllava', 'vision', 'moondream', 'minicpm-v',
      'internvl', 'cogvlm', 'qwen-vl', 'qwenvl', 'idefics',
      'pixtral', 'paligemma', 'florence',
    ];
    return families.any((f) => lower.contains(f));
  }

  /// True if this model is primarily a code model.
  bool get likelySupportsCode {
    if (supportsCode) return true;
    final lower = '${displayName.toLowerCase()} ${repoId.toLowerCase()}';
    const families = [
      'codellama', 'codegemma', 'deepseek-coder', 'starcoder',
      'wizardcoder', 'phind-codellama', 'codestral', 'qwen2.5-coder',
      'granite-code', 'opencoder',
    ];
    return families.any((f) => lower.contains(f));
  }
}

// ─── Helpers ─────────────────────────────────────────────────────
//
// humanizeName/timeAgo/fmtCount are public: both discover_tab.dart and
// model_detail_panel.dart use them for display formatting, not just this
// file's own DiscoverItem construction.

String humanizeName(String repoId) {
  final name = repoId.contains('/') ? repoId.split('/').last : repoId;
  return name
      .replaceAll(RegExp(r'[-_]gguf', caseSensitive: false), '')
      .replaceAll(RegExp(r'[-_]'), ' ')
      .trim();
}

String? _extractParams(String text) {
  final m = RegExp(r'(\d+(?:\.\d+)?)\s*[Bb](?:\b|$)', caseSensitive: false)
      .firstMatch(text);
  if (m == null) return null;
  final val = double.tryParse(m.group(1)!);
  if (val == null || val > 2000 || val < 0.1) return null;
  if (val == val.roundToDouble()) return '${val.round()}B';
  return '${val}B';
}

String? _detectArch(List<String> tags) {
  const knownArchs = {
    'gemma', 'gemma2', 'gemma3', 'llama', 'mistral', 'qwen2', 'qwen3',
    'phi', 'phi3', 'phi4', 'falcon', 'gpt2', 'mpt', 'bloom', 'internlm2',
    'deepseek', 'deepseek2', 'cohere',
  };
  for (final t in tags) {
    if (knownArchs.contains(t.toLowerCase())) return t.toLowerCase();
  }
  return null;
}

String timeAgo(String? iso) {
  if (iso == null || iso.isEmpty) return '';
  final dt = DateTime.tryParse(iso);
  if (dt == null) return '';
  final diff = DateTime.now().difference(dt);
  if (diff.inDays > 365) return '${(diff.inDays / 365).round()}y ago';
  if (diff.inDays > 30) return '${(diff.inDays / 30).round()}mo ago';
  if (diff.inDays > 0) return '${diff.inDays}d ago';
  if (diff.inHours > 0) return '${diff.inHours}h ago';
  return 'just now';
}

String fmtCount(int n) {
  if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
  if (n >= 1000) return '${(n / 1000).toStringAsFixed(0)}K';
  return '$n';
}
