import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:memo_flutter/core/api_client.dart';
import 'package:memo_flutter/models/agent.dart';
import 'package:memo_flutter/providers/chat_provider.dart';
import 'package:memo_flutter/providers/agent_provider.dart';
import 'package:memo_flutter/widgets/agent/permission_dialog.dart';
import 'package:memo_flutter/core/l10n.dart';
import 'package:memo_flutter/utils/tool_names.dart';

class _DisabledAgentApi extends MemoApiClient {
  _DisabledAgentApi() : super(baseUrl: 'http://memo.test');
  @override
  Future<bool> getAgentEnabled() async => false;
}

void main() {
  test('communication permission and event labels exist in both languages', () {
    final previous = L10n.locale;
    addTearDown(() => L10n.setLocale(previous));
    for (final locale in MemoLocale.values) {
      L10n.setLocale(locale);
      for (final name in [
        'mail_accounts',
        'mail_inbox',
        'mail_search',
        'mail_read',
        'mail_send',
        'mail_save_draft',
        'telegram_send',
      ]) {
        expect(ToolNames.displayName(name), isNot(name));
        expect(ToolNames.displayName(name), isNot('tool_$name'));
        expect(ToolNames.description(name), isNotEmpty);
        expect(ToolNames.description(name), isNot('tool_${name}_desc'));
      }
    }
  });

  testWidgets('mail permission is visible with Agent mode off', (tester) async {
    final container = ProviderContainer(
      overrides: [apiClientProvider.overrideWithValue(_DisabledAgentApi())],
    );
    addTearDown(container.dispose);
    container.read(isSendingProvider.notifier).state = true;
    expect(container.read(agentEnabledProvider), isFalse);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MaterialApp(
          home: Scaffold(
            body: PermissionDialog(
              event: AgentEvent(
                type: 'permission_request',
                requestId: 'mail-send',
                toolName: 'mail_send',
                dangerLevel: 'medium',
                args: '{"to":["friend@example.com"],"body":"hello"}',
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    expect(
      find.textContaining(ToolNames.displayName('mail_send')),
      findsOneWidget,
    );
    expect(find.text(L10n.t('allow_short')), findsOneWidget);
    expect(find.text('friend@example.com'), findsOneWidget);
    expect(find.text('hello'), findsOneWidget);
    expect(container.read(agentEnabledProvider), isFalse);
    await tester.pumpWidget(const SizedBox.shrink());
  });
}
