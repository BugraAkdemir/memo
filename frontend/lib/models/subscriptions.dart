import 'provider_models.dart';

/// One signed-in vendor account (antigravity, claude, codex). Identity only —
/// the backend never sends a token.
class SubscriptionAccount {
  final String provider;
  final String email;
  final String project;
  final bool disabled;

  const SubscriptionAccount({
    required this.provider,
    this.email = '',
    this.project = '',
    this.disabled = false,
  });

  factory SubscriptionAccount.fromJson(Map<String, dynamic> json) => SubscriptionAccount(
        provider: json['provider'] as String? ?? '',
        email: json['email'] as String? ?? '',
        project: json['project'] as String? ?? '',
        disabled: json['disabled'] as bool? ?? false,
      );
}

/// The browser sign-in in flight (or the last one finished).
class SubscriptionLogin {
  final String provider;
  final bool running;
  final String url;
  final bool done;
  final String error;

  const SubscriptionLogin({
    this.provider = '',
    this.running = false,
    this.url = '',
    this.done = false,
    this.error = '',
  });

  factory SubscriptionLogin.fromJson(Map<String, dynamic> json) => SubscriptionLogin(
        provider: json['provider'] as String? ?? '',
        running: json['running'] as bool? ?? false,
        url: json['url'] as String? ?? '',
        done: json['done'] as bool? ?? false,
        error: json['error'] as String? ?? '',
      );
}

/// What Settings → Subscriptions renders — mirrors models.SubscriptionsState
/// (GET /api/subscriptions). The sidecar is bundled with the app (never
/// downloaded at run time), so [bundled] is false only for a build that ships
/// without it, and [problem] then says why.
class SubscriptionsState {
  final bool bundled;
  final String version;
  final String problem;
  final bool running;
  final List<String> providers;
  final List<SubscriptionAccount> accounts;
  final List<ProviderModel> models;
  final String model;
  final SubscriptionLogin login;

  const SubscriptionsState({
    this.bundled = false,
    this.version = '',
    this.problem = '',
    this.running = false,
    this.providers = const [],
    this.accounts = const [],
    this.models = const [],
    this.model = '',
    this.login = const SubscriptionLogin(),
  });

  factory SubscriptionsState.fromJson(Map<String, dynamic> json) {
    List<T> list<T>(String key, T Function(Map<String, dynamic>) parse) {
      final raw = json[key];
      return raw is List ? raw.whereType<Map<String, dynamic>>().map(parse).toList() : <T>[];
    }

    final rawProviders = json['providers'];
    final login = json['login'];
    return SubscriptionsState(
      bundled: json['bundled'] as bool? ?? false,
      version: json['version'] as String? ?? '',
      problem: json['problem'] as String? ?? '',
      running: json['running'] as bool? ?? false,
      providers: rawProviders is List ? rawProviders.whereType<String>().toList() : const [],
      accounts: list('accounts', SubscriptionAccount.fromJson),
      models: list('models', ProviderModel.fromJson),
      model: json['model'] as String? ?? '',
      login: login is Map<String, dynamic> ? SubscriptionLogin.fromJson(login) : const SubscriptionLogin(),
    );
  }

  /// Accounts signed in under [provider].
  List<SubscriptionAccount> accountsFor(String provider) =>
      accounts.where((a) => a.provider == provider).toList();
}
