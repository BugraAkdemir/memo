import 'dart:async';
import 'dart:ui' as ui;

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../core/friendly_error.dart';
import '../core/l10n.dart';

/// What a picture is, read from its first bytes — the file name a message carries
/// says nothing (stored pictures keep a generic name, and the format is the
/// provider's choice).
class ImageFormat {
  const ImageFormat(this.label, this.extension, this.mime);
  final String label;
  final String extension;
  final String mime;

  static const png = ImageFormat('PNG', 'png', 'image/png');
  static const jpeg = ImageFormat('JPEG', 'jpg', 'image/jpeg');
  static const webp = ImageFormat('WebP', 'webp', 'image/webp');
  static const gif = ImageFormat('GIF', 'gif', 'image/gif');
  static const unknown = ImageFormat('?', 'png', 'application/octet-stream');
}

/// Identifies PNG, JPEG, WebP and GIF by their signatures; anything else is
/// [ImageFormat.unknown] (saved with a .png name rather than none).
ImageFormat sniffImageFormat(Uint8List b) {
  bool starts(List<int> sig, [int at = 0]) {
    if (b.length < at + sig.length) return false;
    for (var i = 0; i < sig.length; i++) {
      if (b[at + i] != sig[i]) return false;
    }
    return true;
  }

  if (starts(const [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])) return ImageFormat.png;
  if (starts(const [0xFF, 0xD8, 0xFF])) return ImageFormat.jpeg;
  if (starts(const [0x47, 0x49, 0x46, 0x38])) return ImageFormat.gif;
  if (starts(const [0x52, 0x49, 0x46, 0x46]) && starts(const [0x57, 0x45, 0x42, 0x50], 8)) {
    return ImageFormat.webp;
  }
  return ImageFormat.unknown;
}

/// "812 B", "34.2 KB", "1.5 MB".
String formatByteSize(int bytes) {
  if (bytes < 1024) return '$bytes B';
  if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
  return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
}

/// A file name for a saved picture: sortable and unique enough for one person.
String defaultImageFileName(ImageFormat format, [DateTime? now]) {
  final t = now ?? DateTime.now();
  String two(int n) => n.toString().padLeft(2, '0');
  return 'memo-image-${t.year}${two(t.month)}${two(t.day)}-${two(t.hour)}${two(t.minute)}${two(t.second)}.${format.extension}';
}

/// Writes picture bytes where the person chooses; returns where they landed, or
/// null when they cancelled. Injectable so tests need no native dialog.
typedef ImageSaver = Future<Uri?> Function({
  required String fileName,
  required Uint8List bytes,
  required ImageFormat format,
});

Future<Uri?> _saveWithFilePicker({
  required String fileName,
  required Uint8List bytes,
  required ImageFormat format,
}) {
  // saveFile writes the bytes itself on every platform and returns where they
  // landed (a file:// path on desktop, content:// on Android, a blob on web).
  return FilePicker.saveFile(
    dialogTitle: L10n.t('image_download'),
    fileName: fileName,
    bytes: bytes,
    mimeType: format.mime,
    type: FileType.custom,
    allowedExtensions: [format.extension],
  );
}

/// Saves a picture and tells the person where it went (or why it did not).
Future<void> saveImageBytes(
  BuildContext context,
  Uint8List bytes, {
  ImageSaver? saver,
}) async {
  final format = sniffImageFormat(bytes);
  final messenger = ScaffoldMessenger.maybeOf(context);
  try {
    final saved = await (saver ?? _saveWithFilePicker)(
      fileName: defaultImageFileName(format),
      bytes: bytes,
      format: format,
    );
    if (saved == null) return; // cancelled
    final where = saved.scheme == 'file' ? saved.toFilePath() : saved.toString();
    messenger?.showSnackBar(
      SnackBar(content: Text(L10n.t('image_downloaded', {'path': where}))),
    );
  } catch (e) {
    messenger?.showSnackBar(
      SnackBar(
        content: Text(L10n.t('image_download_failed', {'e': FriendlyError.describeGeneric(e)})),
      ),
    );
  }
}

/// Opens [bytes] full-window: zoom (buttons, wheel, pinch, double-tap), pan, the
/// picture's details, and download.
Future<void> showImageViewer(
  BuildContext context,
  Uint8List bytes, {
  ImageSaver? saver,
}) {
  return showGeneralDialog<void>(
    context: context,
    barrierLabel: L10n.t('image_close'),
    barrierColor: Colors.black87,
    pageBuilder: (ctx, _, _) => ImageViewer(bytes: bytes, saver: saver),
  );
}

class ImageViewer extends StatefulWidget {
  const ImageViewer({super.key, required this.bytes, this.saver});

  final Uint8List bytes;
  final ImageSaver? saver;

  @override
  State<ImageViewer> createState() => _ImageViewerState();
}

class _ImageViewerState extends State<ImageViewer> {
  static const _minScale = 0.5;
  static const _maxScale = 8.0;
  static const _step = 1.25;

  final _controller = TransformationController();
  final _focus = FocusNode();
  bool _showInfo = false;
  Size? _pixels;
  late final ImageFormat _format = sniffImageFormat(widget.bytes);

  @override
  void initState() {
    super.initState();
    _controller.addListener(() => setState(() {}));
    _decodeSize();
  }

