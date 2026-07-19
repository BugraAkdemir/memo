import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/curated_models.dart';
import '../../models/gpu_info.dart';
import '../../models/local_model.dart';
import '../../providers/chat_provider.dart';
import '../../providers/models_provider.dart';
import '../../widgets/model_config_dialog.dart';

import 'discover_item.dart';

// ─── Right panel: model detail ────────────────────────────────────

class ModelDetailPanel extends ConsumerStatefulWidget {
  final DiscoverItem item;
  final void Function(String repoId, String displayName)? onSelectOther;

  const ModelDetailPanel({
    super.key,
    required this.item,
    this.onSelectOther,
  });

  @override
  ConsumerState<ModelDetailPanel> createState() => _ModelDetailPanelState();
}

class _ModelDetailPanelState extends ConsumerState<ModelDetailPanel> {
  // Files / selection
  List<GGUFFile>? _files;
  String? _filesError;
  GGUFFile? _selectedFile;

  // Dropdown state
  bool _dropdownOpen = false;

  // README
  String? _readme;
  bool _readmeLoading = false;
  String? _readmeError;

  // More from author
  List<Map<String, dynamic>>? _moreModels;

  /// The model's *actual* brand/creator (e.g. "google" for a
  /// bartowski-quantized Gemma repo), resolved from HF's own
  /// cardData.base_model — not derived from the repo owner, which for a
  /// GGUF conversion is almost always a third-party quantizer
  /// (bartowski/unsloth/TheBloke/...), not the original org. Falls back to
  /// item.avatarAuthor (the repo's own owner) at every render site when
  /// this stays null — an official first-party repo (uploader IS the
  /// brand) or a repo with no declared base_model both look identical
  /// either way.
  String? _brandAuthor;

  Future<void> _loadBrandAuthor() async {
    try {
      final resp = await ref.read(apiClientProvider).dio.get<Map<String, dynamic>>(
        'https://huggingface.co/api/models/${widget.item.repoId}',
        options: Options(
          receiveTimeout: const Duration(seconds: 6),
          sendTimeout: const Duration(seconds: 6),
        ),
      );
      final cardData = resp.data?['cardData'] as Map<String, dynamic>?;
      final rawBaseModel = cardData?['base_model'];
      // HF model cards declare base_model either as a single string or, for
      // merges, a list of strings — take the first usable one either way.
      final baseModel = rawBaseModel is String
          ? rawBaseModel
          : (rawBaseModel is List && rawBaseModel.isNotEmpty)
              ? rawBaseModel.first as String?
              : null;
      if (baseModel == null || !baseModel.contains('/')) return;
      final brand = baseModel.split('/').first;
      if (mounted && brand.isNotEmpty) {
        setState(() => _brandAuthor = brand);
      }
    } catch (_) {
      // No cardData.base_model, no network, whatever — just fall back to
      // the repo's own owner everywhere this is used.
    }
  }

  /// Real, live-fetched tools signal: true once `_loadToolsSupport` has
  /// confirmed the repo's own tokenizer_config.json chat_template actually
  /// references tool_calls. Stays null (never explicitly false) on a 404
  /// or any other fetch failure — many GGUF-only repos (quantizers like
  /// bartowski/unsloth typically upload only .gguf files, not the source
  /// model's tokenizer_config.json) simply don't have this file, which is
  /// not the same as "confirmed not to support tools." A null here just
  /// means "no live answer," so the render sites fall back to
  /// item.supportsTools (the HF tag) exactly as before this existed.
  bool? _liveToolsSupport;

