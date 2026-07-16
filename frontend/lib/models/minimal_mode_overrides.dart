/// Granular Minimal Mode overrides (Settings → General → Minimal Mode
/// dropdown). Each only has any effect while Minimal Mode itself is on —
/// see internal/identity/identity.go's GetMinimalModeOverrides doc comment
/// on the backend side.
class MinimalModeOverrides {
  final bool keepPersona;
  final bool keepCapabilities;
  final bool keepPassive;
  final bool keepProactive;

  const MinimalModeOverrides({
    required this.keepPersona,
    required this.keepCapabilities,
    required this.keepPassive,
    required this.keepProactive,
  });

  factory MinimalModeOverrides.fromJson(Map<String, dynamic> json) {
    return MinimalModeOverrides(
      keepPersona: json['keep_persona'] as bool? ?? false,
      keepCapabilities: json['keep_capabilities'] as bool? ?? false,
      keepPassive: json['keep_passive'] as bool? ?? false,
      keepProactive: json['keep_proactive'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() => {
        'keep_persona': keepPersona,
        'keep_capabilities': keepCapabilities,
        'keep_passive': keepPassive,
        'keep_proactive': keepProactive,
      };

  MinimalModeOverrides copyWith({
    bool? keepPersona,
    bool? keepCapabilities,
    bool? keepPassive,
    bool? keepProactive,
  }) {
    return MinimalModeOverrides(
      keepPersona: keepPersona ?? this.keepPersona,
      keepCapabilities: keepCapabilities ?? this.keepCapabilities,
      keepPassive: keepPassive ?? this.keepPassive,
      keepProactive: keepProactive ?? this.keepProactive,
    );
  }

  static const allOff = MinimalModeOverrides(
    keepPersona: false,
    keepCapabilities: false,
    keepPassive: false,
    keepProactive: false,
  );
}
