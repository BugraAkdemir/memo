import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/tts_voice.dart';
import '../../providers/chat_provider.dart';

/// Settings → Beta Features → Local Voice Models (Faz 2.6).
///
/// The fully-offline counterpart to [TTSProviderSection] (external API
/// providers, in the sibling file) — no API key, ever. Downloads a curated
/// Piper voice once from Hugging Face, then SynthesizeSpeech
/// (internal/app/tts.go) runs entirely on-device via the local Piper
/// engine, same as it always has (Faz 1) — this section just automates
/// finding a working voice file instead of the user hand-placing one and
/// editing config.yaml themselves.
class TTSVoiceSection extends ConsumerStatefulWidget {
  const TTSVoiceSection({super.key});

  @override
  ConsumerState<TTSVoiceSection> createState() => _TTSVoiceSectionState();
}

class _TTSVoiceSectionState extends ConsumerState<TTSVoiceSection> {
  TTSVoiceStoreState? _state;
  bool _loading = true;
  String? _loadError;
  final Set<String> _busyIds = {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _loadError = null;
    });
    try {
      final state = await ref.read(apiClientProvider).getTTSVoices();
      if (!mounted) return;
      setState(() {
        _state = state;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loadError = L10n.t('tts_voice_load_failed', {'err': '$e'});
        _loading = false;
      });
    }
  }

  Future<void> _download(TTSVoice v) async {
    setState(() => _busyIds.add(v.id));
    try {
      await ref.read(apiClientProvider).downloadTTSVoice(v.locale, v.name, v.quality);
      // Poll briefly for completion so the row updates without the user
      // having to reopen the tab — mirrors the model store's own
      // progress-polling pattern (engine_strip.dart), scaled down since
      // voice files are small (tens of MB, not gigabytes).
      for (var i = 0; i < 60; i++) {
        await Future.delayed(const Duration(seconds: 2));
        if (!mounted) return;
        final state = await ref.read(apiClientProvider).getTTSVoices();
        if (!mounted) return;
        setState(() => _state = state);
        final stillDownloading = state.downloads
            .any((d) => d.voiceId == v.id && d.active && d.error == null);
        if (!stillDownloading) break;
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('tts_voice_download_failed', {'err': '$e'}))),
      );
    } finally {
      if (mounted) setState(() => _busyIds.remove(v.id));
    }
  }

  Future<void> _select(TTSVoice v) async {
    setState(() => _busyIds.add(v.id));
    try {
      await ref.read(apiClientProvider).selectTTSVoice(v.id);
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('tts_voice_select_failed', {'err': '$e'}))),
      );
    } finally {
      if (mounted) setState(() => _busyIds.remove(v.id));
    }
  }

  Future<void> _delete(TTSVoice v) async {
    setState(() => _busyIds.add(v.id));
    try {
      await ref.read(apiClientProvider).deleteTTSVoice(v.id);
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('tts_voice_delete_failed', {'err': '$e'}))),
      );
    } finally {
      if (mounted) setState(() => _busyIds.remove(v.id));
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: theme.bgPanel,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            L10n.t('tts_voices_title'),
            style: TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: theme.textMain),
          ),
          const SizedBox(height: 4),
          Text(
            L10n.t('tts_voices_desc'),
            style: TextStyle(fontSize: 12, height: 1.4, color: theme.textDim),
          ),
          const SizedBox(height: 10),
          if (_loading)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 8),
              child: SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            )
          else if (_loadError != null)
            Text(_loadError!, style: TextStyle(fontSize: 12, color: MemoTheme.red))
          else
            ..._state!.catalog.map((v) {
              final localMatches = _state!.local.where((l) => l.id == v.id);
              final local = localMatches.isEmpty ? null : localMatches.first;
              final downloadMatches =
                  _state!.downloads.where((d) => d.voiceId == v.id && d.active);
              return _VoiceRow(
                voice: v,
                local: local,
                isSelected: local != null &&
                    _state!.selectedPath.isNotEmpty &&
                    local.path == _state!.selectedPath,
                downloadProgress: downloadMatches.isEmpty ? null : downloadMatches.first,
                busy: _busyIds.contains(v.id),
                onDownload: () => _download(v),
                onSelect: () => _select(v),
                onDelete: () => _delete(v),
              );
            }),
        ],
      ),
    );
  }
}

class _VoiceRow extends StatelessWidget {
  final TTSVoice voice;
  final TTSLocalVoice? local;
  final bool isSelected;
  final TTSVoiceDownloadProgress? downloadProgress;
  final bool busy;
  final VoidCallback onDownload;
  final VoidCallback onSelect;
  final VoidCallback onDelete;

  const _VoiceRow({
    required this.voice,
    required this.local,
    required this.isSelected,
    required this.downloadProgress,
    required this.busy,
    required this.onDownload,
    required this.onSelect,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    final downloading = downloadProgress != null;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '${TTSVoiceLanguageNames.of(voice.language)} · ${voice.name} (${voice.quality})',
                  style: TextStyle(fontSize: 13, color: theme.textMain, fontWeight: FontWeight.w600),
                ),
                if (downloading)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: LinearProgressIndicator(
                      value: downloadProgress!.percent > 0 ? downloadProgress!.percent / 100 : null,
                      minHeight: 3,
                    ),
                  ),
              ],
            ),
          ),
          if (downloading)
            Text(
              L10n.t('tts_voice_downloading', {'percent': downloadProgress!.percent.toStringAsFixed(0)}),
              style: TextStyle(fontSize: 11, color: theme.textDim),
            )
          else if (local == null)
            OutlinedButton(
              onPressed: busy ? null : onDownload,
              child: Text(L10n.t('tts_voice_download')),
            )
          else ...[
            if (isSelected)
              Text(
                L10n.t('tts_voice_selected'),
                style: TextStyle(fontSize: 11, color: MemoTheme.accent, fontWeight: FontWeight.w600),
              )
            else
              TextButton(
                onPressed: busy ? null : onSelect,
                child: Text(L10n.t('tts_voice_select')),
              ),
            IconButton(
              icon: Icon(Icons.delete_outline, size: 18, color: theme.textDim),
              onPressed: busy ? null : onDelete,
              tooltip: L10n.t('tts_voice_delete'),
            ),
          ],
        ],
      ),
    );
  }
}