  Future<void> _loadToolsSupport() async {
    try {
      final resp = await ref.read(apiClientProvider).dio.get<String>(
        'https://huggingface.co/${widget.item.repoId}/raw/main/tokenizer_config.json',
        options: Options(responseType: ResponseType.plain),
      );
      // A plain substring scan (not JSON-shape-specific parsing) on
      // purpose: HuggingFace tokenizers store chat_template either as a
      // single string or, on newer conversions, as a list of named
      // templates ([{"name": ..., "template": ...}, ...]) — either way,
      // "tool_calls" only legitimately shows up here inside a template
      // that actually renders tool calls, so this mirrors the same check
      // internal/gguf.Read already does server-side on the downloaded
      // file's own tokenizer.chat_template.
      final body = resp.data ?? '';
      if (mounted && body.toLowerCase().contains('tool_calls')) {
        setState(() => _liveToolsSupport = true);
      }
    } catch (_) {
      // 404 (most common — file just doesn't exist in this repo) or any
      // other fetch failure: leave _liveToolsSupport null, fall back to
      // the HF tag. Not worth surfacing as an error — this is a best-
      // effort enrichment, not a feature the user directly asked to run.
    }
  }

  /// Real, file-tree-derived vision signal (BUG fix: this used to be a
  /// hardcoded model-family name list, e.g. 'llava'/'moondream'/'qwen-vl',
  /// guessed from the display name). Once the repo's actual file list is
  /// fetched (`_loadFiles`, already needed to let the user pick a quant),
  /// the presence of a real "mmproj" file in that same repo is the same
  /// reliable per-file signal `findMmproj` already uses for already-
  /// downloaded models — just checked against the HF file tree instead of
  /// the local disk, since nothing is downloaded yet at this point.
  bool get _hasMmprojInRepo =>
      (_files ?? const []).any((f) => f.filename.toLowerCase().contains('mmproj'));

  @override
  void initState() {
    super.initState();
    _loadFiles();
    _loadReadme();
    _loadMoreModels();
    _loadBrandAuthor();
    // Only worth a live fetch when the HF tag hasn't already confirmed
    // tools support (nothing to add) and this isn't an embedding model
    // (never tool-capable).
    if (!widget.item.supportsTools && !widget.item.isEmbedding) {
      _loadToolsSupport();
    }
  }

  Future<void> _loadFiles() async {
    try {
      final files =
          await ref.read(apiClientProvider).getModelFiles(widget.item.repoId);
      files.sort((a, b) =>
          quantInfo(b.filename).priority.compareTo(quantInfo(a.filename).priority));
      if (mounted) {
        setState(() {
          _files = files;
          _selectedFile = pickRecommendedFile(files);
        });
      }
    } catch (e) {
      if (mounted) setState(() => _filesError = e.toString());
    }
  }

  Future<void> _loadReadme() async {
    if (mounted) setState(() { _readmeLoading = true; _readmeError = null; });
    try {
      final url =
          'https://huggingface.co/${widget.item.repoId}/raw/main/README.md';
      final resp = await ref.read(apiClientProvider).dio.get<String>(
        url,
        options: Options(responseType: ResponseType.plain),
      );
      if (mounted) {
        setState(() {
          _readme = _stripFrontmatter(resp.data ?? '');
          _readmeLoading = false;
        });
      }
    } on DioException catch (e) {
      // A 404 just means this repo has no README.md — a normal, common case,
      // not worth surfacing as an error. Anything else (timeout, DNS
      // failure, 5xx) is a real fetch failure the user should be able to
      // see instead of the README section just silently not appearing as
      // if the model had none.
      final is404 = e.response?.statusCode == 404;
      if (mounted) {
        setState(() {
          _readmeLoading = false;
          if (!is404) _readmeError = e.message ?? e.toString();
        });
      }
    } catch (e) {
      if (mounted) setState(() { _readmeLoading = false; _readmeError = e.toString(); });
    }
  }

  String _stripFrontmatter(String content) {
    if (!content.startsWith('---\n')) return content;
    final end = content.indexOf('\n---\n', 4);
    if (end == -1) return content;
    return content.substring(end + 5);
  }

  Future<void> _loadMoreModels() async {
    try {
      final author = widget.item.author;
      if (author.isEmpty) return;
      final url =
          'https://huggingface.co/api/models?author=$author&filter=gguf&sort=downloads&direction=-1&limit=6';
      final resp = await ref.read(apiClientProvider).dio.get<dynamic>(url);
      final data = resp.data;
      if (data is List && mounted) {
        final results = data
            .whereType<Map<String, dynamic>>()
            .where((m) => (m['id'] as String? ?? '') != widget.item.repoId)
            .take(5)
            .map((m) => {
                  'id': m['id'] as String? ?? '',
                  'downloads': m['downloads'] as int? ?? 0,
                })
            .toList();
        setState(() => _moreModels = results);
      }
    } catch (_) {
      // silently ignore
    }
  }

