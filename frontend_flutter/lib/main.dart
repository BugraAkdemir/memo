import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'core/theme.dart';
import 'core/l10n.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(const ProviderScope(child: MemoApp()));
}

class MemoApp extends StatelessWidget {
  const MemoApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Memo',
      debugShowCheckedModeBanner: false,
      theme: MemoTheme.themeData,
      home: const _PlaceholderHome(),
    );
  }
}

/// Temporary placeholder until Faz 4 screens are built.
class _PlaceholderHome extends StatelessWidget {
  const _PlaceholderHome();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: MemoTheme.bgApp,
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 80,
              height: 80,
              decoration: BoxDecoration(
                color: MemoTheme.accentPale,
                borderRadius: BorderRadius.circular(MemoTheme.radiusLg),
                border: Border.all(color: MemoTheme.accent, width: 2),
              ),
              child: const Center(
                child: Text(
                  'M',
                  style: TextStyle(
                    fontSize: 36,
                    fontWeight: FontWeight.bold,
                    color: MemoTheme.accent,
                  ),
                ),
              ),
            ),
            const SizedBox(height: 24),
            Text(
              L10n.t('app_title'),
              style: Theme.of(context).textTheme.headlineLarge?.copyWith(
                    fontWeight: FontWeight.bold,
                    color: MemoTheme.textMain,
                  ),
            ),
            const SizedBox(height: 8),
            Text(
              L10n.t('app_subtitle'),
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    color: MemoTheme.textDim,
                  ),
            ),
            const SizedBox(height: 48),
            Text(
              'Flutter Core ✅ — Ekranlar yapılıyor...',
              style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                    color: MemoTheme.textMuted,
                  ),
            ),
          ],
        ),
      ),
    );
  }
}
