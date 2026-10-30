import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../core/clipboard_copy.dart';
import '../core/friendly_error.dart';
import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/mail.dart';
import '../widgets/mail_security_dialog.dart';
import '../widgets/mail_body.dart';
import '../providers/chat_provider.dart';
import '../providers/auth_gate_provider.dart';
import '../providers/gate_guard.dart';
import '../providers/mail_provider.dart';

class MailScreen extends ConsumerStatefulWidget {
  const MailScreen({super.key, this.isActive = true});

  final bool isActive;

  @override
  ConsumerState<MailScreen> createState() => _MailScreenState();
}

class _MailScreenState extends ConsumerState<MailScreen>
    with WidgetsBindingObserver {
  Timer? _statsTimer;
  bool _statsReloading = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _updateStatsTimer();
  }

  @override
  void didUpdateWidget(covariant MailScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.isActive != widget.isActive) _updateStatsTimer();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    _updateStatsTimer();
  }

  void _updateStatsTimer() {
    _statsTimer?.cancel();
    final state = WidgetsBinding.instance.lifecycleState;
    if (!widget.isActive ||
        (state != null && state != AppLifecycleState.resumed)) {
      return;
    }
    _statsTimer = Timer.periodic(
      const Duration(seconds: 15),
      (_) => _reloadStats(),
    );
    Future.microtask(_reloadStats);
  }

  Future<void> _reloadStats() async {
    if (!mounted ||
        !widget.isActive ||
        _statsReloading ||
        authGateBlocked(ref.read(authGateProvider).valueOrNull)) {
      return;
    }
    _statsReloading = true;
    try {
      await ref.read(mailStatsProvider.notifier).reload();
    } finally {
      _statsReloading = false;
    }
  }

  final TextEditingController _searchController = TextEditingController();

  List<MailMessageSummary> _messages = const [];
  bool _loadingInbox = false;
  String? _inboxError;
  bool _isSearchActive = false;

  int? _selectedSeqNum;
  MailMessageDetail? _selectedDetail;
  bool _loadingDetail = false;
  String? _detailError;

  bool _syncing = false;
  String? _lastLoadedAccountId;

  @override
  void dispose() {
    _statsTimer?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    _searchController.dispose();
    super.dispose();
  }

  void _toast(String message, {bool isError = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor: isError ? MemoTheme.red : null,
        behavior: SnackBarBehavior.floating,
      ),
    );
  }

  Future<void> _loadInbox(String accountId) async {
    setState(() {
      _loadingInbox = true;
      _inboxError = null;
      _isSearchActive = false;
    });
    try {
      final res = await ref.read(apiClientProvider).getMailInbox(
            accountId,
            limit: 30,
          );
      if (!mounted) return;
      setState(() {
        _messages = res.messages;
        _loadingInbox = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loadingInbox = false;
        _inboxError = FriendlyError.describeGeneric(e);
      });
    }
  }

  Future<void> _runSearch(String accountId) async {
    final q = _searchController.text.trim();
    if (q.isEmpty) {
      await _loadInbox(accountId);
      return;
    }
    setState(() {
      _loadingInbox = true;
      _inboxError = null;
      _isSearchActive = true;
    });
    try {
      final list = await ref.read(apiClientProvider).searchMail(
            accountId,
            q,
            limit: 30,
          );
      if (!mounted) return;
      setState(() {
        _messages = list;
        _loadingInbox = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loadingInbox = false;
        _inboxError = FriendlyError.describeGeneric(e);
      });
    }
  }

  Future<void> _selectMessage(String accountId, int seqNum) async {
    setState(() {
      _selectedSeqNum = seqNum;
      _loadingDetail = true;
      _detailError = null;
    });
    try {
      final detail = await ref.read(apiClientProvider).getMailMessage(
            accountId,
            seqNum,
          );
      if (!mounted) return;
      setState(() {
        _selectedDetail = detail;
        _loadingDetail = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loadingDetail = false;
        _detailError = FriendlyError.describeGeneric(e);
      });
    }
  }

  Future<void> _triggerSync(String? activeAccountId) async {
    if (_syncing) return;
    setState(() => _syncing = true);
    try {
      await ref.read(mailStatsProvider.notifier).syncNow();
      if (activeAccountId != null && activeAccountId.isNotEmpty) {
        await _loadInbox(activeAccountId);
      }
      _toast(L10n.t('mail_sync_done'));
    } catch (e) {
      _toast(
        L10n.t('mail_error_prefix', {'err': FriendlyError.describeGeneric(e)}),
        isError: true,
      );
    } finally {
      if (mounted) setState(() => _syncing = false);
    }
  }

  Future<void> _openOAuthConnectDialog(String provider) async {
    final connected = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => _MailOAuthDialog(provider: provider),
    );
    if (connected == true && mounted) {
      await ref.read(mailAccountsProvider.notifier).reload();
      await ref.read(mailStatsProvider.notifier).reload();
      _toast(L10n.t('mail_oauth_connected'));
    }
  }

  Future<void> _openCustomConnectDialog() async {
    final connected = await showDialog<bool>(
      context: context,
      builder: (ctx) => const _MailCustomConnectDialog(),
    );
    if (connected == true && mounted) {
      await ref.read(mailAccountsProvider.notifier).reload();
      await ref.read(mailStatsProvider.notifier).reload();
      _toast(L10n.t('mail_oauth_connected'));
    }
  }

  Future<void> _openComposeDialog({
    required String accountId,
    String initialTo = '',
    String initialSubject = '',
    String inReplyTo = '',
  }) async {
    await showDialog<void>(
      context: context,
      builder: (ctx) => _MailComposeDialog(
        accountId: accountId,
        initialTo: initialTo,
        initialSubject: initialSubject,
        inReplyTo: inReplyTo,
        onToast: _toast,
      ),
    );
  }

  Future<void> _confirmRemoveAccount(MailAccount account) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('mail_remove_confirm_title')),
        content: Text(
          L10n.t('mail_remove_confirm_body', {'email': account.email}),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(L10n.t('cancel')),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(backgroundColor: MemoTheme.red),
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(L10n.t('delete')),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    try {
      await ref.read(mailAccountsProvider.notifier).removeAccount(account.id);
      setState(() {
        _lastLoadedAccountId = null;
        _messages = const [];
        _selectedSeqNum = null;
        _selectedDetail = null;
      });
    } catch (e) {
      _toast(
        L10n.t('mail_error_prefix', {'err': FriendlyError.describeGeneric(e)}),
        isError: true,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final accountsAsync = ref.watch(mailAccountsProvider);
    final statsAsync = ref.watch(mailStatsProvider);
    final activeAccountId = ref.watch(activeMailAccountIdProvider);

    ref.listen<String?>(activeMailAccountIdProvider, (prev, next) {
      if (next != null && next.isNotEmpty && next != _lastLoadedAccountId) {
        _lastLoadedAccountId = next;
        _selectedSeqNum = null;
        _selectedDetail = null;
        _loadInbox(next);
      }
    });

    if (activeAccountId != null &&
        activeAccountId.isNotEmpty &&
        _lastLoadedAccountId != activeAccountId &&
        !_loadingInbox) {
      _lastLoadedAccountId = activeAccountId;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _loadInbox(activeAccountId);
      });
    }

    return Container(
      color: c.bgApp,
      padding: const EdgeInsets.fromLTRB(24, 20, 24, 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _buildHeader(c, statsAsync.valueOrNull, activeAccountId),
          const SizedBox(height: 16),
          _buildConnectCardsRow(c),
          const SizedBox(height: 16),
          Expanded(
            child: accountsAsync.when(
              loading: () => const Center(
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
              error: (err, _) => Center(
                child: Text(
                  L10n.t(
                    'mail_error_prefix',
                    {'err': FriendlyError.describeGeneric(err)},
                  ),
                  style: const TextStyle(color: MemoTheme.red),
                ),
              ),
              data: (acctList) {
                if (acctList.isEmpty) {
                  return _buildEmptyAccountsState(c);
                }
                final currentId = activeAccountId ?? acctList.first.id;
                return _buildWorkspace(c, acctList, currentId);
              },
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildHeader(
    ThemeColors c,
    MailWatcherStats? stats,
    String? activeAccountId,
  ) {
    return Wrap(
      alignment: WrapAlignment.spaceBetween,
      crossAxisAlignment: WrapCrossAlignment.center,
      spacing: 12,
      runSpacing: 12,
      children: [
        Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              L10n.t('mail_title'),
              style: TextStyle(
                fontSize: 22,
                fontWeight: FontWeight.w700,
                color: c.textMain,
              ),
            ),
            const SizedBox(height: 4),
            Text(
              L10n.t('mail_subtitle'),
              style: TextStyle(fontSize: 13, color: c.textDim),
            ),
          ],
        ),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            if (stats != null) ...[
              _StatBadge(
                icon: Icons.fact_check_outlined,
                label: L10n.t(
                  'mail_stats_evaluated',
                  {'n': '${stats.totalEvaluated}'},
                ),
                color: c.textSecondary,
              ),
              _StatBadge(
                icon: Icons.shield_outlined,
                label: L10n.t(
                  'mail_stats_blocked',
                  {'n': '${stats.droppedMarketing}'},
                ),
                color: MemoTheme.warningOrange,
              ),
              _StatBadge(
                icon: Icons.psychology_outlined,
                label: L10n.t(
                  'mail_stats_indexed',
                  {'n': '${stats.indexedInRag}'},
                ),
                color: MemoTheme.green,
              ),
              _StatBadge(
                icon: Icons.event_available_outlined,
                label: L10n.t(
                  'mail_stats_calendar',
                  {'n': '${stats.calendarExtracted}'},
                ),
                color: MemoTheme.accent,
              ),
            ],
            if (stats != null && stats.securityBlocked > 0)
              Tooltip(
                message: mailSecurityReason(stats.lastSecurityReason),
                child: _StatBadge(
                  icon: Icons.gpp_bad_outlined,
                  label: L10n.t('mail_stats_security', {'n': '${stats.securityBlocked}'}),
                  color: MemoTheme.warningOrange,
                ),
              ),
            OutlinedButton.icon(
              onPressed: () => showDialog<void>(
                context: context,
                builder: (_) => const MailSecurityDialog(),
              ),
              icon: const Icon(Icons.security_outlined, size: 16),
              label: Text(L10n.t('mail_security_title')),
            ),
            OutlinedButton.icon(
              onPressed: _syncing ? null : () => _triggerSync(activeAccountId),
              icon: _syncing
                  ? const SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.sync, size: 16),
              label: Text(
                L10n.t(_syncing ? 'mail_syncing' : 'mail_sync_now'),
              ),
            ),
            if (activeAccountId != null && activeAccountId.isNotEmpty)
              ElevatedButton.icon(
                onPressed: () => _openComposeDialog(accountId: activeAccountId),
                icon: const Icon(Icons.edit_outlined, size: 16),
                label: Text(L10n.t('mail_compose')),
              ),
          ],
        ),
      ],
    );
  }

  Widget _buildConnectCardsRow(ThemeColors c) {
    return Row(
      children: [
        Expanded(
          child: _ProviderConnectCard(
            icon: Icons.mail_lock_outlined,
            title: L10n.t('mail_connect_gmail'),
            subtitle: L10n.t('mail_connect_gmail_desc'),
            accentColor: MemoTheme.red,
            onTap: () => _openOAuthConnectDialog('google'),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: _ProviderConnectCard(
            icon: Icons.mark_email_read_outlined,
            title: L10n.t('mail_connect_outlook'),
            subtitle: L10n.t('mail_connect_outlook_desc'),
            accentColor: MemoTheme.accent,
            onTap: () => _openOAuthConnectDialog('microsoft'),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: _ProviderConnectCard(
            icon: Icons.dns_outlined,
            title: L10n.t('mail_connect_custom'),
            subtitle: L10n.t('mail_connect_custom_desc'),
            accentColor: MemoTheme.green,
            onTap: _openCustomConnectDialog,
          ),
        ),
      ],
    );
  }

  Widget _buildEmptyAccountsState(ThemeColors c) {
    return Container(
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: c.borderSoft),
      ),
      alignment: Alignment.center,
      padding: const EdgeInsets.all(32),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.alternate_email, size: 48, color: c.textDim),
          const SizedBox(height: 12),
          Text(
            L10n.t('mail_no_accounts'),
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 14, color: c.textSecondary),
          ),
        ],
      ),
    );
  }

  Widget _buildWorkspace(
    ThemeColors c,
    List<MailAccount> accounts,
    String activeAccountId,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _buildAccountBarAndSearch(c, accounts, activeAccountId),
        const SizedBox(height: 12),
        Expanded(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              SizedBox(
                width: 360,
                child: _buildInboxListPane(c, activeAccountId),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _buildMessageDetailPane(c, activeAccountId),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildAccountBarAndSearch(
    ThemeColors c,
    List<MailAccount> accounts,
    String activeAccountId,
  ) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: c.borderSoft),
      ),
      child: Row(
        children: [
          Text(
            L10n.t('mail_connected_accounts'),
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: c.textDim,
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: accounts.map((acct) {
                  final selected = acct.id == activeAccountId;
                  return Padding(
                    padding: const EdgeInsets.only(right: 8),
                    child: InputChip(
                      selected: selected,
                      selectedColor: MemoTheme.accentMuted,
                      label: Text(
                        acct.email,
                        style: TextStyle(
                          fontSize: 12,
                          fontWeight:
                              selected ? FontWeight.w600 : FontWeight.w400,
                          color: selected ? MemoTheme.accent : c.textMain,
                        ),
                      ),
                      onSelected: (_) {
                        ref.read(activeMailAccountIdProvider.notifier).state =
                            acct.id;
                      },
                      onDeleted: () => _confirmRemoveAccount(acct),
                      deleteButtonTooltipMessage: L10n.t('mail_remove_account'),
                    ),
                  );
                }).toList(),
              ),
            ),
          ),
          const SizedBox(width: 12),
          SizedBox(
            width: 260,
            child: TextField(
              controller: _searchController,
              style: TextStyle(fontSize: 13, color: c.textMain),
              decoration: InputDecoration(
                isDense: true,
                hintText: L10n.t('mail_search_hint'),
                contentPadding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 10,
                ),
                suffixIcon: _isSearchActive
                    ? IconButton(
                        tooltip: L10n.t('mail_clear_search'),
                        icon: const Icon(Icons.clear, size: 16),
                        onPressed: () {
                          _searchController.clear();
                          _loadInbox(activeAccountId);
                        },
                      )
                    : IconButton(
                        tooltip: L10n.t('mail_search_button'),
                        icon: const Icon(Icons.search, size: 16),
                        onPressed: () => _runSearch(activeAccountId),
                      ),
              ),
              onSubmitted: (_) => _runSearch(activeAccountId),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildInboxListPane(ThemeColors c, String activeAccountId) {
    return Container(
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: c.borderSoft),
      ),
      child: _loadingInbox
          ? const Center(child: CircularProgressIndicator(strokeWidth: 2))
          : _inboxError != null
              ? Center(
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          L10n.t('mail_error_prefix', {'err': _inboxError!}),
                          textAlign: TextAlign.center,
                          style: const TextStyle(
                            color: MemoTheme.red,
                            fontSize: 12,
                          ),
                        ),
                        const SizedBox(height: 10),
                        OutlinedButton(
                          onPressed: () => _loadInbox(activeAccountId),
                          child: Text(L10n.t('retry')),
                        ),
                      ],
                    ),
                  ),
                )
              : _messages.isEmpty
                  ? Center(
                      child: Padding(
                        padding: const EdgeInsets.all(24),
                        child: Text(
                          L10n.t('mail_inbox_empty'),
                          textAlign: TextAlign.center,
                          style: TextStyle(fontSize: 13, color: c.textDim),
                        ),
                      ),
                    )
                  : ListView.separated(
                      itemCount: _messages.length,
                      separatorBuilder: (_, _) =>
                          Divider(height: 1, color: c.borderSoft),
                      itemBuilder: (context, idx) {
                        final msg = _messages[idx];
                        final selected = msg.seqNum == _selectedSeqNum;
                        return InkWell(
                          onTap: () =>
                              _selectMessage(activeAccountId, msg.seqNum),
                          child: Container(
                            color: selected
                                ? MemoTheme.accentMuted
                                : Colors.transparent,
                            padding: const EdgeInsets.symmetric(
                              horizontal: 14,
                              vertical: 12,
                            ),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Row(
                                  children: [
                                    if (!msg.seen)
                                      Container(
                                        width: 8,
                                        height: 8,
                                        margin: const EdgeInsets.only(right: 6),
                                        decoration: const BoxDecoration(
                                          color: MemoTheme.accent,
                                          shape: BoxShape.circle,
                                        ),
                                      ),
                                    Expanded(
                                      child: Text(
                                        msg.from,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: TextStyle(
                                          fontSize: 12,
                                          fontWeight: msg.seen
                                              ? FontWeight.w500
                                              : FontWeight.w700,
                                          color: c.textMain,
                                        ),
                                      ),
                                    ),
                                    const SizedBox(width: 6),
                                    Text(
                                      msg.date,
                                      style: TextStyle(
                                        fontSize: 10,
                                        color: c.textDim,
                                      ),
                                    ),
                                  ],
                                ),
                                const SizedBox(height: 4),
                                Text(
                                  msg.subject,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    fontSize: 13,
                                    fontWeight: msg.seen
                                        ? FontWeight.w400
                                        : FontWeight.w600,
                                    color: c.textMain,
                                  ),
                                ),
                                const SizedBox(height: 6),
                                Wrap(
                                  spacing: 6,
                                  runSpacing: 4,
                                  children: [
                                    if (msg.hasCalendarInvite)
                                      _MiniTag(
                                        label: L10n.t('mail_badge_calendar'),
                                        color: MemoTheme.green,
                                      ),
                                    if (msg.hasListUnsub)
                                      _MiniTag(
                                        label: L10n.t('mail_badge_bulk'),
                                        color: MemoTheme.warningOrange,
                                      ),
                                    if (msg.isAutoSubmitted)
                                      _MiniTag(
                                        label: L10n.t('mail_badge_auto'),
                                        color: c.textDim,
                                      ),
                                  ],
                                ),
                              ],
                            ),
                          ),
                        );
                      },
                    ),
    );
  }

  Widget _buildMessageDetailPane(ThemeColors c, String activeAccountId) {
    return Container(
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: c.borderSoft),
      ),
      child: _loadingDetail
          ? const Center(child: CircularProgressIndicator(strokeWidth: 2))
          : _detailError != null
              ? Center(
                  child: Text(
                    L10n.t('mail_error_prefix', {'err': _detailError!}),
                    style: const TextStyle(color: MemoTheme.red, fontSize: 13),
                  ),
                )
              : _selectedDetail == null
                  ? Center(
                      child: Text(
                        L10n.t('mail_select_message_prompt'),
                        style: TextStyle(fontSize: 13, color: c.textDim),
                      ),
                    )
                  : _buildMessageDetailContent(
                      c,
                      activeAccountId,
                      _selectedDetail!,
                    ),
    );
  }

  Widget _buildMessageDetailContent(
    ThemeColors c,
    String activeAccountId,
    MailMessageDetail detail,
  ) {
    final structured = detail.structured;
    return Padding(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Text(
                  detail.subject,
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w700,
                    color: c.textMain,
                  ),
                ),
              ),
              const SizedBox(width: 12),
              OutlinedButton.icon(
                onPressed: () => _openComposeDialog(
                  accountId: activeAccountId,
                  initialTo: detail.from,
                  initialSubject: detail.subject,
                  inReplyTo: detail.messageId,
                ),
                icon: const Icon(Icons.reply, size: 16),
                label: Text(L10n.t('mail_reply')),
              ),
            ],
          ),
          const SizedBox(height: 10),
          _MetaLine(label: L10n.t('mail_from'), value: detail.from),
          if (detail.to.isNotEmpty)
            _MetaLine(label: L10n.t('mail_to'), value: detail.to),
          if (detail.cc.isNotEmpty)
            _MetaLine(label: L10n.t('mail_cc'), value: detail.cc),
          if (detail.date.isNotEmpty)
            _MetaLine(label: L10n.t('mail_date'), value: detail.date),
          const SizedBox(height: 12),
          if (structured.calendarInvites.isNotEmpty ||
              structured.flights.isNotEmpty ||
              structured.invoices.isNotEmpty ||
              structured.otpCode.isNotEmpty) ...[
            _buildStructuredCards(c, structured),
            const SizedBox(height: 12),
          ],
          if (detail.attachments.isNotEmpty) ...[
            Wrap(
              spacing: 8,
              runSpacing: 6,
              children: detail.attachments.map((att) {
                return Chip(
                  avatar: const Icon(Icons.attach_file, size: 14),
                  label: Text(
                    att.filename,
                    style: TextStyle(fontSize: 11, color: c.textMain),
                  ),
                );
              }).toList(),
            ),
            const SizedBox(height: 12),
          ],
          Divider(height: 1, color: c.borderSoft),
          const SizedBox(height: 12),
          Expanded(
            child: SingleChildScrollView(
              child: MailBody(
                key: ValueKey('${detail.messageId}:${detail.uid}'),
                textBody: detail.textBody,
                htmlBody: detail.htmlBody,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStructuredCards(
    ThemeColors c,
    MailStructuredExtraction structured,
  ) {
    return Wrap(
      spacing: 10,
      runSpacing: 10,
      children: [
        for (final inv in structured.calendarInvites)
          _StructuredBadgeCard(
            icon: Icons.event_available,
            title: L10n.t('mail_structured_calendar'),
            value: '${inv.summary} (${inv.startTime})',
            color: MemoTheme.green,
          ),
        for (final fl in structured.flights)
          _StructuredBadgeCard(
            icon: Icons.flight_takeoff,
            title: L10n.t('mail_structured_flight'),
            value:
                '${fl.flightNumber} ${fl.departureAirport} → ${fl.arrivalAirport}',
            color: MemoTheme.accent,
          ),
        for (final bill in structured.invoices)
          _StructuredBadgeCard(
            icon: Icons.receipt_long,
            title: L10n.t('mail_structured_invoice'),
            value:
                '${bill.providerName} ${bill.totalAmount} ${bill.currency}',
            color: MemoTheme.warningOrange,
          ),
        if (structured.otpCode.isNotEmpty)
          _StructuredBadgeCard(
            icon: Icons.key,
            title: L10n.t('mail_structured_otp'),
            value: structured.otpCode,
            color: MemoTheme.accent,
          ),
      ],
    );
  }
}

class _StatBadge extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color color;

  const _StatBadge({
    required this.icon,
    required this.label,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: c.borderSoft),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 14, color: color),
          const SizedBox(width: 6),
          Text(
            label,
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w600,
              color: c.textMain,
            ),
          ),
        ],
      ),
    );
  }
}