  Future<void> _download() async {
    final file = _selectedFile;
    if (file == null) return;
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref
          .read(apiClientProvider)
          .downloadModel(widget.item.repoId, file.filename,
              expectedSize: file.size);
      if (mounted) {
        messenger.showSnackBar(SnackBar(
          content: Text(L10n.t('download_started', {'file': file.filename})),
          backgroundColor: MemoTheme.green,
        ));
      }
    } catch (e) {
      if (mounted) {
        messenger.showSnackBar(SnackBar(
            content: Text(L10n.t('import_error', {'e': e.toString()}))));
      }
    }
  }

  /// Extracts a short quant code from the filename (e.g. "Q4_K_M").
  String _quantCode(String filename) {
    final m = RegExp(r'[Qq]\d[^.]*').firstMatch(filename);
    if (m != null) return m.group(0)!.toUpperCase();
    if (filename.toLowerCase().contains('fp16') ||
        filename.toLowerCase().contains('f16')) {
      return 'FP16';
    }
    return 'GGUF';
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final gpu = ref.watch(gpuInfoProvider).valueOrNull ?? const GPUInfo();
    final localModels = ref.watch(localModelsProvider).valueOrNull ?? [];
    final downloads = ref.watch(downloadProgressProvider).valueOrNull ?? [];
    final installed = ref.watch(llamaInstalledProvider).valueOrNull ?? false;
    final item = widget.item;
    // Only this repo+file counts — other files can download concurrently
    // (e.g. the setup wizard's chat + memory model) without flipping this
    // panel's button to "Cancel" for an unrelated download.
    final downloadingNow = _selectedFile != null &&
        downloads.any((p) =>
            p.active && p.repoId == item.repoId && p.filename == _selectedFile!.filename);

    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(28, 24, 28, 32),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // ── Header: Logo + Repo ID + copy ──
          Row(
            children: [
              AuthorAvatar(
                author: _brandAuthor ?? item.avatarAuthor,
                dio: ref.read(apiClientProvider).dio,
                size: 40,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  item.repoId,
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w700,
                    color: c.textMain,
                  ),
                ),
              ),
              IconButton(
                icon: Icon(Icons.copy_outlined, size: 15, color: c.textDim),
                tooltip: 'Copy repo ID',
                onPressed: () =>
                    Clipboard.setData(ClipboardData(text: item.repoId)),
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
              ),
            ],
          ),
          const SizedBox(height: 6),

          // ── Stats row ──
          Wrap(
            spacing: 12,
            children: [
              if (item.downloads > 0)
                _StatChip(
                    icon: Icons.download_rounded,
                    text: fmtCount(item.downloads)),
              if (item.likes > 0)
                _StatChip(
                    icon: Icons.star_border_rounded, text: '${item.likes}'),
              if (timeAgo(item.lastModified).isNotEmpty)
                _StatChip(
                    icon: Icons.schedule_outlined,
                    text: timeAgo(item.lastModified)),
            ],
          ),

          // ── Description ──
          if (item.description.isNotEmpty) ...[
            const SizedBox(height: 14),
            Container(
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(
                color: c.bgPanel,
                borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                border: Border.all(color: c.borderSoft),
              ),
              child: Text(
                item.description,
                style:
                    TextStyle(fontSize: 13, color: c.textMain, height: 1.5),
              ),
            ),
          ],

          const SizedBox(height: 14),

          // ── Metadata tags ──
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              if (item.paramCount != null)
                _InfoTag(
                    label:
                        L10n.t('params'),
                    value: item.paramCount!),
              if (item.arch != null)
                _InfoTag(
                    label: L10n.t('arch'),
                    value: item.arch!),
              _InfoTag(
                  label: L10n.t('format'),
                  value: 'GGUF',
                  accent: true),
              if (item.isEmbedding)
                _InfoTag(
                    label: L10n.t('domain'),
                    value: 'Embedding'),
            ],
          ),

          // ── Capabilities ──
          if (item.supportsTools ||
              (_liveToolsSupport ?? false) ||
              item.supportsVision ||
              _hasMmprojInRepo ||
              item.supportsCode) ...[
            const SizedBox(height: 12),
            Row(
              children: [
                Text(
                  L10n.t('capabilities'),
                  style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: c.textMuted),
                ),
                const SizedBox(width: 4),
                Flexible(
                  child: Wrap(
                    spacing: 8,
                    runSpacing: 6,
                    children: [
                      if (item.supportsVision || _hasMmprojInRepo)
                        _CapabilityPill(
                          icon: Icons.visibility_outlined,
                          label: L10n.t('vision_2'),
                          color: const Color(0xFF50C878),
                        ),
                      if (item.supportsTools || (_liveToolsSupport ?? false))
                        _CapabilityPill(
                          icon: Icons.build_outlined,
                          label: L10n.t('tool_use'),
                          color: MemoTheme.accent,
                        ),
                      if (item.supportsCode)
                        _CapabilityPill(
                          icon: Icons.code,
                          label:
                              L10n.t('code_2'),
                          color: const Color(0xFF7C6FEE),
                        ),
                    ],
                  ),
                ),
              ],
            ),
          ],

          const SizedBox(height: 20),

          // ── Download Options section header ──
          Text(
            L10n.t('download_options'),
            style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w700,
                color: c.textMuted,
                letterSpacing: 0.4),
          ),
          const SizedBox(height: 10),

          // ── Download Options body ──
          if (_filesError != null)
            Text('${L10n.t('error')}: $_filesError',
                style: const TextStyle(color: MemoTheme.red, fontSize: 13))
          else if (_files == null)
            const Center(
              child: Padding(
                padding: EdgeInsets.symmetric(vertical: 12),
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            )
          else if (_files!.isEmpty)
            Text(L10n.t('no_files_found'),
                style: TextStyle(color: c.textDim, fontSize: 13))
          else ...[
            // ── Collapsible dropdown ──
            _buildDropdown(c, gpu, localModels),
            const SizedBox(height: 10),

            // ── Download button + hardware fit chip ──
            if (_selectedFile != null)
              _buildDownloadRow(c, gpu, localModels, downloadingNow, installed),
          ],

          // ── README section ──
          if (_readmeLoading || _readmeError != null || (_readme != null && _readme!.isNotEmpty)) ...[
            const SizedBox(height: 28),
            Row(
              children: [
                Text(
                  'README',
                  style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w700,
                      color: c.textMuted,
                      letterSpacing: 0.4),
                ),
                const SizedBox(width: 8),
                if (_readmeLoading)
                  const SizedBox(
                    width: 12,
                    height: 12,
                    child: CircularProgressIndicator(strokeWidth: 1.5),
                  ),
              ],
            ),
            if (_readmeError != null) ...[
              const SizedBox(height: 8),
              Text('${L10n.t('error')}: $_readmeError',
                  style: const TextStyle(color: MemoTheme.red, fontSize: 13)),
            ],
            if (!_readmeLoading && _readme != null && _readme!.isNotEmpty) ...[
              const SizedBox(height: 10),
              Container(
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: c.bgPanel,
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  border: Border.all(color: c.borderSoft),
                ),
                child: MarkdownBody(
                  data: _readme!,
                  shrinkWrap: true,
                  styleSheet: MarkdownStyleSheet.fromTheme(
                    Theme.of(context),
                  ).copyWith(
                    p: TextStyle(
                        fontSize: 13, color: c.textMain, height: 1.6),
                    code: TextStyle(
                        fontSize: 12,
                        color: MemoTheme.accent,
                        backgroundColor:
                            MemoTheme.accent.withValues(alpha: 0.08)),
                    h1: TextStyle(
                        fontSize: 17,
                        fontWeight: FontWeight.w700,
                        color: c.textMain),
                    h2: TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w700,
                        color: c.textMain),
                    h3: TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w700,
                        color: c.textMain),
                  ),
                ),
              ),
            ],
          ],

          // ── More from [author] ──
          if (_moreModels != null && _moreModels!.isNotEmpty) ...[
            const SizedBox(height: 28),
            Text(
              L10n.t('more_from_author', {'author': widget.item.author}),
              style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                  color: c.textMuted,
                  letterSpacing: 0.4),
            ),
            const SizedBox(height: 8),
            Container(
              decoration: BoxDecoration(
                color: c.bgPanel,
                borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                border: Border.all(color: c.borderSoft),
              ),
              child: Column(
                children: [
                  for (int i = 0; i < _moreModels!.length; i++) ...[
                    if (i > 0)
                      Divider(height: 1, thickness: 1, color: c.borderSoft),
                    _MoreModelRow(
                      id: _moreModels![i]['id'] as String? ?? '',
                      downloads: (_moreModels![i]['downloads'] as int?) ?? 0,
                      onTap: () {
                        final id = _moreModels![i]['id'] as String?;
                        if (id == null) return;
                        widget.onSelectOther
                            ?.call(id, humanizeName(id));
                      },
                    ),
                  ],
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildDropdown(
      ThemeColors c, GPUInfo gpu, List<LocalModel> localModels) {
    final files = _files!;
    final selected = _selectedFile;

    // Hardware fit badge widget
    Widget fitBadge(GGUFFile file) {
      final fit = hardwareFit(file.size, gpu);
      final (color, icon, label) = switch (fit.level) {
        FitLevel.good => (
            MemoTheme.green,
            Icons.check_circle_outline,
            L10n.t('fits'),
          ),
        FitLevel.ok => (
            MemoTheme.accent,
            Icons.check_circle_outline,
            L10n.t('cpu_ok'),
          ),
        FitLevel.warn => (
            MemoTheme.warningOrange,
            Icons.warning_amber_rounded,
            L10n.t('too_large'),
          ),
      };
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.12),
          borderRadius: BorderRadius.circular(5),
          border: Border.all(color: color.withValues(alpha: 0.35)),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 11, color: color),
            const SizedBox(width: 4),
            Text(label,
                style: TextStyle(
                    fontSize: 11, fontWeight: FontWeight.w600, color: color)),
          ],
        ),
      );
    }

    // Collapsed header row
    Widget headerRow(GGUFFile? file) {
      if (file == null) {
        return Text(
          L10n.t('select_a_file'),
          style: TextStyle(fontSize: 13, color: c.textDim),
        );
      }
      final q = quantInfo(file.filename);
      final qCode = _quantCode(file.filename);
      return Row(
        children: [
          _GgufBadge(),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              q.label, // user-friendly: "Dengeli kalite", "Yüksek kalite"...
              style: TextStyle(
                  fontSize: 13, fontWeight: FontWeight.w500, color: c.textMain),
              overflow: TextOverflow.ellipsis,
              maxLines: 1,
            ),
          ),
          const SizedBox(width: 6),
          Text(qCode,
              style: TextStyle(
                  fontSize: 11,
                  fontWeight: FontWeight.w600,
                  color: c.textDim,
                  fontFamily: 'monospace')),
          const SizedBox(width: 10),
          fitBadge(file),
          const SizedBox(width: 10),
          Text(file.sizeFormatted,
              style: TextStyle(fontSize: 12, color: c.textMuted)),
          const SizedBox(width: 6),
          Icon(
            _dropdownOpen ? Icons.keyboard_arrow_up : Icons.keyboard_arrow_down,
            size: 18,
            color: c.textDim,
          ),
        ],
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Collapsed trigger
        GestureDetector(
          onTap: () => setState(() => _dropdownOpen = !_dropdownOpen),
          child: MouseRegion(
            cursor: SystemMouseCursors.click,
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 150),
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
              decoration: BoxDecoration(
                color: c.bgPanel,
                borderRadius: _dropdownOpen
                    ? const BorderRadius.vertical(top: Radius.circular(8))
                    : BorderRadius.circular(8),
                border: Border.all(
                  color: _dropdownOpen
                      ? MemoTheme.accent.withValues(alpha: 0.5)
                      : c.borderSoft,
                ),
              ),
              child: headerRow(selected),
            ),
          ),
        ),

        // Expanded list
        if (_dropdownOpen)
          Container(
            decoration: BoxDecoration(
              color: c.bgPanel,
              borderRadius:
                  const BorderRadius.vertical(bottom: Radius.circular(8)),
              border: Border(
                left: BorderSide(color: MemoTheme.accent.withValues(alpha: 0.4)),
                right: BorderSide(color: MemoTheme.accent.withValues(alpha: 0.4)),
                bottom: BorderSide(color: MemoTheme.accent.withValues(alpha: 0.4)),
              ),
            ),
            child: Column(
              children: files.map((file) {
                final isSelected = selected?.filename == file.filename;
                final isDownloaded =
                    localModels.any((m) => m.filename == file.filename);
                final q = quantInfo(file.filename);
                final qCode = _quantCode(file.filename);
                return MouseRegion(
                  cursor: SystemMouseCursors.click,
                  child: GestureDetector(
                    onTap: () => setState(() {
                      _selectedFile = file;
                      _dropdownOpen = false;
                    }),
                    child: AnimatedContainer(
                      duration: const Duration(milliseconds: 100),
                      padding: const EdgeInsets.symmetric(
                          horizontal: 14, vertical: 11),
                      decoration: BoxDecoration(
                        color: isSelected
                            ? MemoTheme.accent.withValues(alpha: 0.08)
                            : Colors.transparent,
                        border:
                            Border(top: BorderSide(color: c.borderSoft)),
                      ),
                      child: Row(
                        children: [
                          // checkmark / placeholder
                          SizedBox(
                            width: 18,
                            child: isSelected
                                ? Icon(Icons.check,
                                    size: 14, color: MemoTheme.accent)
                                : null,
                          ),
                          const SizedBox(width: 4),
                          _GgufBadge(),
                          const SizedBox(width: 8),
                          // Friendly quant label
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  q.label,
                                  style: TextStyle(
                                    fontSize: 13,
                                    fontWeight: isSelected
                                        ? FontWeight.w600
                                        : FontWeight.w500,
                                    color: isSelected
                                        ? MemoTheme.accent
                                        : c.textMain,
                                  ),
                                ),
                                Text(
                                  qCode,
                                  style: TextStyle(
                                      fontSize: 10,
                                      color: c.textDim,
                                      fontFamily: 'monospace'),
                                ),
                              ],
                            ),
                          ),
                          const SizedBox(width: 8),
                          // Hardware fit badge
                          fitBadge(file),
                          const SizedBox(width: 10),
                          // Size
                          Text(file.sizeFormatted,
                              style: TextStyle(
                                  fontSize: 12, color: c.textMuted)),
                          if (isDownloaded) ...[
                            const SizedBox(width: 8),
                            Icon(Icons.check_circle,
                                size: 14, color: MemoTheme.green),
                          ],
                        ],
                      ),
                    ),
                  ),
                );
              }).toList(),
            ),
          ),
      ],
    );
  }

  Widget _buildDownloadRow(ThemeColors c, GPUInfo gpu,
      List<LocalModel> localModels, bool downloadingNow, bool installed) {
    final file = _selectedFile!;
    final fit = hardwareFit(file.size, gpu);
    final isFileDownloaded =
        localModels.any((m) => m.filename == file.filename);

    final (fitColor, _) = switch (fit.level) {
      FitLevel.good => (MemoTheme.green, Icons.check_circle_outline),
      FitLevel.ok => (MemoTheme.accent, Icons.check_circle_outline),
      FitLevel.warn => (MemoTheme.warningOrange, Icons.warning_amber_rounded),
    };

    Widget button;
    if (isFileDownloaded) {
      final localModel =
          localModels.firstWhere((m) => m.filename == file.filename);
      button = Expanded(
        child: ElevatedButton.icon(
          onPressed: !installed
              ? null
              : () => showDialog(
                    context: context,
                    builder: (_) => ModelConfigDialog(model: localModel),
                  ),
          icon: const Icon(Icons.play_arrow_rounded, size: 18),
          label: Text(!installed
              ? (L10n.t('install_llama_cpp_first'))
              : (L10n.t('start_sizeformatted', {'sizeFormatted': file.sizeFormatted}))),
          style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: 14)),
        ),
      );
    } else if (downloadingNow) {
      button = Expanded(
        child: ElevatedButton.icon(
          onPressed: () => ref
              .read(apiClientProvider)
              .cancelDownload(widget.item.repoId, file.filename),
          icon: const Icon(Icons.stop_rounded, size: 18),
          label: Text(
            L10n.t('cancel_2'),
          ),
          style: ElevatedButton.styleFrom(
            padding: const EdgeInsets.symmetric(vertical: 14),
            backgroundColor: MemoTheme.red.withValues(alpha: 0.12),
            foregroundColor: MemoTheme.red,
          ),
        ),
      );
    } else {
      button = Expanded(
        child: ElevatedButton.icon(
          onPressed: _download,
          icon: const Icon(Icons.download_rounded, size: 18),
          label: Text('${L10n.t('download')} · ${file.sizeFormatted}'),
          style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: 14)),
        ),
      );
    }

    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        button,
        const SizedBox(width: 10),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 6),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(6),
            border: Border.all(color: fitColor.withValues(alpha: 0.5)),
          ),
          child: Text(
            fit.label,
            style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: fitColor),
          ),
        ),
      ],
    );
  }
}

