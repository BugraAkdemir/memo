import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/auth_gate_provider.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/screens/routines_screen.dart';

/// A fake [HttpClientAdapter] for the handful of endpoints RoutinesScreen
/// calls. Hooks into Dio's adapter layer directly (real sockets aren't
/// reachable under `flutter test`) — same technique as
/// settings_toggle_race_test.dart.
class _FakeRoutinesAdapter implements HttpClientAdapter {
  bool whatsAppChatsFetched = false;
  Map<String, dynamic>? lastCreatedDraft;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    ResponseBody json(Object body, [int status = 200]) => ResponseBody.fromString(
          jsonEncode(body),
          status,
          headers: {
            Headers.contentTypeHeader: [Headers.jsonContentType],
          },
        );

    if (options.method == 'GET' && options.path == '/api/routines') {
      return json(<Map<String, dynamic>>[]);
    }
    if (options.method == 'POST' && options.path == '/api/routines/parse') {
      // The LLM picked Telegram-only delivery — the user must be able to
      // switch WhatsApp on from the confirmation card without discarding
      // and re-typing the request (BUG-L2).
      return json({
        'time_of_day': '09:00',
        'weekdays': <int>[],
        'prompt': 'Bana günaydın de',
        'needs_agent_mode': false,
        'context_source_type': 'none',
        'whatsapp_chat_hint': '',
        'delivery_whatsapp': false,
        'delivery_telegram': true,
      });
    }
    if (options.method == 'GET' && options.path == '/api/whatsapp/chats') {
      whatsAppChatsFetched = true;
      return json(<Map<String, dynamic>>[
        {'jid': '123@s.whatsapp.net', 'display_name': 'Test Chat'},
      ]);
    }
    if (options.method == 'POST' && options.path == '/api/routines') {
      final bytes = await (requestStream ?? const Stream.empty()).expand((e) => e).toList();
      final body = jsonDecode(utf8.decode(bytes)) as Map<String, dynamic>;
      lastCreatedDraft = Map<String, dynamic>.from(body['draft'] as Map);
      return json({'id': 'r1', 'enabled': true});
    }
    return json({'error': 'unhandled ${options.method} ${options.path}'}, 404);
  }

  @override
  void close({bool force = false}) {}
}

Future<ProviderContainer> _pumpRoutines(WidgetTester tester, _FakeRoutinesAdapter adapter) async {
  final client = MemoApiClient(baseUrl: 'http://memo.test');
  client.dio.httpClientAdapter = adapter;

  final container = ProviderContainer(overrides: [
    apiClientProvider.overrideWithValue(client),
    authGateProvider.overrideWith((ref) => Stream.value(const AuthGateInfo(AuthGateState.ok))),
  ]);
  addTearDown(container.dispose);
  await container.read(authGateProvider.future);

  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: const MaterialApp(home: Scaffold(body: RoutinesScreen())),
    ),
  );
  await tester.pumpAndSettle();
  return container;
}

// Regression test for BUG-L2: the WhatsApp/Telegram delivery chips in the
// routine confirmation card used to be plain, non-interactive Chip widgets
// showing only whichever channel(s) the LLM happened to parse from the free
// text — with no onTap/onSelected at all, so a wrong guess (or one the user
// simply wants to add to) could only be fixed by discarding the draft and
// re-typing the whole request with more explicit wording. They're
// FilterChips now: both channels are always shown and toggleable.
void main() {
  testWidgets('routine delivery-channel chips are interactive and toggle the draft',
      (tester) async {
    final adapter = _FakeRoutinesAdapter();
    await _pumpRoutines(tester, adapter);

    await tester.enterText(find.byType(TextField), 'her sabah saat 9da günaydın de');
    await tester.tap(find.text(L10n.t('send')));
    await tester.pumpAndSettle();

    // Parsed draft: WhatsApp chip unselected, Telegram chip selected.
    final whatsappChip = tester.widget<FilterChip>(
      find.ancestor(of: find.text('WhatsApp'), matching: find.byType(FilterChip)),
    );
    expect(whatsappChip.selected, false);
    final telegramChip = tester.widget<FilterChip>(
      find.ancestor(of: find.text('Telegram'), matching: find.byType(FilterChip)),
    );
    expect(telegramChip.selected, true);

    // Turning WhatsApp on must actually flip the draft (not just repaint a
    // static Chip) and lazily fetch the chat list, same as if the LLM had
    // picked WhatsApp itself.
    await tester.tap(find.ancestor(of: find.text('WhatsApp'), matching: find.byType(FilterChip)));
    await tester.pumpAndSettle();

    expect(adapter.whatsAppChatsFetched, true,
        reason: 'toggling WhatsApp on should fetch the chat list to pick a target');
    expect(find.byType(DropdownButton<String>), findsOneWidget,
        reason: 'the WhatsApp chat picker should appear once the chip is on and chats are loaded');

    // Turning Telegram back off must also reach the draft that gets saved.
    await tester.tap(find.ancestor(of: find.text('Telegram'), matching: find.byType(FilterChip)));
    await tester.pumpAndSettle();

    // Pick the WhatsApp chat (required before saving with delivery_whatsapp
    // true — see BUG-L2's existing _confirm() guard) and save.
    await tester.tap(find.byType(DropdownButton<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Test Chat').last);
    await tester.pumpAndSettle();

    await tester.tap(find.text(L10n.t('save')));
    await tester.pumpAndSettle();

    expect(adapter.lastCreatedDraft, isNotNull);
    expect(adapter.lastCreatedDraft!['delivery_whatsapp'], true,
        reason: 'toggling the WhatsApp chip on must be reflected in the saved routine');
    expect(adapter.lastCreatedDraft!['delivery_telegram'], false,
        reason: 'toggling the Telegram chip off must be reflected in the saved routine');
  });
}
