import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/clipboard_copy.dart';

/// The reported bug: on a Memo reached over plain HTTP (a Raspberry Pi on the
/// LAN, a VDS with no TLS) `navigator.clipboard` does not exist, Flutter's
/// web `Clipboard.setData` throws, and every copy button silently did nothing
/// while the UI still said "Copied" — the Future was never awaited, so the
/// error never surfaced either.
///
/// `flutter test` runs on the VM, so this exercises the native half: the
/// helper must reach the platform clipboard and report success. The web half
/// (secure-context branch, execCommand fallback) cannot run here — it is
/// covered by the real-browser check and by the fact that both branches are
/// exercised through the same `copyToClipboard` contract call sites use.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  int writes = 0;
  String? copied;

  setUp(() {
    writes = 0;
    copied = null;
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform,
            (MethodCall call) async {
      if (call.method == 'Clipboard.setData') {
        writes++;
        copied = (call.arguments as Map)['text'] as String?;
      }
      return null;
    });
  });

  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, null);
  });

  test('copyToClipboard puts the exact text on the clipboard', () async {
    final ok = await copyToClipboard('https://example.test/a?b=c&d=e');

    expect(ok, isTrue);
    expect(copied, 'https://example.test/a?b=c&d=e');
    expect(writes, 1, reason: 'exactly one clipboard write, not a retry storm');
  });

  test('copyToClipboard handles the empty string without throwing', () async {
    final ok = await copyToClipboard('');

    expect(ok, isTrue);
    expect(copied, '');
  });

  test('copyToClipboard preserves a long multi-line payload byte for byte',
      () async {
    // What actually gets copied: an OAuth authorize URL is one long line, and
    // a JSON token blob is many. A trim or a length clamp here would silently
    // produce an unusable link.
    final payload = List.generate(
      500,
      (i) => 'line $i — "quoted" & <tagged> \\ backslash ünïcödé',
    ).join('\n');

    final ok = await copyToClipboard(payload);

    expect(ok, isTrue);
    expect(copied, payload);
  });
}