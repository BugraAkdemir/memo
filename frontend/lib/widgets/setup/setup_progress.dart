/// Pure rules for the setup wizard: which steps exist, which are required and
/// which are done. Everything is computed from a [SetupSnapshot] of the real
/// application state (active provider, local models, mail accounts, the twin
/// reply model), never from a separate "progress" record, so the wizard picks
/// up where it was left, and stays right when something is changed elsewhere.
library;

/// Every step, in the order the wizard shows them.
enum SetupStepId {
  languageTheme,
  model,
  embedding,
  persona,
  mail,
  mailModel,
  decision,
  memory,
  voiceImage,
  channels,
  briefing,
  permissions,
  summary,
}

const setupStepOrder = <SetupStepId>[
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
];

/// What the wizard needs to know about the app to judge its steps.
class SetupSnapshot {
  final bool hasActiveProvider;
  final int localChatModels;
  final bool hasEmbeddingModel;
  final int mailAccounts;
  final bool mailReplyModelSet;

  /// The morning briefing is switched on.
  final bool briefingOn;

  const SetupSnapshot({
    this.hasActiveProvider = false,
    this.localChatModels = 0,
    this.hasEmbeddingModel = false,
    this.mailAccounts = 0,
    this.mailReplyModelSet = false,
    this.briefingOn = false,
  });

  bool get hasModel => hasActiveProvider || localChatModels > 0;
  bool get hasMail => mailAccounts > 0;
}

enum SetupStepState {
  /// Required and finished, or optional and finished.
  done,

  /// Required and not finished: blocks "finish".
  requiredTodo,

  /// Optional and not (known to be) done.
  optional,

  /// Does not apply right now (the reply model without a mail account).
  notApplicable,
}

bool _required(SetupStepId id, SetupSnapshot s) => switch (id) {
  SetupStepId.model => true,
  // Mail can only be answered with a model chosen for it.
  SetupStepId.mailModel => s.hasMail,
  _ => false,
};

/// Whether the snapshot satisfies the step. Steps whose result lives outside
/// the snapshot yet (later units) report false, which is harmless: they are
/// optional.
bool _complete(SetupStepId id, SetupSnapshot s) => switch (id) {
  SetupStepId.model => s.hasModel,
  SetupStepId.embedding => s.hasEmbeddingModel,
  SetupStepId.mail => s.hasMail,
  SetupStepId.mailModel => s.hasMail && s.mailReplyModelSet,
  SetupStepId.briefing => s.briefingOn,
  _ => false,
};

SetupStepState stepState(SetupStepId id, SetupSnapshot s) {
  if (id == SetupStepId.mailModel && !s.hasMail) {
    return SetupStepState.notApplicable;
  }
  if (_complete(id, s)) return SetupStepState.done;
  return _required(id, s)
      ? SetupStepState.requiredTodo
      : SetupStepState.optional;
}

List<SetupStepId> requiredSteps(SetupSnapshot s) => [
  for (final id in setupStepOrder)
    if (_required(id, s)) id,
];

List<SetupStepId> missingRequiredSteps(SetupSnapshot s) => [
  for (final id in requiredSteps(s))
    if (!_complete(id, s)) id,
];

bool canFinishSetup(SetupSnapshot s) => missingRequiredSteps(s).isEmpty;

/// The steps to show right now: the reply-model step only appears once mail is
/// connected, and always right after the mail step.
List<SetupStepId> visibleSteps(SetupSnapshot s) => [
  for (final id in setupStepOrder)
    if (stepState(id, s) != SetupStepState.notApplicable) id,
];