// ─── GGUF badge (reused across dropdown header + rows) ───────────

class _GgufBadge extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: MemoTheme.accent.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: MemoTheme.accent.withValues(alpha: 0.3)),
      ),
      child: Text(
        'GGUF',
        style: TextStyle(
            fontSize: 10, fontWeight: FontWeight.w700, color: MemoTheme.accent),
      ),
    );
  }
}

// ─── More-from-author compact row ─────────────────────────────────

class _MoreModelRow extends StatelessWidget {
  final String id;
  final int downloads;
  final VoidCallback onTap;

  const _MoreModelRow({
    required this.id,
    required this.downloads,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 11),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  humanizeName(id),
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w500,
                    color: c.textMain,
                  ),
                  overflow: TextOverflow.ellipsis,
                  maxLines: 1,
                ),
              ),
              const SizedBox(width: 8),
              Text(
                '↓ ${fmtCount(downloads)}',
                style: TextStyle(fontSize: 12, color: c.textDim),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _StatChip extends StatelessWidget {
  final IconData icon;
  final String text;
  const _StatChip({required this.icon, required this.text});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 13, color: c.textDim),
        const SizedBox(width: 4),
        Text(text, style: TextStyle(fontSize: 12, color: c.textDim)),
      ],
    );
  }
}

class _InfoTag extends StatelessWidget {
  final String label;
  final String value;
  final bool accent;
  const _InfoTag(
      {required this.label, required this.value, this.accent = false});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final tagColor = accent ? MemoTheme.accent : c.textMuted;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          '$label ',
          style: TextStyle(fontSize: 12, color: c.textDim),
        ),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
          decoration: BoxDecoration(
            color: tagColor.withValues(alpha: accent ? 0.15 : 0.08),
            borderRadius: BorderRadius.circular(5),
            border: accent
                ? Border.all(color: tagColor.withValues(alpha: 0.4))
                : null,
          ),
          child: Text(
            value,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: tagColor,
            ),
          ),
        ),
      ],
    );
  }
}

class _CapabilityPill extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color color;
  const _CapabilityPill(
      {required this.icon, required this.label, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(6),
        border: Border.all(color: color.withValues(alpha: 0.35)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 13, color: color),
          const SizedBox(width: 5),
          Text(
            label,
            style: TextStyle(
                fontSize: 12, fontWeight: FontWeight.w600, color: color),
          ),
        ],
      ),
    );
  }
}
