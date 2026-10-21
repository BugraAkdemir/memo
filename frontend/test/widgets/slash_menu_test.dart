import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/providers/skill_provider.dart';
import 'package:memo_flutter/widgets/slash_menu.dart';

import '../support/fake_skills.dart';

void main() {
  List<SlashItem> items({String query = '', Set<String> active = const {}, bool atStart = true}) =>
      filterSlashItems(
        buildSlashItems(skills: fakeSkills(), activeSkills: active, atMessageStart: atStart),
        query,
      );

  group('what the menu lists', () {
    test('every installed skill, then the commands that do something real', () {
      final all = items();
      final skills = all.where((i) => i.kind == SlashKind.skill).toList();
      expect(skills, hasLength(fakeSkills().length));
      // Skills first, in name order…
      expect(all.take(skills.length).every((i) => i.kind == SlashKind.skill), isTrue);
      final names = skills.map((s) => s.label.toLowerCase()).toList();
      expect([...names]..sort(), names);
      // …then the commands.
      expect(all.skip(skills.length).map((i) => i.key),
          ['/remember', '/forget', '/model', '/orchestra']);
    });

    test('no canned prompt templates are left', () {
      final keys = items().map((i) => i.key).toSet();
      for (final gone in ['/code', '/explain', '/fix', '/plan', '/summary', '/compare', '/brainstorm', '/translate', '/skill']) {
        expect(keys, isNot(contains(gone)));
      }
    });

    test('a skill is inserted as its /name form, the one the backend recognises', () {
      final byLabel = {for (final i in items()) i.label: i.key};
      expect(byLabel['Pair Programming'], '/pair-programming');
      expect(byLabel['Verification & Quality Assurance'], '/verification-quality-assurance');
      expect(byLabel['codebase-memory'], '/codebase-memory');
      expect(skillSlug('  V3 CLI  '), 'v3-cli');
    });

    test('from the middle of a sentence only skills can be called', () {
      final mid = items(atStart: false);
      expect(mid.every((i) => i.kind == SlashKind.skill), isTrue);
      expect(mid, hasLength(fakeSkills().length));
    });

    test('typing filters by name, command and description', () {
      expect(items(query: 'pair').single.label, 'Pair Programming');
      expect(items(query: 'navigator').single.label, 'Pair Programming');
      expect(items(query: 'mod').map((i) => i.key), contains('/model'));
      expect(items(query: 'zzzz'), isEmpty);
    });

    test('typing a command beats skills that only mention it in their description', () {
      final crowded = [
        ...fakeSkills(),
        const SkillDefinition(name: 'aaa-first', description: 'works with any model and remembers things'),
        const SkillDefinition(name: 'zzz-modelling', description: 'model everything'),
      ];
      List<String> keys(String q) => filterSlashItems(buildSlashItems(skills: crowded), q).map((i) => i.key).toList();
      expect(keys('model').first, '/model', reason: 'the exact command comes first');
      expect(keys('remember').first, '/remember');
      expect(keys('mod').first, '/model', reason: 'a command starting with it beats a name containing it');
      expect(keys('model'), containsAll(['/zzz-modelling', '/aaa-first']));
    });

    // The same vectors are in internal/app/skill_invoke_test.go: the slug the
    // menu inserts is the one the backend recognises, so they must agree.
    test('skillSlug agrees with the backend on Turkish and edge-case names', () {
      const vectors = {
        'Pair Programming': 'pair-programming',
        'codebase-memory': 'codebase-memory',
        'Verification & Quality Assurance': 'verification-quality-assurance',
        '  V3 CLI  ': 'v3-cli',
        'ecc:go-review': 'ecc-go-review',
        'İş Planı': 'iş-planı',
        'GEREKLİ SKİLLER': 'gerekli-skiller',
        'x² y': 'x-y',
      };
      vectors.forEach((name, slug) => expect(skillSlug(name), slug, reason: name));
    });

    test('a skill the chat already called is marked', () {
      final marked = items(active: {'Pair Programming'}).where((i) => i.active).toList();
      expect(marked.map((i) => i.label), ['Pair Programming']);
    });
  });

  group('the popup', () {
    Widget host(int selected, {List<SlashItem>? rows, ValueChanged<SlashItem>? onSelect}) => MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(
              child: Column(children: [
                SlashMenuPopup(
                  items: rows ?? items(),
                  selectedIndex: selected,
                  onSelect: onSelect ?? (_) {},
                ),
                const SizedBox(height: 1000),
              ]),
            ),
          ),
        );

    void expectVisible(WidgetTester tester, String key) {
      final popup = tester.getRect(find.byType(SlashMenuPopup));
      final row = tester.getRect(find.text(key));
      expect(row.top, greaterThanOrEqualTo(popup.top));
      expect(row.bottom, lessThanOrEqualTo(popup.bottom));
    }

    testWidgets('selection scrolls down, up and wraps inside a scrollable page', (tester) async {
      final all = items();
      await tester.pumpWidget(host(0));
      for (final index in [5, 9, all.length - 1, 0, all.length - 1, 4, 0]) {
        await tester.pumpWidget(host(index));
        await tester.pump();
        expectVisible(tester, all[index].key);
      }
    });

    testWidgets('mouse wheel scrolls the menu and its newly visible row is clickable', (tester) async {
      final all = items();
      SlashItem? picked;
      await tester.pumpWidget(host(0, onSelect: (i) => picked = i));
      await tester.pump();
      final outer = tester.state<ScrollableState>(find.byType(Scrollable).first);
      final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
      await mouse.addPointer(location: tester.getCenter(find.byType(SlashMenuPopup)));
      await tester.sendEventToBinding(PointerScrollEvent(
        position: tester.getCenter(find.byType(SlashMenuPopup)),
        scrollDelta: const Offset(0, 2000),
      ));
      await tester.pumpAndSettle();
      expect(outer.position.pixels, 0, reason: 'the wheel belongs to the menu');
      expectVisible(tester, all.last.key);
      await tester.tap(find.text(all.last.key));
      expect(picked?.key, all.last.key);
      await mouse.removePointer();
    });

    testWidgets('filtering a scrolled list resets the visible results', (tester) async {
      final all = items();
      await tester.pumpWidget(host(all.length - 1));
      await tester.pump();
      await tester.pumpWidget(host(0, rows: items(query: 'pair')));
      await tester.pumpAndSettle();
      expect(find.text('/pair-programming').hitTestable(), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets('says so when the skill list could not be loaded', (tester) async {
      L10n.setLocale(MemoLocale.en);
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(body: SlashMenuPopup(items: const [], loadFailed: true, onSelect: (_) {})),
      ));
      expect(find.text(L10n.t('slash_load_failed')), findsOneWidget);
      // With commands still listed, the failure is a banner above them.
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(body: SlashMenuPopup(items: items(atStart: true).skip(fakeSkills().length).toList(), loadFailed: true, onSelect: (_) {})),
      ));
      expect(find.text(L10n.t('slash_load_failed')), findsOneWidget);
      expect(find.text('/model'), findsOneWidget);
    });

    testWidgets('says so while skills load, and when nothing matches', (tester) async {
      L10n.setLocale(MemoLocale.en);
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(body: SlashMenuPopup(items: const [], loading: true, onSelect: (_) {})),
      ));
      expect(find.text(L10n.t('slash_loading')), findsOneWidget);
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(body: SlashMenuPopup(items: const [], onSelect: (_) {})),
      ));
      expect(find.text(L10n.t('no_matching_command')), findsOneWidget);
    });

    for (final locale in MemoLocale.values) {
      testWidgets('the last row stays visible with large text in ${locale.name}', (tester) async {
        final previous = L10n.locale;
        L10n.setLocale(locale);
        addTearDown(() => L10n.setLocale(previous));
        final all = items();
        await tester.pumpWidget(MaterialApp(
          home: Scaffold(
            body: MediaQuery(
              data: const MediaQueryData(textScaler: TextScaler.linear(2)),
              child: SlashMenuPopup(
                items: all,
                selectedIndex: all.length - 1,
                onSelect: (_) {},
              ),
            ),
          ),
        ));
        await tester.pumpAndSettle();
        expectVisible(tester, all.last.key);
        expect(tester.takeException(), isNull);
      });
    }
  });
}
