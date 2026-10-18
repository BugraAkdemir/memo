import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/core/local_session_state.dart';
import 'package:memo_flutter/models/server_browse_entry.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/server_file_browser_dialog.dart';

ServerBrowseEntry d(String n) => ServerBrowseEntry(name: n, isDir: true);
ServerBrowseEntry f(String n) => ServerBrowseEntry(name: n, isDir: false, size: 1);

/// A small fake filesystem: path -> listing. Unknown paths fail like the backend
/// does ("stat: no such file or directory").
class _Api extends MemoApiClient {
  _Api() : super(baseUrl: 'http://memo.test');
  final asked = <String>[];
  final fs = <String, List<ServerBrowseEntry>>{
    '/home/u': [d('.cache'), d('.config'), d('Documents'), d('work'), f('notes.txt')],
    '/home/u/Documents': [d('memo'), d('minecraft-ai-agent'), f('cv.pdf')],
    '/home/u/Documents/memo': [d('internal'), f('go.mod'), f('AGENTS.md')],
    '/home/u/work': [],
  };

  @override
  Future<ServerBrowseResult> browseServer(String path) async {
    asked.add(path);
    var p = path.isEmpty ? '/home/u' : path;
    // A file path browses its folder, like the backend.
    if (p == '/home/u/Documents/memo/go.mod') p = '/home/u/Documents/memo';
    final entries = fs[p];
    if (entries == null) {
      throw DioException(
        requestOptions: RequestOptions(path: '/api/files/browse'),
        type: DioExceptionType.badResponse,
        response: Response(requestOptions: RequestOptions(path: '/api/files/browse'), statusCode: 500, data: {'error': 'stat: no such file or directory'}),
      );
    }
    final parent = p == '/' || p.lastIndexOf('/') == 0 ? '/' : p.substring(0, p.lastIndexOf('/'));
    return ServerBrowseResult(path: p, parent: p == '/' ? '' : parent, entries: entries);
  }
}

