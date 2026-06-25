import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../providers/chat_provider.dart';

// 10 saniyede bir mood skorunu yeniler.
final moodScoreProvider = StreamProvider.autoDispose<double>((ref) {
  final api = ref.read(apiClientProvider);
  return Stream.periodic(const Duration(seconds: 10), (_) => 0)
      .asyncMap((_) => api.getMoodScore())
      .distinct();
});

// 10 saniyede bir mood enabled durumunu yeniler.
final moodEnabledProvider = StreamProvider.autoDispose<bool>((ref) {
  final api = ref.read(apiClientProvider);
  return Stream.periodic(const Duration(seconds: 10), (_) => 0)
      .asyncMap((_) => api.getMoodEnabled())
      .distinct();
});