  Future<void> _decodeSize() async {
    try {
      final codec = await ui.instantiateImageCodec(widget.bytes);
      final frame = await codec.getNextFrame();
      final size = Size(frame.image.width.toDouble(), frame.image.height.toDouble());
      frame.image.dispose();
      codec.dispose();
      if (mounted) setState(() => _pixels = size);
    } catch (_) {
      // Details simply omit the resolution.
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    _focus.dispose();
    super.dispose();
  }

  // The x axis' scale, not getMaxScaleOnAxis(): the matrix never scales z, so the
  // "max" of (0.8, 0.8, 1.0) would read 1.0 for any zoomed-OUT picture.
  double get _scale => _controller.value.getColumn(0).length;

  void _zoomTo(double target) {
    final next = target.clamp(_minScale, _maxScale);
    final factor = next / _scale;
    // Scale around the centre of the viewport, not the corner.
    final box = context.findRenderObject() as RenderBox?;
    final centre = box == null ? Offset.zero : box.size.center(Offset.zero);
    _controller.value = _controller.value.clone()
      ..translateByDouble(centre.dx, centre.dy, 0, 1)
      ..scaleByDouble(factor, factor, 1, 1)
      ..translateByDouble(-centre.dx, -centre.dy, 0, 1);
  }

  void _reset() => _controller.value = Matrix4.identity();

  void _toggleDoubleTapZoom() => _scale > 1.05 ? _reset() : _zoomTo(3);

  KeyEventResult _onKey(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent) return KeyEventResult.ignored;
    final k = event.logicalKey;
    if (k == LogicalKeyboardKey.equal || k == LogicalKeyboardKey.add || k == LogicalKeyboardKey.numpadAdd) {
      _zoomTo(_scale * _step);
      return KeyEventResult.handled;
    }
    if (k == LogicalKeyboardKey.minus || k == LogicalKeyboardKey.numpadSubtract) {
      _zoomTo(_scale / _step);
      return KeyEventResult.handled;
    }
    if (k == LogicalKeyboardKey.digit0 || k == LogicalKeyboardKey.numpad0) {
      _reset();
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  @override
  Widget build(BuildContext context) {
    final percent = (_scale * 100).round();
    return Focus(
      focusNode: _focus,
      autofocus: true,
      onKeyEvent: _onKey,
      child: Material(
        color: Colors.transparent,
        child: SafeArea(
          child: Column(
            children: [
              _toolbar(percent),
              Expanded(
                child: Stack(
                  children: [
                    Positioned.fill(
                      child: GestureDetector(
                        key: const Key('image_viewer_surface'),
                        behavior: HitTestBehavior.opaque,
                        onDoubleTap: _toggleDoubleTapZoom,
                        child: InteractiveViewer(
                          key: const Key('image_viewer_zoom'),
                          transformationController: _controller,
                          minScale: _minScale,
                          maxScale: _maxScale,
                          child: Center(
                            child: Image.memory(
                              widget.bytes,
                              fit: BoxFit.contain,
                              errorBuilder: (_, _, _) => const Icon(Icons.broken_image, color: Colors.white54, size: 64),
                            ),
                          ),
                        ),
                      ),
                    ),
                    if (_showInfo) Positioned(right: 12, bottom: 12, child: _infoCard()),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _toolbar(int percent) {
    Widget btn(String key, IconData icon, String tip, VoidCallback? onTap) => IconButton(
          key: Key(key),
          tooltip: tip,
          icon: Icon(icon, color: Colors.white),
          onPressed: onTap,
        );
    return Container(
      color: Colors.black54,
      padding: const EdgeInsets.symmetric(horizontal: 8),
      child: Row(
        children: [
          Expanded(
            child: Text(
              L10n.t('image_viewer_title'),
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w600),
            ),
          ),
          btn('image_zoom_out', Icons.zoom_out, L10n.t('image_zoom_out'), () => _zoomTo(_scale / _step)),
          SizedBox(
            width: 52,
            child: Text(
              '$percent%',
              key: const Key('image_zoom_percent'),
              textAlign: TextAlign.center,
              style: const TextStyle(color: Colors.white70),
            ),
          ),
          btn('image_zoom_in', Icons.zoom_in, L10n.t('image_zoom_in'), () => _zoomTo(_scale * _step)),
          btn('image_zoom_reset', Icons.fit_screen, L10n.t('image_zoom_reset'), _reset),
          btn('image_info', _showInfo ? Icons.info : Icons.info_outline, L10n.t('image_info'),
              () => setState(() => _showInfo = !_showInfo)),
          btn('image_download', Icons.download, L10n.t('image_download'),
              () => saveImageBytes(context, widget.bytes, saver: widget.saver)),
          btn('image_close', Icons.close, L10n.t('image_close'), () => Navigator.of(context).maybePop()),
        ],
      ),
    );
  }

  Widget _infoCard() {
    final px = _pixels;
    String ratio() {
      if (px == null || px.height == 0) return '';
      int gcd(int a, int b) => b == 0 ? a : gcd(b, a % b);
      final w = px.width.round(), h = px.height.round();
      final g = gcd(w, h);
      return '${w ~/ g}:${h ~/ g}';
    }

    Widget row(String label, String value) => Padding(
          padding: const EdgeInsets.symmetric(vertical: 2),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              SizedBox(width: 120, child: Text(label, style: const TextStyle(color: Colors.white60, fontSize: 12))),
              Text(value, style: const TextStyle(color: Colors.white, fontSize: 12)),
            ],
          ),
        );

    return Container(
      key: const Key('image_info_card'),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.black87,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: Colors.white24),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (px != null) row(L10n.t('image_info_dimensions'), '${px.width.round()} × ${px.height.round()}'),
          if (px != null) row(L10n.t('image_info_ratio'), ratio()),
          row(L10n.t('image_info_format'), _format.label),
          row(L10n.t('image_info_size'), formatByteSize(widget.bytes.length)),
        ],
      ),
    );
  }
}
