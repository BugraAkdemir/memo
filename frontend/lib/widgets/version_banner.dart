import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:url_launcher/url_launcher.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/version_provider.dart';

/// A subtle, dismissable banner shown in the bottom-right corner
/// when a new Memo version is available.
class VersionBanner extends ConsumerStatefulWidget {
  const VersionBanner({super.key});

  @override
  ConsumerState<VersionBanner> createState() => _VersionBannerState();
}

class _VersionBannerState extends ConsumerState<VersionBanner>
    with SingleTickerProviderStateMixin {
  bool _dismissed = false;
  late AnimationController _animCtrl;
  late Animation<Offset> _slideAnim;
  late Animation<double> _fadeAnim;

  @override
  void initState() {
    super.initState();
    _animCtrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 400),
    );
    _slideAnim = Tween<Offset>(
      begin: const Offset(0, 0.5),
      end: Offset.zero,
    ).animate(CurvedAnimation(parent: _animCtrl, curve: Curves.easeOutCubic));
    _fadeAnim = Tween<double>(
      begin: 0,
      end: 1,
    ).animate(CurvedAnimation(parent: _animCtrl, curve: Curves.easeOutCubic));
  }

  @override
  void dispose() {
    _animCtrl.dispose();
    super.dispose();
  }

  Future<void> _dismiss() async {
    _animCtrl.reverse();
    await Future.delayed(const Duration(milliseconds: 400));
    if (!mounted) return;
    setState(() => _dismissed = true);
    // Remember dismissal for today
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(
      'version_banner_dismissed_at',
      DateTime.now().toIso8601String().substring(0, 10),
    );
  }

  Future<void> _openDownloadPage() async {
    final uri = Uri.parse('https://memo.bugradev.com');
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_dismissed) return const SizedBox.shrink();

    final versionAsync = ref.watch(versionCheckProvider);

    return versionAsync.when(
      data: (info) {
        if (!info.hasUpdate) return const SizedBox.shrink();
        return _AnimatedBanner(
          animCtrl: _animCtrl,
          slideAnim: _slideAnim,
          fadeAnim: _fadeAnim,
          current: info.current,
          latest: info.latest!,
          onDismiss: _dismiss,
          onTap: _openDownloadPage,
        );
      },
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
    );
  }
}

class _AnimatedBanner extends StatelessWidget {
  final AnimationController animCtrl;
  final Animation<Offset> slideAnim;
  final Animation<double> fadeAnim;
  final String current;
  final String latest;
  final VoidCallback onDismiss;
  final VoidCallback onTap;

  const _AnimatedBanner({
    required this.animCtrl,
    required this.slideAnim,
    required this.fadeAnim,
    required this.current,
    required this.latest,
    required this.onDismiss,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    // Start entrance animation on first frame
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (animCtrl.status == AnimationStatus.dismissed) {
        animCtrl.forward();
      }
    });

    return Positioned(
      right: 16,
      bottom: 16,
      child: SlideTransition(
        position: slideAnim,
        child: FadeTransition(
          opacity: fadeAnim,
          child: Material(
            elevation: 6,
            borderRadius: BorderRadius.circular(14),
            color: MemoTheme.of(context).bgPanel,
            surfaceTintColor: Colors.transparent,
            child: InkWell(
              borderRadius: BorderRadius.circular(14),
              onTap: onTap,
              child: Container(
                constraints: const BoxConstraints(maxWidth: 340),
                padding: const EdgeInsets.only(
                  left: 14,
                  top: 10,
                  bottom: 10,
                  right: 4,
                ),
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(14),
                  border: Border.all(
                    color: MemoTheme.accent.withValues(alpha: 0.25),
                    width: 1,
                  ),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Icon area
                    Container(
                      width: 36,
                      height: 36,
                      decoration: BoxDecoration(
                        color: MemoTheme.accent.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(10),
                      ),
                      child: Icon(
                        Icons.system_update_rounded,
                        color: MemoTheme.accent,
                        size: 20,
                      ),
                    ),
                    const SizedBox(width: 10),
                    // Text content
                    Flexible(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            L10n.t('version_new', {'v': latest}),
                            style: TextStyle(
                              fontSize: 13,
                              fontWeight: FontWeight.w700,
                              color: MemoTheme.of(context).textMain,
                              letterSpacing: -0.2,
                            ),
                          ),
                          const SizedBox(height: 2),
                          Text(
                            L10n.t('version_click_to_update'),
                            style: TextStyle(
                              fontSize: 11.5,
                              color: MemoTheme.of(context).textMuted,
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(width: 4),
                    // Close button
                    SizedBox(
                      width: 30,
                      height: 30,
                      child: IconButton(
                        padding: EdgeInsets.zero,
                        iconSize: 16,
                        icon: Icon(
                          Icons.close_rounded,
                          color: MemoTheme.of(context).textDim,
                        ),
                        onPressed: onDismiss,
                        tooltip: L10n.t('close'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
