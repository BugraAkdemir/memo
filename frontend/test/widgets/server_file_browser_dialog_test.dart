import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/server_browse_entry.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/settings_provider.dart';
import 'package:memo_flutter/widgets/server_file_browser_dialog.dart';

class _Api extends MemoApiClient {
  _Api() : super(baseUrl: 'http://memo.test');
  final created = <(String, String)>[];
  String? refuse;

  @override
  Future<ServerBrowseResult> browseServer(String path) async =>
      const ServerBrowseResult(path: '/home/u', parent: '/home', entries: [
        ServerBrowseEntry(name: 'docs', isDir: true, size: 0),
        ServerBrowseEntry(name: 'a.txt', isDir: false, size: 1),
      ]);

  @override
  Future<ServerBrowseResult> makeServerDirectory(String parent, String name) async {
    created.add((parent, name));
    if (refuse != null) {
      throw DioException(
        requestOptions: RequestOptions(path: '/api/files/mkdir'),
        type: DioExceptionType.badResponse,
        response: Response(
          requestOptions: RequestOptions(path: '/api/files/mkdir'),
          statusCode: 400,
          data: {'error': refuse},
        ),
      );
    }
    return ServerBrowseResult(path: '$parent/$name', parent: parent, entries: const []);
  }
}

void main() {
  late _Api api;
  String? picked;

  Future<void> open(WidgetTester tester, ServerBrowseMode mode) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    L10n.setLocale(MemoLocale.en);
    api = _Api();
    picked = null;
    await tester.pumpWidget(ProviderScope(
      overrides: [
        prefsProvider.overrideWithValue(prefs),
        apiClientProvider.overrideWithValue(api),
      ],
      child: MaterialApp(
        home: Builder(
          builder: (context) => Scaffold(
            body: TextButton(
              onPressed: () async =>
                  picked = await showServerFileBrowserDialog(context, mode: mode),
              child: const Icon(Icons.launch),
            ),
          ),
        ),
      ),
    ));
    await tester.tap(find.byIcon(Icons.launch));
    await tester.pumpAndSettle();
  }

  Finder newFolderButton() => find.byTooltip(L10n.t('server_browse_new_folder'));

  testWidgets('a folder picker offers "new folder"; the created folder opens and can be selected',
      (tester) async {
    await open(tester, ServerBrowseMode.directory);
    expect(newFolderButton(), findsOneWidget);

    await tester.tap(newFolderButton());
    await tester.pumpAndSettle();
    await tester.enterText(find.descendant(of: find.byType(AlertDialog), matching: find.byType(TextField)), 'my-project');
    await tester.tap(find.text(L10n.t('server_browse_create')));
    await tester.pumpAndSettle();

    expect(api.created, [('/home/u', 'my-project')]);
    // Now inside the new, empty folder.
    expect(find.text('/home/u/my-project'), findsOneWidget);
    expect(find.text(L10n.t('server_browse_empty')), findsOneWidget);

    await tester.tap(find.text(L10n.t('server_browse_select_folder')));
    await tester.pumpAndSettle();
    expect(picked, '/home/u/my-project');
  });

  testWidgets('a refusal stays in the name dialog with the server\'s reason',
      (tester) async {
    await open(tester, ServerBrowseMode.directory);
    api.refuse = '"docs" already exists';
    await tester.tap(newFolderButton());
    await tester.pumpAndSettle();
    await tester.enterText(find.descendant(of: find.byType(AlertDialog), matching: find.byType(TextField)), 'docs');
    await tester.tap(find.text(L10n.t('server_browse_create')));
    await tester.pumpAndSettle();

    expect(find.textContaining('already exists'), findsOneWidget);
    expect(find.text(L10n.t('server_browse_folder_name')), findsOneWidget,
        reason: 'the dialog must stay open so the name can be fixed');
    // Editing clears the message.
    await tester.enterText(find.descendant(of: find.byType(AlertDialog), matching: find.byType(TextField)), 'docs2');
    await tester.pump();
    expect(find.textContaining('already exists'), findsNothing);
  });

  testWidgets('an empty name creates nothing', (tester) async {
    await open(tester, ServerBrowseMode.directory);
    await tester.tap(newFolderButton());
    await tester.pumpAndSettle();
    await tester.tap(find.text(L10n.t('server_browse_create')));
    await tester.pumpAndSettle();
    expect(api.created, isEmpty);
  });

  testWidgets('a file picker has no new-folder button', (tester) async {
    await open(tester, ServerBrowseMode.file);
    expect(newFolderButton(), findsNothing);
  });

  testWidgets('fits a 360 px phone without overflowing', (tester) async {
    tester.view.physicalSize = const Size(360, 700);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await open(tester, ServerBrowseMode.directory);
    expect(tester.takeException(), isNull, reason: 'the picker itself');
    await tester.tap(newFolderButton());
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull, reason: 'the name prompt');
  });
}
