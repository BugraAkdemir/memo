/// Live Mode's top-level engine selector — mirrors Go `config.LiveModeConfig`
/// (internal/config/config.go). Deliberately just the four scalar fields
/// this phase needs; per-engine credentials/model/voice choices are a later
/// phase's own model (see docs/plans/PLAN_live_mode_v2.md, Phase 3+).
class LiveModeConfig {
  final bool enabled;
  final String activeEngine; // "local" | "google_live" | "openai_realtime" | "elevenlabs" | "custom"
  final String workMode; // "delegate" | "standalone" — meaningful only for google_live/openai_realtime
  final String agentPermissionPolicy; // "voice_prompt" | "auto_allow_once"

  const LiveModeConfig({
    this.enabled = false,
    this.activeEngine = 'local',
    this.workMode = 'delegate',
    this.agentPermissionPolicy = 'voice_prompt',
  });

  factory LiveModeConfig.fromJson(Map<String, dynamic> json) {
    return LiveModeConfig(
      enabled: json['enabled'] as bool? ?? false,
      activeEngine: json['active_engine'] as String? ?? 'local',
      workMode: json['work_mode'] as String? ?? 'delegate',
      agentPermissionPolicy:
          json['agent_permission_policy'] as String? ?? 'voice_prompt',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'enabled': enabled,
      'active_engine': activeEngine,
      'work_mode': workMode,
      'agent_permission_policy': agentPermissionPolicy,
    };
  }

  LiveModeConfig copyWith({
    bool? enabled,
    String? activeEngine,
    String? workMode,
    String? agentPermissionPolicy,
  }) {
    return LiveModeConfig(
      enabled: enabled ?? this.enabled,
      activeEngine: activeEngine ?? this.activeEngine,
      workMode: workMode ?? this.workMode,
      agentPermissionPolicy:
          agentPermissionPolicy ?? this.agentPermissionPolicy,
    );
  }
}

/// Live Mode engine types the backend knows about
/// (internal/livemode.EngineType, Phase 3+). "local" is always implemented
/// (today's existing Whisper+Piper pipeline) — the other four are wired in
/// phase by phase per PLAN_live_mode_v2.md.
class LiveModeEngineDefaults {
  static const Map<String, String> displayNames = {
    'local': 'Local',
    'google_live': 'Google Live',
    'openai_realtime': 'OpenAI',
    'elevenlabs': 'ElevenLabs',
    'custom': 'Custom',
  };

  static const List<String> implementedEngines = ['local'];

  /// Engines with a genuine native audio-to-audio reasoning model that can
  /// itself act as the "live model" — WorkMode only means something for
  /// these two (see LiveModeConfig.workMode's doc comment).
  static const List<String> nativeReasoningEngines = [
    'google_live',
    'openai_realtime',
  ];
}
