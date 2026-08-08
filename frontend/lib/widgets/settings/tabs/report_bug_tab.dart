import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/chat_provider.dart';
import '../../../core/friendly_error.dart';

/// Manual, explicit bug reporting: the user writes what happened and can
/// optionally attach the last 10 backend error events (from the existing
/// event ring, GET /api/events). Nothing is ever sent automatically or in
/// the background — submitting opens a prefilled GitHub "new issue" page in
/// the browser, so the report is only actually transmitted once the user
/// reviews it there and clicks GitHub's own submit button. No screenshot is
/// attached (screen content could contain private chat text). Deliberately
/// does not use any bugradev-controlled backend, so "we collect zero data"
/// stays true — the report goes to GitHub, sent by the user's own account,
/// not to us.
class ReportBugTab extends ConsumerStatefulWidget {
  const ReportBugTab({super.key});

  @override
  ConsumerState<ReportBugTab> createState() => _ReportBugTabState();
}

class _ReportBugTabState extends ConsumerState<ReportBugTab> {
  final _descCtrl = TextEditingController();
  bool _includeErrors = false;
  bool _sending = false;

  @override
  void dispose() {
    _descCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final desc = _descCtrl.text.trim();
    if (desc.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('report_bug_empty_error'))),
      );
      return;
    }

    setState(() => _sending = true);
    try {
      final buffer = StringBuffer(desc);

      if (_includeErrors) {
        List<Map<String, dynamic>> events = [];
        try {
          events = await ref.read(apiClientProvider).getEvents();
        } catch (_) {
          // Best-effort — a failed fetch shouldn't block sending the report.
        }
        final errors = events
            .where((e) => (e['name'] as String? ?? '').toLowerCase().contains('error'))
            .toList();
        final last10 = errors.length > 10 ? errors.sublist(errors.length - 10) : errors;
        if (last10.isNotEmpty) {
          buffer.writeln();
          buffer.writeln();
          buffer.writeln('---');
          buffer.writeln(L10n.t('report_bug_last_errors_header'));
          for (final e in last10) {
            buffer.writeln('- ${e['name']}: ${e['data'] ?? ''}');
          }
        }
      }

      final shortTitle = desc.length > 60 ? '${desc.substring(0, 60)}...' : desc;
      final title = Uri.encodeComponent('${L10n.t('report_bug_issue_title_prefix')}: $shortTitle');
      final body = Uri.encodeComponent(buffer.toString());
      final url = Uri.parse(
        'https://github.com/BugraAkdemir/memo/issues/new?title=$title&body=$body',
      );

      final launched = await launchUrl(url, mode: LaunchMode.externalApplication);
      if (!mounted) return;
      if (launched) {
        _descCtrl.clear();
        setState(() => _includeErrors = false);
      } else {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('report_bug_launch_failed'))),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('report_bug_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return ListView(
      padding: const EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('report_bug_title'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
                fontWeight: FontWeight.bold,
                color: theme.textMain,
              ),
        ),
        const SizedBox(height: 8),
        Text(
          L10n.t('report_bug_desc'),
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        const SizedBox(height: 24),
        TextField(
          controller: _descCtrl,
          maxLines: 8,
          minLines: 5,
          decoration: InputDecoration(
            hintText: L10n.t('report_bug_hint'),
            hintStyle: TextStyle(color: theme.textDim),
            border: const OutlineInputBorder(),
            filled: true,
            fillColor: theme.bgElement,
          ),
          style: TextStyle(color: theme.textMain, fontSize: 13),
        ),
        const SizedBox(height: 12),
        Card(
          child: CheckboxListTile(
            value: _includeErrors,
            onChanged: (v) => setState(() => _includeErrors = v ?? false),
            controlAffinity: ListTileControlAffinity.leading,
            title: Text(L10n.t('report_bug_include_errors')),
            subtitle: Text(
              L10n.t('report_bug_include_errors_desc'),
              style: TextStyle(fontSize: 12, color: theme.textDim),
            ),
          ),
        ),
        const SizedBox(height: 20),
        Align(
          alignment: Alignment.centerRight,
          child: FilledButton.icon(
            onPressed: _sending ? null : _submit,
            icon: _sending
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                  )
                : const Icon(Icons.bug_report_outlined, size: 18),
            label: Text(L10n.t('report_bug_submit_btn')),
            style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
          ),
        ),
        const SizedBox(height: 16),
        Text(
          L10n.t('report_bug_footer_note'),
          style: TextStyle(fontSize: 11, color: theme.textDim, fontStyle: FontStyle.italic),
        ),
      ],
    );
  }
}
