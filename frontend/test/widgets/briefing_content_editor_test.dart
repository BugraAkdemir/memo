import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/models/mail.dart';
import 'package:memo_flutter/models/provider_config.dart';
import 'package:memo_flutter/models/provider_models.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/provider_provider.dart';
import 'package:memo_flutter/widgets/briefing_content_editor.dart';

class _Api extends MemoApiClient {
  _Api() : super(baseUrl: 'http://memo.test');
  @override
  Future<ProviderModelList> listProviderModels(
    String name, {
    bool fresh = false,
  }) async => ProviderModelList.fromJson({
    'models': [
      {'id': 'fast'},
      {'id': 'smart'},
    ],
  });
}

class _OneProvider extends ProviderListNotifier {
  @override
  Future<List<ProviderConfig>> build() async => [
    const ProviderConfig(
      type: 'custom',
      name: 'writer',
      model: 'fast',
      enabled: true,
    ),
  ];
}

Future<List<Map<String, dynamic>>> pump(
  WidgetTester tester, {
  Map<String, dynamic> settings = const {},
  BriefingChannelAvailability availability =
      const BriefingChannelAvailability(),
}) async {
  tester.view.physicalSize = const Size(700, 4000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final saved = <Map<String, dynamic>>[];
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(_Api()),
        providerListProvider.overrideWith(_OneProvider.new),
      ],
      child: MaterialApp(
        home: Scaffold(
          body: SingleChildScrollView(
            child: BriefingContentEditor(
              settings: settings,
              availability: availability,
              onChanged: saved.add,
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return saved;
}

bool checked(WidgetTester tester, String key) =>
    tester.widget<CheckboxListTile>(find.byKey(ValueKey(key))).value!;

void main() {
  for (final locale in MemoLocale.values) {
    group('briefing content editor ($locale)', () {
      setUp(() => L10n.setLocale(locale));
      tearDown(() => L10n.setLocale(MemoLocale.en));

      testWidgets('a briefing saved before sections existed shows all on', (
        tester,
      ) async {
        await pump(tester);
        for (final id in briefingSectionIds) {
          expect(checked(tester, 'briefing-section-$id'), isTrue, reason: id);
        }
        expect(find.text(L10n.t('briefing_sections_title')), findsOneWidget);
      });

      testWidgets(
        'unchecking a section saves an explicit list and keeps every other '
        'setting',
        (tester) async {
          final saved = await pump(
            tester,
            settings: {
              'morning': true,
              'morning_time': '07:30',
              'channels': ['telegram'],
            },
          );
          await tester.tap(find.byKey(const ValueKey('briefing-section-code')));
          await tester.pump();
          expect(saved.single['sections'], [
            'meetings',
            'overnight',
            'drafts',
            'waiting',
            'promises',
          ]);
          expect(saved.single['morning_time'], '07:30');
          expect(saved.single['channels'], ['telegram']);
        },
      );

      testWidgets('custom items are added, listed and removed', (tester) async {
        final saved = await pump(
          tester,
          settings: {
            'custom': [
              {'title': 'Water', 'prompt': 'Drink a glass'},
            ],
          },
        );
        expect(find.text('Water'), findsOneWidget);
        // Add stays off until both fields have text.
        FilledButton add() => tester.widget<FilledButton>(
          find.byKey(const ValueKey('briefing-custom-add')),
        );
        expect(add().onPressed, isNull);
        await tester.enterText(
          find.byKey(const ValueKey('briefing-custom-title')),
          ' Focus ',
        );
        await tester.pump();
        expect(add().onPressed, isNull);
        await tester.enterText(
          find.byKey(const ValueKey('briefing-custom-prompt')),
          'What first?',
        );
        await tester.pump();
        await tester.tap(find.byKey(const ValueKey('briefing-custom-add')));
        await tester.pump();
        expect(saved.last['custom'], [
          {'title': 'Water', 'prompt': 'Drink a glass'},
          {'title': 'Focus', 'prompt': 'What first?'},
        ]);
        await tester.tap(
          find.byKey(const ValueKey('briefing-custom-remove-0')),
        );
        await tester.pump();
        expect(saved.last['custom'], isEmpty);
      });

      testWidgets('five items hide the add form', (tester) async {
        await pump(
          tester,
          settings: {
            'custom': [
              for (var i = 0; i < briefingMaxCustom; i++)
                {'title': 'T$i', 'prompt': 'P$i'},
            ],
          },
        );
        expect(find.byKey(const ValueKey('briefing-custom-add')), findsNothing);
        expect(find.text(L10n.t('briefing_custom_limit')), findsOneWidget);
      });

      testWidgets(
        'a channel that is not set up cannot be switched on, but one already '
        'on can be switched off',
        (tester) async {
          final saved = await pump(
            tester,
            settings: {
              'channels': ['whatsapp'],
            },
            availability: const BriefingChannelAvailability(telegram: true),
          );
          // Not connected and not chosen: disabled, with the reason shown.
          expect(
            tester
                .widget<CheckboxListTile>(
                  find.byKey(const ValueKey('briefing-channel-mail')),
                )
                .onChanged,
            isNull,
          );
          expect(
            find.text(L10n.t('briefing_channel_mail_missing')),
            findsOneWidget,
          );
          // Chosen earlier but no longer connected: it can still be removed.
          await tester.tap(
            find.byKey(const ValueKey('briefing-channel-whatsapp')),
          );
          await tester.pump();
          expect(saved.last['channels'], isEmpty);
          // A ready one switches on, next to what was already chosen.
          await tester.tap(
            find.byKey(const ValueKey('briefing-channel-telegram')),
          );
          await tester.pump();
          expect(saved.last['channels'], ['whatsapp', 'telegram']);
        },
      );

      testWidgets(
        'the mail account choice appears only with several accounts',
        (tester) async {
          final accounts = [
            const MailAccount(id: 'a', email: 'a@example.com'),
            const MailAccount(id: 'b', email: 'b@example.com'),
          ];
          final saved = await pump(
            tester,
            settings: {
              'channels': ['mail'],
            },
            availability: BriefingChannelAvailability(mailAccounts: accounts),
          );
          await tester.tap(find.byType(DropdownButtonFormField<String>).first);
          await tester.pumpAndSettle();
          await tester.tap(find.text('b@example.com').last);
          await tester.pumpAndSettle();
          expect(saved.last['mail_account'], 'b');
        },
      );

      testWidgets('one mail account needs no choice', (tester) async {
        await pump(
          tester,
          settings: {
            'channels': ['mail'],
          },
          availability: const BriefingChannelAvailability(
            mailAccounts: [MailAccount(id: 'a', email: 'a@example.com')],
          ),
        );
        expect(find.text(L10n.t('briefing_mail_account')), findsNothing);
      });

      testWidgets('the briefing model is saved on its own, keeping the rest', (
        tester,
      ) async {
        final saved = await pump(tester, settings: {'morning': true});
        expect(find.text(L10n.t('briefing_model_title')), findsOneWidget);
        // The first dropdown of the picker is the provider, the last the model.
        final dropdowns = find.byType(DropdownButtonFormField<String>);
        await tester.tap(dropdowns.first);
        await tester.pumpAndSettle();
        await tester.tap(find.text('writer').last);
        await tester.pumpAndSettle();
        await tester.tap(dropdowns.last);
        await tester.pumpAndSettle();
        await tester.tap(find.text('smart').last);
        await tester.pumpAndSettle();
        await tester.ensureVisible(
          find.byKey(const Key('briefing_model_save')),
        );
        await tester.tap(find.byKey(const Key('briefing_model_save')));
        await tester.pumpAndSettle();
        expect(saved.last['model'], {'provider': 'writer', 'model': 'smart'});
        expect(saved.last['morning'], true);
      });

      testWidgets('an existing model is shown', (tester) async {
        await pump(
          tester,
          settings: {
            'model': {'provider': 'writer', 'model': 'fast'},
          },
        );
        expect(find.text('writer'), findsWidgets);
        expect(find.text('fast'), findsWidgets);
      });
    });
  }

  test('time options cover the day and keep an unusual current value', () {
    final times = briefingTimeOptions('06:45');
    expect(times.first, '00:00');
    expect(times.last, '23:30');
    expect(times, contains('06:45'));
    expect(times.length, 49);
    expect(times.toSet().length, times.length);
  });
}
