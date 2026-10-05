import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/chat_provider.dart';

/// A chat message's image, loaded through the backend (`GET /api/image`).
///
/// The message only carries the image's path *on the backend's disk*. This
/// used to be rendered with `Image.file(File(path))`, which only works when
/// the client runs on the same machine as the backend — on the web client,
/// the mobile app and any remote-access session every sent or generated
/// image silently rendered as nothing (the errorBuilder swallowed it).
/// Fetching the bytes from the backend works everywhere.
///
/// Fallback: an older desktop message can point at the user's own original
/// file (a picture picked from their Pictures folder, before the backend
/// kept its own copy), which the backend deliberately refuses to serve. When
/// the fetch yields nothing and that file exists locally, it is shown
/// straight from disk, exactly as before.
class ChatImage extends ConsumerStatefulWidget {
  const ChatImage({super.key, required this.path, this.width = 480});

  final String path;
  final double width;

  @override
  ConsumerState<ChatImage> createState() => _ChatImageState();
}

class _ChatImageState extends ConsumerState<ChatImage> {
  /// Decoded images by path — a message list rebuilds often (every streamed
  /// token), and a chat's images never change once written.
  static final Map<String, Uint8List> _cache = {};

  late Future<Uint8List?> _bytes;

  @override
  void initState() {
    super.initState();
    _bytes = _load();
  }

  @override
  void didUpdateWidget(ChatImage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.path != widget.path) _bytes = _load();
  }

  Future<Uint8List?> _load() async {
    final cached = _cache[widget.path];
    if (cached != null) return cached;
    try {
      final dataUri = await ref.read(apiClientProvider).getImageBase64(widget.path);
      final comma = dataUri.indexOf(',');
      if (!dataUri.startsWith('data:') || comma < 0) return null;
      final bytes = base64Decode(dataUri.substring(comma + 1));
      _cache[widget.path] = bytes;
      return bytes;
    } catch (e) {
      debugPrint('ChatImage: could not load ${widget.path}: $e');
      return null;
    }
  }

  bool get _localFileExists {
    if (kIsWeb) return false;
    try {
      return File(widget.path).existsSync();
    } catch (_) {
      return false;
    }
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<Uint8List?>(
      future: _bytes,
      builder: (context, snap) {
        final bytes = snap.data;
        if (bytes != null) {
          return Image.memory(
            bytes,
            width: widget.width,
            fit: BoxFit.contain,
            errorBuilder: (_, _, _) => const SizedBox.shrink(),
          );
        }
        if (snap.connectionState != ConnectionState.done) {
          return const SizedBox(
            width: 32,
            height: 32,
            child: Padding(
              padding: EdgeInsets.all(8),
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
          );
        }
        if (_localFileExists) {
          return Image.file(
            File(widget.path),
            width: widget.width,
            fit: BoxFit.contain,
            errorBuilder: (_, _, _) => const SizedBox.shrink(),
          );
        }
        return const SizedBox.shrink();
      },
    );
  }
}
