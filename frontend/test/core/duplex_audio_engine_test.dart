import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/duplex_audio_engine.dart';

// NoAecDuplexAudioEngine.start() isn't exercised here (it opens a real
// platform audio channel, unavailable in a plain `flutter test` run) -- so
// these cover the parts of the Faz 4.1 contract that don't require a live
// device: the pre-start/no-AEC state every caller can rely on before
// deciding whether to enable automatic barge-in (see the plan's 4.3). Even
// so, `AudioRecorder`'s constructor fires an unawaited platform-channel
// `create` call, so the channel is stubbed below -- otherwise it throws a
// `MissingPluginException` asynchronously after the test that created it
// has already finished.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const channel = MethodChannel('com.llfbandit.record/messages');
  TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
      .setMockMethodCallHandler(channel, (call) async => null);

  test('aecAvailable is always false for the fallback engine', () {
    final engine = NoAecDuplexAudioEngine();
    expect(engine.aecAvailable, isFalse);
  });

  test('isActive is false before start()', () {
    final engine = NoAecDuplexAudioEngine();
    expect(engine.isActive, isFalse);
  });

  test('captureStream is empty before start()', () async {
    final engine = NoAecDuplexAudioEngine();
    expect(await engine.captureStream.isEmpty, isTrue);
  });

  test('writeRenderFrame() is a no-op and never throws', () {
    final engine = NoAecDuplexAudioEngine();
    engine.writeRenderFrame(Uint8List.fromList([1, 2, 3, 4]));
  });

  test('stop() before start() does not throw', () async {
    final engine = NoAecDuplexAudioEngine();
    await engine.stop(); // must not throw
  });

  test('dispose() before start() does not throw', () {
    final engine = NoAecDuplexAudioEngine();
    engine.dispose(); // must not throw
  });
}
