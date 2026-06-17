import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../providers/chat_provider.dart';

// 5 saniyede bir mood skorunu yeniler.
final moodScoreProvider = StreamProvider<double>((ref) {
  final api = ref.read(apiClientProvider);
  return Stream.periodic(const Duration(seconds: 5), (_) => 0)
      .asyncMap((_) => api.getMoodScore())
      .distinct();
});

// 5 saniyede bir mood enabled durumunu yeniler.
final moodEnabledProvider = StreamProvider<bool>((ref) {
  final api = ref.read(apiClientProvider);
  return Stream.periodic(const Duration(seconds: 5), (_) => 0)
      .asyncMap((_) => api.getMoodEnabled())
      .distinct();
});
