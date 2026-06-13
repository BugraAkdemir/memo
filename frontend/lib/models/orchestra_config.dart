import 'package:flutter/material.dart';

/// Orchestra mode configuration — mirrors Go `orchestra.OrchestraConfig`
class OrchestraConfig {
  final bool enabled;
  final String chiefType;
  final String chiefModel;
  final List<RoleConfig> roles;

  const OrchestraConfig({
    this.enabled = false,
    this.chiefType = 'claude',
    this.chiefModel = 'claude-sonnet-4-20250514',
    this.roles = const [],
  });

  factory OrchestraConfig.fromJson(Map<String, dynamic> json) {
    return OrchestraConfig(
      enabled: json['enabled'] as bool? ?? false,
      chiefType: json['chief_type'] as String? ?? 'claude',
      chiefModel: json['chief_model'] as String? ?? 'claude-sonnet-4-20250514',
      roles: (json['roles'] as List? ?? [])
          .map((e) => RoleConfig.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'enabled': enabled,
      'chief_type': chiefType,
      'chief_model': chiefModel,
      'roles': roles.map((r) => r.toJson()).toList(),
    };
  }

  OrchestraConfig copyWith({
    bool? enabled,
    String? chiefType,
    String? chiefModel,
    List<RoleConfig>? roles,
  }) {
    return OrchestraConfig(
      enabled: enabled ?? this.enabled,
      chiefType: chiefType ?? this.chiefType,
      chiefModel: chiefModel ?? this.chiefModel,
      roles: roles ?? this.roles,
    );
  }
}

/// Role configuration for a specialist.
class RoleConfig {
  final String role;
  final bool enabled;
  final String modelType;
  final String modelName;
  final String systemPrompt;

  const RoleConfig({
    required this.role,
    this.enabled = false,
    this.modelType = '',
    this.modelName = '',
    this.systemPrompt = '',
  });

  factory RoleConfig.fromJson(Map<String, dynamic> json) {
    return RoleConfig(
      role: json['role'] as String? ?? '',
      enabled: json['enabled'] as bool? ?? false,
      modelType: json['model_type'] as String? ?? '',
      modelName: json['model_name'] as String? ?? '',
      systemPrompt: json['system_prompt'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'role': role,
      'enabled': enabled,
      'model_type': modelType,
      'model_name': modelName,
      'system_prompt': systemPrompt,
    };
  }

  RoleConfig copyWith({
    String? role,
    bool? enabled,
    String? modelType,
    String? modelName,
    String? systemPrompt,
  }) {
    return RoleConfig(
      role: role ?? this.role,
      enabled: enabled ?? this.enabled,
      modelType: modelType ?? this.modelType,
      modelName: modelName ?? this.modelName,
      systemPrompt: systemPrompt ?? this.systemPrompt,
    );
  }
}

/// Default orchestra configuration.
class OrchestraDefaults {
  static const List<Map<String, String>> defaultRoles = [
    {'role': 'planner', 'model_type': 'claude', 'model_name': 'claude-sonnet-4-20250514', 'label': 'Planner'},
    {'role': 'frontend', 'model_type': 'grok', 'model_name': 'grok-2', 'label': 'Frontend'},
    {'role': 'backend', 'model_type': 'openai', 'model_name': 'gpt-4o', 'label': 'Backend'},
    {'role': 'bug_fixer', 'model_type': 'gemini', 'model_name': 'gemini-2.0-flash', 'label': 'Bug Fixer'},
    {'role': 'reviewer', 'model_type': 'claude', 'model_name': 'claude-sonnet-4-20250514', 'label': 'Reviewer'},
    {'role': 'security', 'model_type': 'openai', 'model_name': 'gpt-4o', 'label': 'Security'},
    {'role': 'devops', 'model_type': 'grok', 'model_name': 'grok-2', 'label': 'DevOps'},
    {'role': 'general', 'model_type': 'openai', 'model_name': 'gpt-4o', 'label': 'General'},
  ];

  static IconData iconForRole(String role) {
    switch (role) {
      case 'planner':
        return Icons.account_tree_outlined;
      case 'frontend':
        return Icons.dashboard_outlined;
      case 'backend':
        return Icons.dns_outlined;
      case 'bug_fixer':
        return Icons.bug_report_outlined;
      case 'reviewer':
        return Icons.rate_review_outlined;
      case 'security':
        return Icons.shield_outlined;
      case 'devops':
        return Icons.rocket_launch_outlined;
      case 'general':
        return Icons.smart_toy_outlined;
      default:
        return Icons.extension_outlined;
    }
  }

  static String labelForRole(String role) {
    for (final r in defaultRoles) {
      if (r['role'] == role) return r['label']!;
    }
    return role;
  }
}
