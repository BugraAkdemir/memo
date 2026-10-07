/// One model a configured provider offers, as the backend lists it live
/// (GET /api/providers/model). [ownedBy] is only set when the provider says
/// whose model it is — the Subscriptions sidecar does (antigravity, claude,
/// codex …) — and lets a selector group a long list.
class ProviderModel {
  final String id;
  final String ownedBy;

  /// Share of the model's allowance left, 0..1, when the vendor reports one
  /// (Antigravity does). Null = unknown, which is not the same as empty.
  final double? remaining;

  /// When that allowance refills (RFC 3339), if the vendor says.
  final String resetAt;

  /// Which allowance window the figure is for when the vendor has several
  /// ("5h", "7d" — the most limiting one is reported); '' when it has just one.
  final String quotaWindow;

  const ProviderModel({
    required this.id,
    this.ownedBy = '',
    this.remaining,
    this.resetAt = '',
    this.quotaWindow = '',
  });

  factory ProviderModel.fromJson(Map<String, dynamic> json) => ProviderModel(
        id: json['id'] as String? ?? '',
        ownedBy: json['owned_by'] as String? ?? '',
        remaining: (json['remaining'] as num?)?.toDouble(),
        resetAt: json['reset_at'] as String? ?? '',
        quotaWindow: json['quota_window'] as String? ?? '',
      );

  /// [remaining] as a whole percentage, or null when unknown. A sliver that
  /// would round to 0 still shows as 1 so "almost out" is not read as "out".
  int? get remainingPercent {
    final r = remaining;
    if (r == null) return null;
    final pct = (r * 100).round();
    return pct == 0 && r > 0 ? 1 : pct.clamp(0, 100);
  }
}

/// The live list for one provider plus the model it currently uses.
class ProviderModelList {
  final List<ProviderModel> models;
  final String current;

  /// Why the list could not be fetched (empty when it could).
  final String error;

  const ProviderModelList({this.models = const [], this.current = '', this.error = ''});

  factory ProviderModelList.fromJson(Map<String, dynamic> json) {
    final raw = json['models'];
    return ProviderModelList(
      models: raw is List
          ? raw
              .whereType<Map<String, dynamic>>()
              .map(ProviderModel.fromJson)
              .where((m) => m.id.isNotEmpty)
              .toList()
          : const [],
      current: json['current'] as String? ?? '',
      error: json['error'] as String? ?? '',
    );
  }
}

/// Name of the provider that fronts the bundled CLIProxyAPI sidecar — every
/// model every signed-in vendor account offers is reachable through it, so the
/// selectors expand it into that list. Must match `subsProviderName` in
/// internal/app/subs.go; it is a config Name (data), not UI text.
const String kSubscriptionsProviderName = 'Subscriptions';

/// Prefix of a selector value that means "this provider, this model" rather
/// than just "this provider". Value shape: `<prefix><provider>\u0001<model>`.
const String kModelSelectPrefix = '__model__:';

String encodeModelSelection(String provider, String model) =>
    '$kModelSelectPrefix$provider\u0001$model';

/// Splits a value made by [encodeModelSelection]; null if it is not one.
({String provider, String model})? decodeModelSelection(String value) {
  if (!value.startsWith(kModelSelectPrefix)) return null;
  final parts = value.substring(kModelSelectPrefix.length).split('\u0001');
  if (parts.length != 2 || parts[0].isEmpty || parts[1].isEmpty) return null;
  return (provider: parts[0], model: parts[1]);
}

/// Display name of a vendor key from the sidecar ("antigravity" → "Antigravity").
/// Brand names, not translatable UI text; an unknown key is shown as-is.
String vendorLabel(String ownedBy) {
  switch (ownedBy) {
    case 'antigravity':
      return 'Antigravity';
    case 'claude':
      return 'Claude';
    case 'codex':
    case 'openai': // the sidecar's Codex accounts list their models as owned by openai
      return 'Codex';
    case '':
      return '';
    default:
      return ownedBy[0].toUpperCase() + ownedBy.substring(1);
  }
}
