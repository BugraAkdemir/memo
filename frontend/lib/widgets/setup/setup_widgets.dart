import 'package:flutter/material.dart';

import '../../core/l10n.dart';
import '../../core/theme.dart';
import '../../models/local_model.dart';

class SetupPage extends StatelessWidget {
  final String title;
  final String? subtitle;
  final Widget child;
  final ThemeColors color;
  const SetupPage({
    super.key,
    required this.title,
    this.subtitle,
    required this.child,
    required this.color,
  });
  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      Text(
        title,
        style: TextStyle(
          fontSize: 26,
          height: 1.2,
          fontWeight: FontWeight.w700,
          color: color.textMain,
        ),
      ),
      if (subtitle != null) ...[
        const SizedBox(height: 10),
        Text(
          subtitle!,
          style: TextStyle(fontSize: 13, height: 1.5, color: color.textDim),
        ),
      ],
      const SizedBox(height: 24),
      child,
    ],
  );
}

class SetupConnectionChoice extends StatelessWidget {
  final ThemeColors color;
  final bool selected;
  final IconData icon;
  final String title, description;
  final VoidCallback onTap;
  const SetupConnectionChoice({
    super.key,
    required this.color,
    required this.selected,
    required this.icon,
    required this.title,
    required this.description,
    required this.onTap,
  });
  @override
  Widget build(BuildContext context) => Semantics(
    selected: selected,
    button: true,
    child: Material(
      color: selected ? MemoTheme.accent.withValues(alpha: .10) : color.bgApp,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: BorderSide(color: selected ? MemoTheme.accent : color.borderSoft),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(icon, color: MemoTheme.accent, size: 24),
                  const Spacer(),
                  Icon(
                    selected ? Icons.check_circle : Icons.circle_outlined,
                    size: 18,
                    color: selected ? MemoTheme.accent : color.textDim,
                  ),
                ],
              ),
              const SizedBox(height: 14),
              Text(
                title,
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: color.textMain,
                ),
              ),
              const SizedBox(height: 6),
              Text(
                description,
                style: TextStyle(
                  fontSize: 12,
                  height: 1.5,
                  color: color.textDim,
                ),
              ),
            ],
          ),
        ),
      ),
    ),
  );
}

/// A small "?" affordance that explains a term or control on hover — for
/// someone who's never run a local AI model before, "chat model" vs "memory
/// model" or "API provider" aren't self-explanatory just from a label.
class SetupInfoTip extends StatelessWidget {
  final String message;
  final ThemeColors color;

  const SetupInfoTip({super.key, required this.message, required this.color});

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: message,
      waitDuration: const Duration(milliseconds: 300),
      child: Icon(Icons.help_outline_rounded, size: 14, color: color.textDim),
    );
  }
}

class SetupCard extends StatelessWidget {
  final Widget child;
  final ThemeColors color;

  const SetupCard({super.key, required this.child, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: color.bgApp,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: color.borderSoft),
      ),
      child: child,
    );
  }
}

class SetupLabel extends StatelessWidget {
  final String text;
  final ThemeColors color;

  const SetupLabel({super.key, required this.text, required this.color});

  @override
  Widget build(BuildContext context) {
    return Text(
      text,
      style: TextStyle(
        fontWeight: FontWeight.w600,
        fontSize: 13,
        color: color.textMain,
      ),
    );
  }
}

class SetupPill extends StatelessWidget {
  final String label;
  final bool selected;
  final Color accent;
  final ThemeColors color;
  final VoidCallback onTap;

  const SetupPill({
    super.key,
    required this.label,
    required this.selected,
    required this.accent,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Semantics(
      selected: selected,
      child: OutlinedButton(
        onPressed: onTap,
        style: OutlinedButton.styleFrom(
          backgroundColor: selected
              ? accent.withValues(alpha: .10)
              : color.bgElement,
          foregroundColor: selected ? accent : color.textSecondary,
          side: BorderSide(color: selected ? accent : color.borderSoft),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(10),
          ),
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        ),
        child: Text(
          label,
          style: TextStyle(
            fontSize: 13,
            fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
          ),
        ),
      ),
    );
  }
}

class SetupPreferenceToggle extends StatelessWidget {
  final IconData icon;
  final String title;
  final String desc;
  final bool value;
  final ThemeColors color;
  final ValueChanged<bool>? onChanged;

  const SetupPreferenceToggle({
    super.key,
    required this.icon,
    required this.title,
    required this.desc,
    required this.value,
    required this.color,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: color.bgApp,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: color.borderSoft),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 32,
            height: 32,
            decoration: BoxDecoration(
              color: MemoTheme.accent.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Icon(icon, size: 16, color: MemoTheme.accent),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: color.textMain,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  desc,
                  style: TextStyle(
                    fontSize: 11.5,
                    height: 1.4,
                    color: color.textDim,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Switch(
            value: value,
            onChanged: onChanged,
            activeThumbColor: MemoTheme.accent,
          ),
        ],
      ),
    );
  }
}

