import 'dart:async';
// Compatibility with the app's supported SDK.
// ignore: deprecated_member_use
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/core/tray_controller.dart';
import 'package:memo_flutter/providers/models_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _Api extends MemoApiClient {
  _Api() : super(baseUrl: 'http://fixture.invalid');
  final unregistered = <String>[];
  @override
  Future<void> unregisterClient(String id) async => unregistered.add(id);
}

void main() {
  const windowChannel = MethodChannel('window_manager');
  const trayChannel = MethodChannel('tray_manager');
  final binding = TestWidgetsFlutterBinding.ensureInitialized();
  final windowCalls = <MethodCall>[];
  final trayCalls = <MethodCall>[];
  var failIcon = false;
  var failMenu = false;
  Completer<void>? iconPending;
  late SharedPreferences prefs;
  late _Api api;
  setUp(() async {
    windowCalls.clear();
    trayCalls.clear();
    api = _Api();
    failIcon = failMenu = false;
    iconPending = null;
    SharedPreferences.setMockInitialValues({
      'memo_minimize_to_tray': true,
      'memo_tray_hint_shown': true,
    });
    prefs = await SharedPreferences.getInstance();
    binding.defaultBinaryMessenger.setMockMethodCallHandler(windowChannel, (
      call,
    ) async {
      windowCalls.add(call);
      if (call.method == 'isMinimized') return false;
      return null;
    });
    binding.defaultBinaryMessenger.setMockMethodCallHandler(trayChannel, (
      call,
    ) async {
      trayCalls.add(call);
      if (call.method == 'setIcon') {
        await iconPending?.future;
        if (failIcon) throw PlatformException(code: 'icon_unavailable');
      }
      if (call.method == 'setContextMenu' && failMenu) {
        throw PlatformException(code: 'menu_unavailable');
      }
      return true;
    });
  });
  tearDown(() {
    binding.defaultBinaryMessenger.setMockMethodCallHandler(
      windowChannel,
      null,
    );
    binding.defaultBinaryMessenger.setMockMethodCallHandler(trayChannel, null);
    L10n.setLocale(MemoLocale.en);
  });
  Future<void> mount(WidgetTester tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          prefsProvider.overrideWithValue(prefs),
          apiClientProvider.overrideWithValue(api),
          currentClientIdProvider.overrideWith((ref) => 'desktop-fixture'),
          modelStatusProvider.overrideWith((ref) => const Stream.empty()),
        ],
        child: TrayController(
          child: MaterialApp(
            navigatorKey: trayNavigatorKey,
            home: const Scaffold(body: SizedBox()),
          ),
        ),
      ),
    );
    await tester.pump();
  }

  Future<void> close(WidgetTester tester) async {
    await binding.defaultBinaryMessenger.handlePlatformMessage(
      windowChannel.name,
      const StandardMethodCodec().encodeMethodCall(
        const MethodCall('onEvent', {'eventName': 'close'}),
      ),
      (_) {},
    );
    await tester.pump();
  }

  List<String> calls() => windowCalls.map((c) => c.method).toList();

  testWidgets('successful tray hides without destroying the window', (
    tester,
  ) async {
    await mount(tester);
    await close(tester);
    expect(calls(), contains('hide'));
    expect(api.unregistered, isEmpty);
    expect(calls(), isNot(contains('destroy')));
    expect(calls(), isNot(contains('minimize')));
    expect(trayCalls.where((c) => c.method == 'setContextMenu'), isNotEmpty);
    await tester.pumpWidget(const SizedBox());
  });
  for (final locale in MemoLocale.values) {
    testWidgets('failed icon remains reachable and localized ($locale)', (
      tester,
    ) async {
      L10n.setLocale(locale);
      await prefs.remove('memo_tray_hint_shown');
      failIcon = true;
      await mount(tester);
      await close(tester);
      expect(calls(), containsAllInOrder(['setSkipTaskbar', 'minimize']));
      expect(
        windowCalls.lastWhere((c) => c.method == 'setSkipTaskbar').arguments,
        {'isSkipTaskbar': false},
      );
      expect(calls(), isNot(contains('hide')));
      expect(calls(), isNot(contains('destroy')));
      expect(find.text(L10n.t('tray_unavailable')), findsOneWidget);
      expect(prefs.getBool('memo_minimize_to_tray'), isTrue);
      expect(prefs.getBool('memo_tray_hint_shown'), isNull);
      await tester.pumpWidget(const SizedBox());
    });
  }
  testWidgets('menu failure also prevents hiding', (tester) async {
    failMenu = true;
    await mount(tester);
    await close(tester);
    expect(calls(), contains('minimize'));
    expect(calls(), isNot(contains('hide')));
    await tester.pumpWidget(const SizedBox());
  });
  testWidgets('failed setup recovers on next X', (tester) async {
    failIcon = true;
    await mount(tester);
    await close(tester);
    expect(calls(), contains('minimize'));
    failIcon = false;
    windowCalls.clear();
    await close(tester);
    expect(calls(), contains('hide'));
    expect(calls(), isNot(contains('destroy')));
    await tester.pumpWidget(const SizedBox());
  });
  testWidgets('pending setup times out safely and serializes closes', (
    tester,
  ) async {
    iconPending = Completer<void>();
    await mount(tester);
    await close(tester);
    await close(tester);
    expect(calls(), isNot(contains('hide')));
    await tester.pump(const Duration(seconds: 2));
    await tester.pump();
    expect(calls().where((c) => c == 'minimize'), hasLength(1));
    expect(trayCalls.where((c) => c.method == 'setIcon'), hasLength(1));
    iconPending!.complete();
    await tester.pump();
    windowCalls.clear();
    await close(tester);
    expect(calls(), contains('hide'));
    await tester.pumpWidget(const SizedBox());
  });
  testWidgets('tray Quit unregisters once and destroys, Open restores the UI', (
    tester,
  ) async {
    await mount(tester);
    await close(tester);
    final menuCall = trayCalls.lastWhere((c) => c.method == 'setContextMenu');
    final items = (menuCall.arguments['menu']['items'] as List).cast<Map>();
    Future<void> click(String label) async {
      final item = items.firstWhere((i) => i['label'] == label);
      await binding.defaultBinaryMessenger.handlePlatformMessage(
        trayChannel.name,
        const StandardMethodCodec().encodeMethodCall(
          MethodCall('onTrayMenuItemClick', {'id': item['id']}),
        ),
        (_) {},
      );
      await tester.pump();
    }

    windowCalls.clear();
    await click(L10n.t('tray_open'));
    expect(calls(), containsAllInOrder(['show', 'focus']));
    expect(api.unregistered, isEmpty);
    await click(L10n.t('tray_quit'));
    await click(L10n.t('tray_quit'));
    expect(api.unregistered, ['desktop-fixture']);
    expect(calls().where((c) => c == 'destroy'), hasLength(1));
    await tester.pumpWidget(const SizedBox());
  });

  testWidgets('explicit close preference exits even with failed tray', (
    tester,
  ) async {
    await prefs.setBool('memo_minimize_to_tray', false);
    failIcon = true;
    await mount(tester);
    await close(tester);
    expect(calls(), contains('destroy'));
    expect(calls(), isNot(contains('hide')));
    await tester.pumpWidget(const SizedBox());
  });
}
