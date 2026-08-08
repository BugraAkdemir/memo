import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n.dart';
import '../core/theme.dart';
import '../providers/chat_provider.dart';
import '../core/friendly_error.dart';

/// A configured routine, as returned by GET /api/routines.
///
/// Round-trips every field the backend's Routine struct has (BUG-C2): this
/// model used to only carry a subset (no weekdays/context_source/
/// auto_approve_tools/whatsapp_target_jid), so saving any edit — even just
/// flipping the enable switch — silently wiped those fields via toJson()'s
/// incomplete PUT body. The backend now also merges onto the existing
/// stored routine rather than blindly replacing it, but this model should
/// still carry everything it received rather than relying solely on that
/// server-side safety net.
class _Routine {
  final String id;
  final String createdFromText;
  final String timeOfDay;
  final List<int> weekdays;
  final String prompt;
  final bool agentMode;
  final bool autoApproveTools;
  final String contextSourceType;
  final String contextWhatsAppJid;
  final bool deliveryWhatsApp;
  final bool deliveryMobile;
  final String whatsAppTargetJid;
  final bool enabled;
  final String language;
  final int? utcOffsetMinutes;

  _Routine({
    required this.id,
    required this.createdFromText,
    required this.timeOfDay,
    required this.weekdays,
    required this.prompt,
    required this.agentMode,
    required this.autoApproveTools,
    required this.contextSourceType,
    required this.contextWhatsAppJid,
    required this.deliveryWhatsApp,
    required this.deliveryMobile,
    required this.whatsAppTargetJid,
    required this.enabled,
    required this.language,
    required this.utcOffsetMinutes,
  });

  factory _Routine.fromJson(Map<String, dynamic> j) {
    final schedule = (j['schedule'] as Map?) ?? {};
    final contextSource = (j['context_source'] as Map?) ?? {};
    return _Routine(
      id: j['id'] as String? ?? '',
      createdFromText: j['created_from_text'] as String? ?? '',
      timeOfDay: schedule['time_of_day'] as String? ?? '',
      weekdays: ((schedule['weekdays'] as List?) ?? []).map((e) => e as int).toList(),
      prompt: j['prompt'] as String? ?? '',
      agentMode: j['agent_mode'] as bool? ?? false,
      autoApproveTools: j['auto_approve_tools'] as bool? ?? false,
      contextSourceType: contextSource['type'] as String? ?? 'none',
      contextWhatsAppJid: contextSource['whatsapp_jid'] as String? ?? '',
      deliveryWhatsApp: j['delivery_whatsapp'] as bool? ?? false,
      deliveryMobile: j['delivery_mobile'] as bool? ?? false,
      whatsAppTargetJid: j['whatsapp_target_jid'] as String? ?? '',
      enabled: j['enabled'] as bool? ?? false,
      language: j['language'] as String? ?? 'tr',
      utcOffsetMinutes: schedule['utc_offset_minutes'] as int?,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'created_from_text': createdFromText,
        'schedule': {
          'time_of_day': timeOfDay,
          'weekdays': weekdays,
          'utc_offset_minutes': utcOffsetMinutes,
        },
        'prompt': prompt,
        'agent_mode': agentMode,
        'auto_approve_tools': autoApproveTools,
        'context_source': {'type': contextSourceType, 'whatsapp_jid': contextWhatsAppJid},
        'delivery_whatsapp': deliveryWhatsApp,
        'delivery_mobile': deliveryMobile,
        'whatsapp_target_jid': whatsAppTargetJid,
        'enabled': enabled,
        'language': language,
      };
}

class RoutinesScreen extends ConsumerStatefulWidget {
  const RoutinesScreen({super.key});

  @override
  ConsumerState<RoutinesScreen> createState() => _RoutinesScreenState();
}

class _RoutinesScreenState extends ConsumerState<RoutinesScreen> {
  final _textController = TextEditingController();
  List<_Routine> _routines = [];
  bool _loading = true;
  bool _parsing = false;
  String? _error;

