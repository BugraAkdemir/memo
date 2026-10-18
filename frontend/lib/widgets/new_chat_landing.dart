import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path/path.dart' as p;

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/chat.dart';
import '../providers/new_chat_provider.dart';
import 'welcome_view.dart' show SuggestionCard;

/// What a brand-new, empty chat shows: a Chat | Agent switch at the top, a greeting,
/// the composer in the middle of the page and starter suggestions underneath. The
/// kind (plain chat or agent) is chosen here, before the first message, and never
/// changes afterwards — once the chat has a message this screen is gone and the
/// composer sits at the bottom as usual.
///
/// [composer] is the page's one ChatInput (passed in so its state, text and focus
/// survive the move to the bottom).
class NewChatLanding extends ConsumerWidget {
  final ChatSession chat;
  final Widget composer;

  const NewChatLanding({super.key, required this.chat, required this.composer});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final pending = ref.watch(pendingAgentChatsProvider);
    final agent = isAgentModeFor(chat, pending);
    final project = chat.isAgentChat ? chat.projectPath! : '';

    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(20, 16, 20, 24),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 760),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Center(child: ChatModeSwitch(agent: agent, onChanged: (a) => setNewChatMode(context, ref, chat, agent: a))),
              const SizedBox(height: 40),
              Center(
                child: Text(
                  agent ? L10n.t('landing_agent_title') : L10n.t('landing_chat_hint'),
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w600, color: c.textMain),
                  textAlign: TextAlign.center,
                ),
              ),
              const SizedBox(height: 22),
              composer,
              if (agent) ...[
                const SizedBox(height: 10),
                _ProjectRow(chat: chat, project: project),
              ],
              const SizedBox(height: 26),
              Wrap(
                spacing: 12,
                runSpacing: 12,
                alignment: WrapAlignment.center,
                children: agent ? _agentSuggestions() : _chatSuggestions(),
              ),
            ],
          ),
        ),
      ),
    );
  }

  List<Widget> _chatSuggestions() => [
        SuggestionCard(
          icon: Icons.code_rounded,
          label: L10n.t('suggest_review_label'),
          hint: L10n.t('suggest_review_hint'),
          starter: L10n.t('quick_review_starter'),
        ),
        SuggestionCard(
          icon: Icons.menu_book_outlined,
          label: L10n.t('suggest_explain_label'),
          hint: L10n.t('suggest_explain_hint'),
          starter: L10n.t('quick_explain_starter'),
        ),
        SuggestionCard(
          icon: Icons.checklist_rounded,
          label: L10n.t('suggest_plan_label'),
          hint: L10n.t('suggest_plan_hint'),
          starter: L10n.t('quick_plan_starter'),
        ),
        SuggestionCard(
          icon: Icons.lightbulb_outline_rounded,
          label: L10n.t('suggest_ideate_label'),
          hint: L10n.t('suggest_ideate_hint'),
          starter: L10n.t('quick_ideate_starter'),
        ),
      ];

  List<Widget> _agentSuggestions() => [
        SuggestionCard(
          icon: Icons.travel_explore,
          label: L10n.t('suggest_agent_explore_label'),
          hint: L10n.t('suggest_agent_explore_hint'),
          starter: L10n.t('suggest_agent_explore_starter'),
        ),
        SuggestionCard(
          icon: Icons.bug_report_outlined,
          label: L10n.t('suggest_agent_fix_label'),
          hint: L10n.t('suggest_agent_fix_hint'),
          starter: L10n.t('suggest_agent_fix_starter'),
        ),
        SuggestionCard(
          icon: Icons.add_task,
          label: L10n.t('suggest_agent_feature_label'),
          hint: L10n.t('suggest_agent_feature_hint'),
          starter: L10n.t('suggest_agent_feature_starter'),
        ),
        SuggestionCard(
          icon: Icons.description_outlined,
          label: L10n.t('suggest_agent_docs_label'),
          hint: L10n.t('suggest_agent_docs_hint'),
          starter: L10n.t('suggest_agent_docs_starter'),
        ),
      ];
}

/// The Chat | Agent pill.
class ChatModeSwitch extends StatelessWidget {
  final bool agent;
  final ValueChanged<bool> onChanged;
  const ChatModeSwitch({super.key, required this.agent, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    Widget seg({required bool selected, required IconData icon, required String label, required VoidCallback onTap}) {
      return Material(
        color: selected ? c.bgElement : Colors.transparent,
        borderRadius: BorderRadius.circular(20),
        child: InkWell(
          borderRadius: BorderRadius.circular(20),
          onTap: onTap,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 8),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(icon, size: 16, color: selected ? MemoTheme.green : c.textDim),
                const SizedBox(width: 7),
                Text(
                  label,
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
                    color: selected ? c.textMain : c.textDim,
                  ),
                ),
              ],
            ),
          ),
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: c.borderSoft),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          seg(selected: !agent, icon: Icons.chat_bubble_outline, label: L10n.t('mode_chat_label'), onTap: () => onChanged(false)),
          seg(selected: agent, icon: Icons.smart_toy_outlined, label: L10n.t('mode_agent_label'), onTap: () => onChanged(true)),
        ],
      ),
    );
  }
}

/// Under the composer in Agent mode: pick (or change) the project folder.
class _ProjectRow extends ConsumerWidget {
  final ChatSession chat;
  final String project;
  const _ProjectRow({required this.chat, required this.project});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final c = MemoTheme.of(context);
    final has = project.isNotEmpty;
    return Wrap(
      alignment: WrapAlignment.start,
      crossAxisAlignment: WrapCrossAlignment.center,
      spacing: 10,
      runSpacing: 6,
      children: [
        OutlinedButton.icon(
          onPressed: () => chooseProjectForNewChat(context, ref, chat.id),
          icon: Icon(has ? Icons.folder : Icons.folder_open_outlined, size: 17),
          label: Text(has ? p.basename(project) : L10n.t('landing_choose_project')),
          style: OutlinedButton.styleFrom(
            foregroundColor: has ? MemoTheme.green : c.textMain,
            side: BorderSide(color: has ? MemoTheme.green.withValues(alpha: 0.5) : c.borderSoft),
          ),
        ),
        if (has)
          TextButton(
            onPressed: () => chooseProjectForNewChat(context, ref, chat.id),
            child: Text(L10n.t('landing_change_project')),
          ),
        Text(
          has ? project : L10n.t('landing_project_hint'),
          style: TextStyle(fontSize: 12, color: c.textDim),
          overflow: TextOverflow.ellipsis,
        ),
      ],
    );
  }
}
