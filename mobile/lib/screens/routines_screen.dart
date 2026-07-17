import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/routine.dart';
import '../providers/connection_provider.dart';
import '../providers/routine_provider.dart';

class RoutinesScreen extends ConsumerStatefulWidget {
  const RoutinesScreen({super.key});

  @override
  ConsumerState<RoutinesScreen> createState() => _RoutinesScreenState();
}

class _RoutinesScreenState extends ConsumerState<RoutinesScreen> {
  final _textController = TextEditingController();
  bool _parsing = false;
  String? _error;

  Map<String, dynamic>? _draft;
  String? _originalText;
  String? _selectedWhatsAppJid;
  bool _autoApproveTools = false;
  List<Map<String, dynamic>> _whatsAppChats = [];

  @override
  void dispose() {
    _textController.dispose();
    super.dispose();
  }

  Future<void> _parse() async {
    final text = _textController.text.trim();
    if (text.isEmpty) return;
    setState(() {
      _parsing = true;
      _error = null;
    });
    try {
      final api = ref.read(apiClientProvider);
      final draft = await api.parseRoutineText(text);
      List<Map<String, dynamic>> chats = [];
      if (draft['context_source_type'] == 'whatsapp' || draft['delivery_whatsapp'] == true) {
        try {
          final raw = await api.getWhatsAppChats();
          chats = raw.map((e) => Map<String, dynamic>.from(e as Map)).toList();
        } catch (_) {}
      }
      if (!mounted) return;
      setState(() {
        _draft = draft;
        _originalText = text;
        _whatsAppChats = chats;
        _selectedWhatsAppJid = null;
        _autoApproveTools = false;
        _parsing = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _parsing = false;
        _error = L10n.t('routines_parse_error', {'e': '$e'});
      });
    }
  }

  Future<void> _confirm() async {
    final draft = _draft;
    final original = _originalText;
    if (draft == null || original == null) return;
    await ref.read(routineProvider.notifier).createFromDraft(
          originalText: original,
          draft: draft,
          whatsAppTargetJid: _selectedWhatsAppJid ?? '',
          autoApproveTools: _autoApproveTools,
        );
    _textController.clear();
    if (!mounted) return;
    setState(() {
      _draft = null;
      _originalText = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(routineProvider);

    return Scaffold(
      backgroundColor: MemoTheme.bg,
      appBar: AppBar(
        backgroundColor: MemoTheme.surface,
        title: Text(L10n.t('routines_title'), style: TextStyle(color: MemoTheme.text, fontWeight: FontWeight.w600)),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              L10n.t('routines_example'),
              style: TextStyle(fontSize: 12, color: MemoTheme.textDim),
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _textController,
                    style: TextStyle(color: MemoTheme.text),
                    decoration: InputDecoration(
                      hintText: L10n.t('routines_hint'),
                      border: const OutlineInputBorder(),
                    ),
                    onSubmitted: (_) => _parse(),
                  ),
                ),
                const SizedBox(width: 8),
                FilledButton(
                  onPressed: _parsing ? null : _parse,
                  child: _parsing
                      ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                      : Text(L10n.t('send')),
                ),
              ],
            ),
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(_error!, style: const TextStyle(color: Colors.red)),
            ],
            if (_draft != null) ...[
              const SizedBox(height: 12),
              _buildConfirmationCard(),
            ],
            const SizedBox(height: 16),
            Expanded(
              child: state.loading
                  ? const Center(child: CircularProgressIndicator())
                  : state.routines.isEmpty
                      ? Center(child: Text(L10n.t('routines_empty'), style: TextStyle(color: MemoTheme.textDim)))
                      : ListView.builder(
                          itemCount: state.routines.length,
                          itemBuilder: (context, i) => _buildRoutineTile(state.routines[i]),
                        ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildConfirmationCard() {
    final draft = _draft!;
    final needsAgent = draft['needs_agent_mode'] == true;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        border: Border.all(color: MemoTheme.textDim.withValues(alpha: 0.3)),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            L10n.t('routines_confirm', {
              'time': '${draft['time_of_day']}',
              'prompt': '${draft['prompt']}',
            }),
            style: TextStyle(fontWeight: FontWeight.w600, color: MemoTheme.text),
          ),
          if (_whatsAppChats.isNotEmpty) ...[
            const SizedBox(height: 8),
            Text(L10n.t('routines_whatsapp_pick'), style: TextStyle(color: MemoTheme.textDim)),
            DropdownButton<String>(
              value: _selectedWhatsAppJid,
              isExpanded: true,
              hint: Text(L10n.t('routines_pick_chat')),
              items: _whatsAppChats
                  .map((chat) => DropdownMenuItem<String>(
                        value: chat['jid'] as String?,
                        child: Text(chat['display_name'] as String? ?? chat['jid'] as String? ?? ''),
                      ))
                  .toList(),
              onChanged: (v) => setState(() => _selectedWhatsAppJid = v),
            ),
          ],
          if (needsAgent) ...[
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(
                  child: Text(
                    L10n.t('routines_auto_approve'),
                    style: TextStyle(fontSize: 12, color: MemoTheme.textDim),
                  ),
                ),
                Switch(value: _autoApproveTools, onChanged: (v) => setState(() => _autoApproveTools = v)),
              ],
            ),
          ],
          const SizedBox(height: 8),
          Row(
            children: [
              FilledButton(onPressed: _confirm, child: Text(L10n.t('save'))),
              const SizedBox(width: 8),
              TextButton(
                onPressed: () => setState(() {
                  _draft = null;
                  _originalText = null;
                }),
                child: Text(L10n.t('routines_discard')),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildRoutineTile(Routine r) {
    return Card(
      color: MemoTheme.surface,
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        title: Text(r.prompt, style: TextStyle(color: MemoTheme.text)),
        subtitle: Text(
          L10n.t('routines_time', {'time': r.timeOfDay}) +
              (r.deliveryWhatsApp ? L10n.t('routines_via_whatsapp') : '') +
              (r.deliveryMobile ? L10n.t('routines_via_mobile') : ''),
          style: TextStyle(color: MemoTheme.textDim),
        ),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Switch(
              value: r.enabled,
              onChanged: (_) => ref.read(routineProvider.notifier).toggleEnabled(r),
            ),
            IconButton(
              icon: Icon(Icons.delete_outline, color: MemoTheme.textDim),
              onPressed: () => ref.read(routineProvider.notifier).delete(r.id),
            ),
          ],
        ),
      ),
    );
  }
}
