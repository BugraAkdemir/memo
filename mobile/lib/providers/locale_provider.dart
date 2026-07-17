import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../core/l10n.dart';

/// Persisted UI language for Memo Mobile (`memo_locale`: `tr` | `en`).
/// Mirrors desktop `localeProvider` in `frontend/lib/providers/settings_provider.dart`.
final localeProvider =
    StateNotifierProvider<LocaleNotifier, MemoLocale>((ref) {
  return LocaleNotifier();
});

class LocaleNotifier extends StateNotifier<MemoLocale> {
  LocaleNotifier() : super(MemoLocale.tr) {
    _load();
  }

  static const _prefsKey = 'memo_locale';

  Future<void> _load() async {
    final prefs = await SharedPreferences.getInstance();
    final saved = prefs.getString(_prefsKey);
    final locale = saved == 'en' ? MemoLocale.en : MemoLocale.tr;
    L10n.setLocale(locale);
    if (state != locale) state = locale;
  }

  Future<void> setLocale(MemoLocale locale) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_prefsKey, locale == MemoLocale.en ? 'en' : 'tr');
    L10n.setLocale(locale);
    state = locale;
  }
}