void main() {
  late _Api api;
  late SharedPreferences prefs;
  String? picked;

  Future<void> open(WidgetTester tester, ServerBrowseMode mode, {Map<String, Object> prefsInit = const {}}) async {
    SharedPreferences.setMockInitialValues(prefsInit);
    prefs = await SharedPreferences.getInstance();
    L10n.setLocale(MemoLocale.en);
    tester.view.physicalSize = const Size(1000, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    api = _Api();
    picked = null;
    await tester.pumpWidget(ProviderScope(
      overrides: [prefsProvider.overrideWithValue(prefs), apiClientProvider.overrideWithValue(api)],
      child: MaterialApp(
        home: Builder(
          builder: (context) => Scaffold(
            body: TextButton(
              onPressed: () async => picked = await showServerFileBrowserDialog(context, mode: mode),
              child: const Icon(Icons.launch),
            ),
          ),
        ),
      ),
    ));
    await tester.tap(find.byIcon(Icons.launch));
    await tester.pumpAndSettle();
  }

  Finder pathBar() => find.byWidgetPredicate((w) => w is TextField && w.decoration?.hintText == L10n.t('server_browse_path_hint'));
  Finder filterBar() => find.byWidgetPredicate((w) => w is TextField && w.decoration?.hintText == L10n.t('server_browse_filter_hint'));

  testWidgets('hidden (dot) folders are not listed until asked for', (tester) async {
    await open(tester, ServerBrowseMode.directory);
    expect(find.text('Documents'), findsOneWidget);
    expect(find.text('.cache'), findsNothing, reason: 'a wall of dot-folders was the reported problem');

    await tester.tap(find.byTooltip(L10n.t('server_browse_show_hidden')));
    await tester.pumpAndSettle();
    expect(find.text('.cache'), findsOneWidget);
    expect(prefs.getBool('memo_server_browse_show_hidden'), isTrue, reason: 'the choice is remembered');
  });

  testWidgets('a pasted path opens on Enter', (tester) async {
    await open(tester, ServerBrowseMode.directory);
    await tester.enterText(pathBar(), '/home/u/Documents/memo');
    await tester.testTextInput.receiveAction(TextInputAction.go);
    await tester.pumpAndSettle();

    expect(api.asked.last, '/home/u/Documents/memo');
    expect(find.text('internal'), findsOneWidget);
    expect(tester.widget<TextField>(pathBar()).controller!.text, '/home/u/Documents/memo');
  });

  testWidgets('~ means the backend\'s home folder', (tester) async {
    await open(tester, ServerBrowseMode.directory);
    await tester.enterText(pathBar(), '~/Documents');
    await tester.testTextInput.receiveAction(TextInputAction.go);
    await tester.pumpAndSettle();
    expect(api.asked.last, '/home/u/Documents');
    expect(find.text('minecraft-ai-agent'), findsOneWidget);
  });

  testWidgets('a path that does not exist keeps the list and says why', (tester) async {
    await open(tester, ServerBrowseMode.directory);
    await tester.enterText(pathBar(), '/home/u/nope');
    await tester.testTextInput.receiveAction(TextInputAction.go);
    await tester.pumpAndSettle();

    expect(find.textContaining('no such file'), findsOneWidget);
    expect(find.text('Documents'), findsOneWidget, reason: 'a typo must not wipe the folder being viewed');
  });

  testWidgets('the filter narrows the folder as you type and says when nothing matches', (tester) async {
    await open(tester, ServerBrowseMode.directory);
    await tester.tap(find.text('Documents'));
    await tester.pumpAndSettle();
    expect(find.text('memo'), findsOneWidget);
    expect(find.text('minecraft-ai-agent'), findsOneWidget);

    await tester.enterText(filterBar(), 'mine');
    await tester.pump();
    expect(find.text('minecraft-ai-agent'), findsOneWidget);
    expect(find.text('memo'), findsNothing);

    await tester.enterText(filterBar(), 'zzz');
    await tester.pump();
    expect(find.text(L10n.t('server_browse_no_match')), findsOneWidget);
  });

  testWidgets('arrow keys move the highlight and Enter opens it, all from the filter box', (tester) async {
    await open(tester, ServerBrowseMode.directory);
    // Visible (dot-folders hidden): Documents, work, notes.txt. Highlight starts on the first.
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown); // -> work
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    expect(api.asked.last, '/home/u/work');

    await tester.enterText(pathBar(), '/home/u');
    await tester.testTextInput.receiveAction(TextInputAction.go);
    await tester.pumpAndSettle();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter); // first row = Documents
    await tester.pumpAndSettle();
    expect(api.asked.last, '/home/u/Documents');
  });

  testWidgets('choosing a folder returns it and remembers it as recent', (tester) async {
    await open(tester, ServerBrowseMode.directory);
    await tester.enterText(pathBar(), '/home/u/Documents/memo');
    await tester.testTextInput.receiveAction(TextInputAction.go);
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('server_browse_select_folder')));
    await tester.pumpAndSettle();

    expect(picked, '/home/u/Documents/memo');
    expect(prefs.getStringList(recentServerDirsKey), ['/home/u/Documents/memo']);
  });

  testWidgets('recent folders show as chips and open with one tap', (tester) async {
    await open(tester, ServerBrowseMode.directory, prefsInit: {
      recentServerDirsKey: ['/home/u/Documents/memo', '/home/u/work'],
    });
    expect(find.text(L10n.t('server_browse_recent')), findsOneWidget);
    await tester.tap(find.widgetWithText(ActionChip, 'memo'));
    await tester.pumpAndSettle();
    expect(api.asked.last, '/home/u/Documents/memo');
    expect(find.text('go.mod'), findsOneWidget);
  });

  test('the recent-folders key is cleared with the other server-coupled state', () {
    expect(serverCoupledPrefsKeys, contains(recentServerDirsKey));
  });

  testWidgets('file mode: pasting the full path of a file selects it directly', (tester) async {
    await open(tester, ServerBrowseMode.file);
    await tester.enterText(pathBar(), '/home/u/Documents/memo/go.mod');
    await tester.testTextInput.receiveAction(TextInputAction.go);
    await tester.pumpAndSettle();
    expect(picked, '/home/u/Documents/memo/go.mod');
  });

  testWidgets('the home button goes back to the home folder', (tester) async {
    await open(tester, ServerBrowseMode.directory);
    await tester.tap(find.text('Documents'));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip(L10n.t('server_browse_home')));
    await tester.pumpAndSettle();
    expect(find.text('work'), findsOneWidget);
  });

  testWidgets('fits a phone width without overflowing', (tester) async {
    await open(tester, ServerBrowseMode.directory);
    tester.view.physicalSize = const Size(360, 700);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });
}
