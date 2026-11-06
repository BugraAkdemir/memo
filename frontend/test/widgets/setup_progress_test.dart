import 'package:flutter_test/flutter_test.dart';
import 'package:memo_flutter/widgets/setup/setup_progress.dart';

void main() {
  group('required steps', () {
    test(
      'a model connection is always required, nothing else is by default',
      () {
        const s = SetupSnapshot();
        expect(requiredSteps(s), [SetupStepId.model]);
        expect(canFinishSetup(s), isFalse);
        expect(missingRequiredSteps(s), [SetupStepId.model]);
      },
    );

    test('an API/subscription provider satisfies the model step', () {
      const s = SetupSnapshot(hasActiveProvider: true);
      expect(stepState(SetupStepId.model, s), SetupStepState.done);
      expect(canFinishSetup(s), isTrue);
    });

    test('a downloaded local chat model satisfies it too', () {
      const s = SetupSnapshot(localChatModels: 1);
      expect(stepState(SetupStepId.model, s), SetupStepState.done);
      expect(canFinishSetup(s), isTrue);
    });

    test('an embedding model alone is not a chat model', () {
      const s = SetupSnapshot(hasEmbeddingModel: true);
      expect(canFinishSetup(s), isFalse);
    });
  });

  group('mail makes the reply model required', () {
    test('no mail account: the reply-model step does not apply', () {
      const s = SetupSnapshot(hasActiveProvider: true);
      expect(stepState(SetupStepId.mailModel, s), SetupStepState.notApplicable);
      expect(requiredSteps(s), isNot(contains(SetupStepId.mailModel)));
    });

    test('a connected account without a reply model blocks finishing', () {
      const s = SetupSnapshot(hasActiveProvider: true, mailAccounts: 1);
      expect(requiredSteps(s), contains(SetupStepId.mailModel));
      expect(stepState(SetupStepId.mailModel, s), SetupStepState.requiredTodo);
      expect(canFinishSetup(s), isFalse);
      expect(missingRequiredSteps(s), [SetupStepId.mailModel]);
    });

    test('choosing the reply model unblocks it', () {
      const s = SetupSnapshot(
        hasActiveProvider: true,
        mailAccounts: 2,
        mailReplyModelSet: true,
      );
      expect(stepState(SetupStepId.mailModel, s), SetupStepState.done);
      expect(canFinishSetup(s), isTrue);
    });

    test('a reply model chosen earlier does not matter once mail is gone', () {
      const s = SetupSnapshot(
        hasActiveProvider: true,
        mailAccounts: 0,
        mailReplyModelSet: true,
      );
      expect(stepState(SetupStepId.mailModel, s), SetupStepState.notApplicable);
      expect(canFinishSetup(s), isTrue);
    });
  });

  group('optional steps never block', () {
    test('embedding and mail report done/optional but are never required', () {
      const none = SetupSnapshot(hasActiveProvider: true);
      expect(stepState(SetupStepId.embedding, none), SetupStepState.optional);
      expect(stepState(SetupStepId.mail, none), SetupStepState.optional);
      const some = SetupSnapshot(
        hasActiveProvider: true,
        hasEmbeddingModel: true,
        mailAccounts: 1,
        mailReplyModelSet: true,
      );
      expect(stepState(SetupStepId.embedding, some), SetupStepState.done);
      expect(stepState(SetupStepId.mail, some), SetupStepState.done);
      expect(canFinishSetup(none), isTrue);
    });
  });

  test('the step order matches the plan and ids are unique', () {
    expect(setupStepOrder, [
      SetupStepId.languageTheme,
      SetupStepId.model,
      SetupStepId.embedding,
      SetupStepId.persona,
      SetupStepId.mail,
      SetupStepId.mailModel,
      SetupStepId.decision,
      SetupStepId.memory,
      SetupStepId.voiceImage,
      SetupStepId.channels,
      SetupStepId.briefing,
      SetupStepId.permissions,
      SetupStepId.summary,
    ]);
    expect(setupStepOrder.toSet().length, setupStepOrder.length);
  });

  test('visible steps hide the reply-model step until mail is connected', () {
    const noMail = SetupSnapshot(hasActiveProvider: true);
    expect(visibleSteps(noMail), isNot(contains(SetupStepId.mailModel)));
    const mail = SetupSnapshot(hasActiveProvider: true, mailAccounts: 1);
    final steps = visibleSteps(mail);
    expect(steps, contains(SetupStepId.mailModel));
    expect(
      steps.indexOf(SetupStepId.mailModel),
      steps.indexOf(SetupStepId.mail) + 1,
      reason: 'the reply model is asked right after connecting mail',
    );
  });

  test(
    'the briefing step is optional, always shown and done once it is on',
    () {
      const off = SetupSnapshot(hasActiveProvider: true);
      const on = SetupSnapshot(hasActiveProvider: true, briefingOn: true);
      expect(visibleSteps(off), contains(SetupStepId.briefing));
      expect(requiredSteps(off), isNot(contains(SetupStepId.briefing)));
      expect(stepState(SetupStepId.briefing, off), SetupStepState.optional);
      expect(stepState(SetupStepId.briefing, on), SetupStepState.done);
      expect(
        canFinishSetup(off),
        isTrue,
        reason: 'a skipped briefing never blocks',
      );
      final order = visibleSteps(off);
      expect(
        order.indexOf(SetupStepId.briefing),
        order.indexOf(SetupStepId.channels) + 1,
        reason: 'asked right after WhatsApp and Telegram are connected',
      );
    },
  );
}