class _ProviderConnectCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final Color accentColor;
  final VoidCallback onTap;

  const _ProviderConnectCard({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.accentColor,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: c.bgPanel,
          borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
          border: Border.all(color: c.borderSoft),
        ),
        child: Row(
          children: [
            Container(
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: accentColor.withValues(alpha: 0.14),
                borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
              ),
              child: Icon(icon, size: 22, color: accentColor),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                      color: c.textMain,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    subtitle,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 11, color: c.textDim),
                  ),
                ],
              ),
            ),
            Icon(Icons.chevron_right, size: 18, color: c.textDim),
          ],
        ),
      ),
    );
  }
}

class _MiniTag extends StatelessWidget {
  final String label;
  final Color color;

  const _MiniTag({required this.label, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 10,
          fontWeight: FontWeight.w600,
          color: color,
        ),
      ),
    );
  }
}

class _MetaLine extends StatelessWidget {
  final String label;
  final String value;

  const _MetaLine({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 72,
            child: Text(
              label,
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w600,
                color: c.textDim,
              ),
            ),
          ),
          Expanded(
            child: SelectableText(
              value,
              style: TextStyle(fontSize: 12, color: c.textSecondary),
            ),
          ),
        ],
      ),
    );
  }
}

class _StructuredBadgeCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final String value;
  final Color color;

  const _StructuredBadgeCard({
    required this.icon,
    required this.title,
    required this.value,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: color.withValues(alpha: 0.35)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 16, color: color),
          const SizedBox(width: 8),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: TextStyle(
                  fontSize: 10,
                  fontWeight: FontWeight.w700,
                  color: color,
                ),
              ),
              Text(
                value,
                style: TextStyle(fontSize: 12, color: c.textMain),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _MailOAuthDialog extends ConsumerStatefulWidget {
  final String provider;

  const _MailOAuthDialog({required this.provider});

  @override
  ConsumerState<_MailOAuthDialog> createState() => _MailOAuthDialogState();
}

class _MailOAuthDialogState extends ConsumerState<_MailOAuthDialog> {
  final TextEditingController _manualController = TextEditingController();
  Timer? _pollTimer;
  MailOAuthSession? _session;
  String? _error;
  bool _starting = true;
  bool _submittingManual = false;

  @override
  void initState() {
    super.initState();
    _startFlow();
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _manualController.dispose();
    super.dispose();
  }

  Future<void> _startFlow() async {
    try {
      final sess = await ref
          .read(apiClientProvider)
          .startMailOAuth(widget.provider);
      if (!mounted) return;
      setState(() {
        _session = sess;
        _starting = false;
      });
      if (sess.authUrl.isNotEmpty) {
        final uri = Uri.tryParse(sess.authUrl);
        if (uri != null) {
          unawaited(
            launchUrl(uri, mode: LaunchMode.externalApplication),
          );
        }
      }
      _startPolling(sess.state);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _starting = false;
        _error = FriendlyError.describeGeneric(e);
      });
    }
  }

  void _startPolling(String stateToken) {
    _pollTimer?.cancel();
    _pollTimer = Timer.periodic(const Duration(seconds: 2), (timer) async {
      try {
        final st = await ref
            .read(apiClientProvider)
            .getMailOAuthStatus(stateToken);
        if (!mounted) return;
        if (st.status == 'completed') {
          timer.cancel();
          Navigator.of(context).pop(true);
        } else if (st.status == 'error' && st.error.isNotEmpty) {
          timer.cancel();
          setState(() => _error = st.error);
        }
      } catch (_) {}
    });
  }

  Future<void> _submitManualCallback() async {
    final sess = _session;
    final raw = _manualController.text.trim();
    if (sess == null || raw.isEmpty || _submittingManual) return;
    setState(() {
      _submittingManual = true;
      _error = null;
    });
    try {
      await ref
          .read(apiClientProvider)
          .completeManualMailOAuth(sess.state, raw);
      if (!mounted) return;
      _pollTimer?.cancel();
      Navigator.of(context).pop(true);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _submittingManual = false;
        _error = FriendlyError.describeGeneric(e);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final sess = _session;

    return AlertDialog(
      title: Text(L10n.t('mail_oauth_dialog_title')),
      content: SizedBox(
        width: 480,
        child: _starting
            ? const Padding(
                padding: EdgeInsets.all(24),
                child: Center(
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              )
            : Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (_error != null) ...[
                    Text(
                      L10n.t('mail_error_prefix', {'err': _error!}),
                      style: const TextStyle(
                        color: MemoTheme.red,
                        fontSize: 12,
                      ),
                    ),
                    const SizedBox(height: 12),
                  ],
                  if (sess != null) ...[
                    Row(
                      children: [
                        const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        ),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Text(
                            L10n.t('mail_oauth_waiting'),
                            style: TextStyle(fontSize: 13, color: c.textMain),
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 14),
                    if (sess.userCode.isNotEmpty) ...[
                      Text(
                        L10n.t('mail_oauth_device_code_label'),
                        style: TextStyle(fontSize: 12, color: c.textDim),
                      ),
                      const SizedBox(height: 4),
                      SelectableText(
                        sess.userCode,
                        style: TextStyle(
                          fontSize: 20,
                          fontWeight: FontWeight.w700,
                          color: MemoTheme.accent,
                        ),
                      ),
                      const SizedBox(height: 12),
                    ],
                    Wrap(
                      spacing: 8,
                      runSpacing: 8,
                      children: [
                        ElevatedButton.icon(
                          onPressed: () {
                            final uri = Uri.tryParse(sess.authUrl);
                            if (uri != null) {
                              launchUrl(
                                uri,
                                mode: LaunchMode.externalApplication,
                              );
                            }
                          },
                          icon: const Icon(Icons.open_in_new, size: 16),
                          label: Text(L10n.t('mail_oauth_open_browser')),
                        ),
                        OutlinedButton.icon(
                          onPressed: () async {
                            final ok = await copyToClipboard(sess.authUrl);
                            if (!context.mounted) return;
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(
                                content: Text(
                                  L10n.t(
                                    ok
                                        ? 'mail_oauth_url_copied'
                                        : 'copy_failed',
                                  ),
                                ),
                              ),
                            );
                          },
                          icon: const Icon(Icons.copy, size: 16),
                          label: Text(L10n.t('mail_oauth_copy_url')),
                        ),
                      ],
                    ),
                    const SizedBox(height: 16),
                    Text(
                      L10n.t('mail_oauth_manual_hint'),
                      style: TextStyle(fontSize: 12, color: c.textDim),
                    ),
                    const SizedBox(height: 8),
                    Row(
                      children: [
                        Expanded(
                          child: TextField(
                            controller: _manualController,
                            style: TextStyle(fontSize: 12, color: c.textMain),
                            decoration: const InputDecoration(isDense: true),
                          ),
                        ),
                        const SizedBox(width: 8),
                        OutlinedButton(
                          onPressed:
                              _submittingManual ? null : _submitManualCallback,
                          child: Text(L10n.t('mail_oauth_manual_submit')),
                        ),
                      ],
                    ),
                  ],
                ],
              ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(L10n.t('cancel')),
        ),
      ],
    );
  }
}

