import 'dart:async';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/mascot_channel.dart';
import 'package:memo_flutter/providers/mascot_provider.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  const windows = MethodChannel('mixin.one/desktop_multi_window');
  const channels = MethodChannel('mixin.one/desktop_multi_window/channels');
  const codec = StandardMethodCodec();
  final messenger =
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  late MascotWindowNotifier owner;
  late List<MethodCall> commands;
  late int created;
  late bool autoReady;

  Future<void> notify(String method) async {
    final reply = Completer<void>();
    await messenger.handlePlatformMessage(
      channels.name,
      codec.encodeMethodCall(
        MethodCall('methodCall', {
          'channel': mascotLifecycleChannel.name,
          'method': method,
          'arguments': null,
        }),
      ),
      (_) => reply.complete(),
    );
    await reply.future;
  }

  setUp(() {
    commands = [];
    created = 0;
    autoReady = true;
    messenger.setMockMethodCallHandler(windows, (call) async {
      if (call.method == 'getAllWindows') {
        return [
          {'windowId': 'main', 'windowArgument': ''},
          if (created > 0)
            {'windowId': 'pet', 'windowArgument': mascotWindowArgument},
        ];
      }
      if (call.method == 'createWindow') {
        created++;
        if (autoReady) {
          unawaited(Future<void>.delayed(Duration.zero, () => notify('ready')));
        }
        return 'pet';
      }
      return null;
    });
    messenger.setMockMethodCallHandler(channels, (call) async {
      if (call.method == 'invokeMethod') {
        final args = call.arguments as Map;
        commands.add(MethodCall(args['method'] as String, args['arguments']));
      }
      return null;
    });
    owner = MascotWindowNotifier();
  });

  tearDown(() async {
    owner.dispose();
    await mascotLifecycleChannel.setMethodCallHandler(null);
    messenger.setMockMethodCallHandler(windows, null);
    messenger.setMockMethodCallHandler(channels, null);
  });

  test('opening waits for the pet handler before sending commands', () async {
    autoReady = false;
    final opening = owner.toggle();
    await Future<void>.delayed(const Duration(milliseconds: 10));
    expect(commands, isEmpty);
    expect(owner.state, isNull);
    await notify('ready');
    await opening;
    expect(commands.single.method, 'show');
    expect(owner.state?.windowId, 'pet');
  });

  test('off/on and rapid toggles retain one engine and run in order', () async {
    await Future.wait([
      owner.toggle(),
      owner.toggle(),
      owner.toggle(),
      owner.toggle(),
    ]);
    expect(created, 1);
    expect(commands.map((c) => c.method), ['show', 'hide', 'show', 'hide']);
    expect(owner.state, isNull);
    await owner.toggle();
    expect(created, 1);
    expect(owner.state?.windowId, 'pet');
  });

  test(
    'a close from the pet clears visibility without losing its engine',
    () async {
      await owner.toggle();
      await notify('hidden');
      expect(owner.state, isNull);
      await owner.toggle();
      expect(created, 1);
      expect(commands.map((c) => c.method), ['show', 'show']);
    },
  );

  test('skin changes are sent to the same open or hidden pet', () async {
    await owner.updateSkin('pixel');
    expect(created, 0);
    await owner.toggle();
    await owner.updateSkin('pixel');
    expect(owner.state?.windowId, 'pet');
    await owner.toggle();
    await owner.updateSkin('classic');
    expect(created, 1);
    expect(owner.state, isNull);
    expect(commands.map((c) => c.method), ['show', 'skin', 'hide', 'skin']);
    expect(commands[1].arguments, 'pixel');
    expect(commands.last.arguments, 'classic');
  });
}