class SetupSpecRow extends StatelessWidget {
  final String label;
  final String value;
  final ThemeColors color;
  final bool bold;
  final String? tooltip;

  const SetupSpecRow({
    super.key,
    required this.label,
    required this.value,
    required this.color,
    this.bold = false,
    this.tooltip,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 100,
          child: Row(
            children: [
              Flexible(
                child: Text(
                  label,
                  style: TextStyle(fontSize: 12, color: color.textDim),
                ),
              ),
              if (tooltip != null) ...[
                SizedBox(width: 4),
                SetupInfoTip(message: tooltip!, color: color),
              ],
            ],
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: TextStyle(
              fontSize: 12,
              fontWeight: bold ? FontWeight.w600 : FontWeight.normal,
              color: color.textMain,
            ),
          ),
        ),
      ],
    );
  }
}

class SetupDownloadProgressRow extends StatelessWidget {
  final DownloadProgress progress;
  final ThemeColors color;

  const SetupDownloadProgressRow({
    super.key,
    required this.progress,
    required this.color,
  });

  /// Parses the backend's already-formatted speed string (e.g. "3.2 MB/s",
  /// from Go's `formatSpeed`) back into bytes/sec, then estimates time
  /// remaining from `totalBytes - downloaded`. A non-technical user staring
  /// at a raw percentage with no sense of whether this takes 30 seconds or
  /// 30 minutes is the #1 reason downloads get abandoned mid-way.
  String? _etaLabel() {
    final p = progress;
    if (p.speed.isEmpty || p.totalBytes <= 0) return null;
    final match = RegExp(r'([\d.]+)\s*(B|KB|MB|GB)/s').firstMatch(p.speed);
    if (match == null) return null;
    final value = double.tryParse(match.group(1)!);
    if (value == null || value <= 0) return null;
    final unit = match.group(2);
    final bytesPerSec = switch (unit) {
      'GB' => value * 1024 * 1024 * 1024,
      'MB' => value * 1024 * 1024,
      'KB' => value * 1024,
      _ => value,
    };
    final remainingBytes = p.totalBytes - p.downloaded;
    if (remainingBytes <= 0 || bytesPerSec <= 0) return null;
    final seconds = remainingBytes / bytesPerSec;
    if (seconds < 60) {
      return L10n.t('setup_eta_seconds', {'s': '${seconds.round()}'});
    }
    final minutes = (seconds / 60).round();
    return L10n.t('setup_eta_minutes', {'m': '$minutes'});
  }

  @override
  Widget build(BuildContext context) {
    final p = progress;
    final pct = p.percent.clamp(0, 100).toDouble();
    final label = p.filename.isNotEmpty
        ? p.filename
        : L10n.t('setup_preparing');
    final speedSuffix = p.speed.isNotEmpty ? ' (${p.speed})' : '';
    final eta = _etaLabel();
    final etaSuffix = eta != null ? ' — $eta' : '';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(4),
          child: LinearProgressIndicator(
            value: pct > 0 ? pct / 100 : null,
            minHeight: 6,
            backgroundColor: color.bgElement,
            color: MemoTheme.accent,
          ),
        ),
        SizedBox(height: 6),
        Text(
          '$label — %${pct.toStringAsFixed(0)}$speedSuffix$etaSuffix',
          style: TextStyle(fontSize: 11, color: color.textDim),
        ),
      ],
    );
  }
}

class SetupDiagRow extends StatelessWidget {
  final String title;
  final bool ok;
  final bool optional;
  final ThemeColors color;
  final String? tooltip;

  const SetupDiagRow({
    super.key,
    required this.title,
    required this.ok,
    this.optional = false,
    required this.color,
    this.tooltip,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(
          ok
              ? Icons.check_circle_rounded
              : optional
              ? Icons.remove_circle_outline
              : Icons.error_outline_rounded,
          color: ok
              ? MemoTheme.green
              : optional
              ? color.textDim
              : MemoTheme.red,
          size: 20,
        ),
        SizedBox(width: 10),
        Expanded(
          child: Text(
            title,
            style: TextStyle(
              fontSize: 13,
              height: 1.4,
              color: ok
                  ? color.textMain
                  : optional
                  ? color.textDim
                  : MemoTheme.red,
            ),
          ),
        ),
        if (tooltip != null) ...[
          SizedBox(width: 6),
          SetupInfoTip(message: tooltip!, color: color),
        ],
      ],
    );
  }
}

/// A short visible note above a step's controls (unlike [SetupInfoTip], which
/// is only an icon with a tooltip).
class SetupNote extends StatelessWidget {
  final String message;
  final ThemeColors color;

  const SetupNote({super.key, required this.message, required this.color});

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(top: 2),
          child: Icon(Icons.info_outline, size: 16, color: color.textDim),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            message,
            style: TextStyle(color: color.textDim, fontSize: 12, height: 1.5),
          ),
        ),
      ],
    );
  }
}