  // The draft awaiting confirmation, plus the human-only choices the backend
  // deliberately never infers from free text (see internal/routine's design
  // notes): which WhatsApp chat, and whether unattended tool execution is
  // allowed.
  Map<String, dynamic>? _draft;
  String? _originalText;
  String? _selectedWhatsAppJid;
  bool _autoApproveTools = false;
  List<Map<String, dynamic>> _whatsAppChats = [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _textController.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final api = ref.read(apiClientProvider);
      final raw = await api.getRoutines();
      if (!mounted) return;
      setState(() {
        _routines = raw.map(_Routine.fromJson).toList();
        _loading = false;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = L10n.t('routines_load_error', {'e': FriendlyError.describeGeneric(e)});
      });
    }
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
        } catch (_) {
          // WhatsApp not connected — user can still save without a chat pick.
        }
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
        _error = L10n.t('routines_parse_error', {'e': FriendlyError.describeGeneric(e)});
      });
    }
  }

  Future<void> _confirm() async {
    final draft = _draft;
    final original = _originalText;
    if (draft == null || original == null) return;
    // BUG-L2: if WhatsApp delivery is requested but no chat could be picked
    // (fetching the chat list failed above, or WhatsApp genuinely has none),
    // saving anyway used to silently create a routine whose delivery can
    // never succeed. Block it instead — there's no in-card way to turn
    // delivery_whatsapp back off, so continuing would just fail forever with
    // no way for the user to know why.
    if (draft['delivery_whatsapp'] == true && (_selectedWhatsAppJid ?? '').isEmpty) {
      setState(() => _error = L10n.t('routines_whatsapp_target_required'));
      return;
    }
    try {
      final api = ref.read(apiClientProvider);
      await api.createRoutine(
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
      await _load();
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = L10n.t('routines_save_error', {'e': FriendlyError.describeGeneric(e)}));
    }
  }

  Future<void> _toggleEnabled(_Routine r) async {
    try {
      final api = ref.read(apiClientProvider);
      final updated = Map<String, dynamic>.from(r.toJson());
      updated['enabled'] = !r.enabled;
      await api.updateRoutine(updated);
      await _load();
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = L10n.t('routines_update_error', {'e': FriendlyError.describeGeneric(e)}));
    }
  }

  Future<void> _delete(_Routine r) async {
    try {
      final api = ref.read(apiClientProvider);
      await api.deleteRoutine(r.id);
      await _load();
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = L10n.t('routines_delete_error', {'e': FriendlyError.describeGeneric(e)}));
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(L10n.t('routines_title'),
              style: TextStyle(fontSize: 22, fontWeight: FontWeight.w600, color: c.textMain)),
          const SizedBox(height: 4),
          Text(
            L10n.t('routines_example'),
            style: TextStyle(fontSize: 13, color: c.textSecondary),
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _textController,
                  decoration: InputDecoration(
                    hintText: L10n.t('routines_hint'),
                    border: const OutlineInputBorder(),
                  ),
                  onSubmitted: (_) => _parse(),
                ),
              ),
              const SizedBox(width: 12),
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
            const SizedBox(height: 16),
            _buildConfirmationCard(c),
          ],
          const SizedBox(height: 24),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _routines.isEmpty
                    ? Center(
                        child: Text(L10n.t('routines_empty'),
                            style: TextStyle(color: c.textSecondary)))
                    : ListView.builder(
                        itemCount: _routines.length,
                        itemBuilder: (context, i) => _buildRoutineTile(_routines[i], c),
                      ),
          ),
        ],
      ),
    );
  }

  Widget _buildConfirmationCard(ThemeColors c) {
    final draft = _draft!;
    final needsAgent = draft['needs_agent_mode'] == true;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        border: Border.all(color: c.borderSoft),
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
            style: const TextStyle(fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 8),
          Wrap(spacing: 8, children: [
            if (draft['delivery_whatsapp'] == true)
              const Chip(label: Text('WhatsApp')),
            if (draft['delivery_mobile'] == true)
              Chip(label: Text(L10n.t('routines_mobile_notify'))),
          ]),
          if (_whatsAppChats.isNotEmpty) ...[
            const SizedBox(height: 12),
            Text(L10n.t('routines_whatsapp_pick')),
            const SizedBox(height: 4),
            DropdownButton<String>(
              value: _selectedWhatsAppJid,
              hint: Text(L10n.t('routines_pick_chat')),
              isExpanded: true,
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
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(color: c.bgApp, borderRadius: BorderRadius.circular(8)),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      L10n.t('routines_auto_approve'),
                      style: TextStyle(fontSize: 12, color: c.textSecondary),
                    ),
                  ),
                  Switch(
                    value: _autoApproveTools,
                    onChanged: (v) => setState(() => _autoApproveTools = v),
                  ),
                ],
              ),
            ),
          ],
          const SizedBox(height: 12),
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

  Widget _buildRoutineTile(_Routine r, ThemeColors c) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        title: Text(r.prompt),
        subtitle: Text(
          L10n.t('routines_time', {'time': r.timeOfDay}) +
              (r.deliveryWhatsApp ? L10n.t('routines_via_whatsapp') : '') +
              (r.deliveryMobile ? L10n.t('routines_via_mobile') : '') +
              (r.agentMode ? L10n.t('routines_can_run_commands') : ''),
        ),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Switch(value: r.enabled, onChanged: (_) => _toggleEnabled(r)),
            IconButton(
              icon: const Icon(Icons.delete_outline),
              onPressed: () => _delete(r),
            ),
          ],
        ),
      ),
    );
  }
}