class _MailCustomConnectDialog extends ConsumerStatefulWidget {
  const _MailCustomConnectDialog();

  @override
  ConsumerState<_MailCustomConnectDialog> createState() =>
      _MailCustomConnectDialogState();
}

class _MailCustomConnectDialogState
    extends ConsumerState<_MailCustomConnectDialog> {
  final TextEditingController _emailCtrl = TextEditingController();
  final TextEditingController _passwordCtrl = TextEditingController();
  final TextEditingController _displayNameCtrl = TextEditingController();
  final TextEditingController _imapHostCtrl = TextEditingController();
  final TextEditingController _imapPortCtrl = TextEditingController(text: '993');
  final TextEditingController _smtpHostCtrl = TextEditingController();
  final TextEditingController _smtpPortCtrl = TextEditingController(text: '587');

  bool _discovering = false;
  bool _connecting = false;
  String? _error;

  @override
  void dispose() {
    _emailCtrl.dispose();
    _passwordCtrl.dispose();
    _displayNameCtrl.dispose();
    _imapHostCtrl.dispose();
    _imapPortCtrl.dispose();
    _smtpHostCtrl.dispose();
    _smtpPortCtrl.dispose();
    super.dispose();
  }

  Future<void> _autodiscover() async {
    final email = _emailCtrl.text.trim();
    if (email.isEmpty || _discovering) return;
    setState(() {
      _discovering = true;
      _error = null;
    });
    try {
      final cfg = await ref.read(apiClientProvider).autodiscoverMail(email);
      if (!mounted) return;
      setState(() {
        _discovering = false;
        if (cfg.imapHost.isNotEmpty) _imapHostCtrl.text = cfg.imapHost;
        if (cfg.imapPort > 0) _imapPortCtrl.text = '${cfg.imapPort}';
        if (cfg.smtpHost.isNotEmpty) _smtpHostCtrl.text = cfg.smtpHost;
        if (cfg.smtpPort > 0) _smtpPortCtrl.text = '${cfg.smtpPort}';
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _discovering = false;
        _error = FriendlyError.describeGeneric(e);
      });
    }
  }

  Future<void> _connect() async {
    final email = _emailCtrl.text.trim();
    final password = _passwordCtrl.text;
    if (email.isEmpty || password.isEmpty || _connecting) return;
    setState(() {
      _connecting = true;
      _error = null;
    });
    try {
      await ref.read(apiClientProvider).connectMailPassword(
            email: email,
            password: password,
            displayName: _displayNameCtrl.text.trim(),
            imapHost: _imapHostCtrl.text.trim(),
            imapPort: int.tryParse(_imapPortCtrl.text.trim()) ?? 993,
            smtpHost: _smtpHostCtrl.text.trim(),
            smtpPort: int.tryParse(_smtpPortCtrl.text.trim()) ?? 587,
          );
      if (!mounted) return;
      Navigator.of(context).pop(true);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _connecting = false;
        _error = FriendlyError.describeGeneric(e);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(L10n.t('mail_custom_dialog_title')),
      content: SizedBox(
        width: 460,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (_error != null) ...[
                Text(
                  L10n.t('mail_error_prefix', {'err': _error!}),
                  style: const TextStyle(color: MemoTheme.red, fontSize: 12),
                ),
                const SizedBox(height: 12),
              ],
              Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _emailCtrl,
                      decoration: InputDecoration(
                        labelText: L10n.t('mail_custom_email_label'),
                        isDense: true,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  OutlinedButton(
                    onPressed: _discovering ? null : _autodiscover,
                    child: Text(L10n.t('mail_custom_autodiscover_btn')),
                  ),
                ],
              ),
              const SizedBox(height: 10),
              TextField(
                controller: _passwordCtrl,
                obscureText: true,
                decoration: InputDecoration(
                  labelText: L10n.t('mail_custom_password_label'),
                  isDense: true,
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: _displayNameCtrl,
                decoration: InputDecoration(
                  labelText: L10n.t('mail_custom_display_name_label'),
                  isDense: true,
                ),
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    flex: 3,
                    child: TextField(
                      controller: _imapHostCtrl,
                      decoration: InputDecoration(
                        labelText: L10n.t('mail_custom_imap_host'),
                        isDense: true,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: TextField(
                      controller: _imapPortCtrl,
                      decoration: InputDecoration(
                        labelText: L10n.t('mail_custom_imap_port'),
                        isDense: true,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 10),
              Row(
                children: [
                  Expanded(
                    flex: 3,
                    child: TextField(
                      controller: _smtpHostCtrl,
                      decoration: InputDecoration(
                        labelText: L10n.t('mail_custom_smtp_host'),
                        isDense: true,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: TextField(
                      controller: _smtpPortCtrl,
                      decoration: InputDecoration(
                        labelText: L10n.t('mail_custom_smtp_port'),
                        isDense: true,
                      ),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(L10n.t('cancel')),
        ),
        ElevatedButton(
          onPressed: _connecting ? null : _connect,
          child: Text(L10n.t('mail_custom_connect_btn')),
        ),
      ],
    );
  }
}

class _MailComposeDialog extends ConsumerStatefulWidget {
  final String accountId;
  final String initialTo;
  final String initialSubject;
  final String inReplyTo;
  final void Function(String message, {bool isError}) onToast;

  const _MailComposeDialog({
    required this.accountId,
    required this.initialTo,
    required this.initialSubject,
    required this.inReplyTo,
    required this.onToast,
  });

  @override
  ConsumerState<_MailComposeDialog> createState() => _MailComposeDialogState();
}

class _MailComposeDialogState extends ConsumerState<_MailComposeDialog> {
  late final TextEditingController _toCtrl;
  final TextEditingController _ccCtrl = TextEditingController();
  late final TextEditingController _subjectCtrl;
  final TextEditingController _bodyCtrl = TextEditingController();
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _toCtrl = TextEditingController(text: widget.initialTo);
    _subjectCtrl = TextEditingController(text: widget.initialSubject);
  }

  @override
  void dispose() {
    _toCtrl.dispose();
    _ccCtrl.dispose();
    _subjectCtrl.dispose();
    _bodyCtrl.dispose();
    super.dispose();
  }

  List<String> _splitAddresses(String raw) {
    return raw
        .split(',')
        .map((s) => s.trim())
        .where((s) => s.isNotEmpty)
        .toList();
  }

  Future<void> _send() async {
    final to = _splitAddresses(_toCtrl.text);
    if (to.isEmpty || _busy) return;
    setState(() => _busy = true);
    try {
      await ref.read(apiClientProvider).sendMail(
            accountId: widget.accountId,
            to: to,
            cc: _splitAddresses(_ccCtrl.text),
            subject: _subjectCtrl.text.trim(),
            body: _bodyCtrl.text,
            inReplyTo: widget.inReplyTo,
          );
      if (!mounted) return;
      widget.onToast(L10n.t('mail_sent_success'));
      Navigator.of(context).pop();
    } catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      widget.onToast(
        L10n.t('mail_error_prefix', {'err': describeMailError(e)}),
        isError: true,
      );
    }
  }

  Future<void> _saveDraft() async {
    final to = _splitAddresses(_toCtrl.text);
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final folder = await ref.read(apiClientProvider).saveMailDraft(
            accountId: widget.accountId,
            to: to,
            cc: _splitAddresses(_ccCtrl.text),
            subject: _subjectCtrl.text.trim(),
            body: _bodyCtrl.text,
            inReplyTo: widget.inReplyTo,
          );
      if (!mounted) return;
      widget.onToast(L10n.t('mail_draft_saved', {'folder': folder}));
      Navigator.of(context).pop();
    } catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      widget.onToast(
        L10n.t('mail_error_prefix', {'err': describeMailError(e)}),
        isError: true,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(L10n.t('mail_compose')),
      content: SizedBox(
        width: 540,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextField(
                controller: _toCtrl,
                decoration: InputDecoration(
                  labelText: L10n.t('mail_to'),
                  isDense: true,
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: _ccCtrl,
                decoration: InputDecoration(
                  labelText: L10n.t('mail_cc'),
                  isDense: true,
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: _subjectCtrl,
                decoration: InputDecoration(
                  labelText: L10n.t('mail_subject'),
                  isDense: true,
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: _bodyCtrl,
                maxLines: 8,
                decoration: InputDecoration(
                  hintText: L10n.t('mail_body_hint'),
                ),
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
          child: Text(L10n.t('cancel')),
        ),
        OutlinedButton.icon(
          onPressed: _busy ? null : _saveDraft,
          icon: const Icon(Icons.drafts_outlined, size: 16),
          label: Text(L10n.t('mail_save_draft')),
        ),
        ElevatedButton.icon(
          onPressed: _busy ? null : _send,
          icon: const Icon(Icons.send, size: 16),
          label: Text(L10n.t('mail_send')),
        ),
      ],
    );
  }
}
