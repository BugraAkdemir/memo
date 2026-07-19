import 'package:dio/dio.dart';
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

// ─── Author avatar (HF logo with letter fallback) ────────────────
//
// Shared between DiscoverTab's list rows and ModelDetailPanel's header —
// moved here (was private to discover_tab.dart) so both can use the exact
// same fetch/cache/fallback logic instead of drifting into two copies.

Color _authorColor(String author) {
  const colors = [
    Color(0xFF4B7BEC),
    Color(0xFF7C6FEE),
    Color(0xFF26C6DA),
    Color(0xFF50C878),
    Color(0xFFFF7043),
    Color(0xFFE91E8C),
    Color(0xFFF9CA24),
  ];
  if (author.isEmpty) return colors[0];
  final idx = author.codeUnits.fold(0, (s, c) => s + c) % colors.length;
  return colors[idx];
}

class AuthorAvatar extends StatefulWidget {
  final String author;
  final Dio dio;
  final double size;
  const AuthorAvatar({super.key, required this.author, required this.dio, this.size = 34});

  @override
  State<AuthorAvatar> createState() => _AuthorAvatarState();
}

class _AuthorAvatarState extends State<AuthorAvatar> {
  // Shared across all instances — one fetch per unique author per session.
  // Capped so browsing thousands of distinct authors across a long-running
  // session doesn't grow this forever; a clear-on-overflow is simplest and
  // just costs a few extra re-fetches right after, not a correctness issue.
  static final _cache = <String, String?>{};
  static const _cacheCap = 500;
  String? _avatarUrl;
  bool _resolved = false;

  @override
  void initState() {
    super.initState();
    _resolve();
  }

  @override
  void didUpdateWidget(AuthorAvatar old) {
    super.didUpdateWidget(old);
    if (old.author != widget.author) _resolve();
  }

  Future<void> _resolve() async {
    final a = widget.author;
    if (_cache.containsKey(a)) {
      if (mounted) setState(() { _avatarUrl = _cache[a]; _resolved = true; });
      return;
    }
    String? url;
    for (final endpoint in [
      'https://huggingface.co/api/organizations/$a',
      'https://huggingface.co/api/users/$a',
    ]) {
      try {
        final r = await widget.dio.get<Map<String, dynamic>>(
          endpoint,
          options: Options(
            receiveTimeout: const Duration(seconds: 6),
            sendTimeout: const Duration(seconds: 6),
          ),
        );
        final u = r.data?['avatarUrl'] as String?;
        if (u != null && u.isNotEmpty) { url = u; break; }
      } catch (_) {}
    }
    if (_cache.length >= _cacheCap) _cache.clear();
    _cache[a] = url;
    if (mounted) setState(() { _avatarUrl = url; _resolved = true; });
  }

  @override
  Widget build(BuildContext context) {
    if (_resolved && _avatarUrl != null) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(8),
        child: Image.network(
          _avatarUrl!,
          width: widget.size,
          height: widget.size,
          fit: BoxFit.cover,
          errorBuilder: (context, error, stackTrace) =>
              LetterAvatar(author: widget.author, size: widget.size),
        ),
      );
    }
    return LetterAvatar(author: widget.author, size: widget.size);
  }
}

class LetterAvatar extends StatelessWidget {
  final String author;
  final double size;
  const LetterAvatar({super.key, required this.author, this.size = 34});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: _authorColor(author),
        borderRadius: BorderRadius.circular(8),
      ),
      alignment: Alignment.center,
      child: Text(
        author.isNotEmpty ? author[0].toUpperCase() : '?',
        style: TextStyle(
          color: Colors.white,
          fontSize: size * 0.41,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }
}
