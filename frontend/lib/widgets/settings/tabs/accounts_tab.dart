import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api_client.dart';
import '../../../core/friendly_error.dart';
import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../providers/auth_gate_provider.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/permissions_provider.dart';
import '../../../providers/settings_provider.dart';

/// Settings tab: account list + add/delete + password change + sign out.
/// Management actions require an admin session; a "user"-role session only
/// sees its own password change (admin-only note otherwise). Local desktop
/// (no account session at all) is treated as admin — callerIsAdmin allows
/// credential-less callers, so every action works there.
class AccountsTab extends ConsumerStatefulWidget {
  const AccountsTab({super.key});

  @override
  ConsumerState<AccountsTab> createState() => _AccountsTabState();
}

class _AccountsTabState extends ConsumerState<AccountsTab> {
  List<Map<String, dynamic>>? _accounts;
  String? _error;
  String? _selfUsername;

  bool get _canManage =>
      (ref.read(prefsProvider).getString('memo_session_role') ?? 'admin') ==
      'admin';

  bool get _hasSession =>
      ref.read(prefsProvider).getString('memo_session_role') != null;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  Future<void> _load() async {
    try {
      final accounts = await ref.read(apiClientProvider).listAccounts();
      if (!mounted) return;
      setState(() {
        _accounts = accounts;
        _error = null;
        // Desktop has no "self" account identity (no session); a logged-in
        // session matches its own account by username so self password
        // changes can skip the current-password field.
        _selfUsername =
            ref.read(prefsProvider).getString('memo_session_username');
      });
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = '$e');
    }
  }

  Future<void> _addAccount() async {
    await showDialog<void>(
      context: context,
      builder: (ctx) => _AddAccountDialog(onDone: _load),
    );
  }

  Future<void> _deleteAccount(Map<String, dynamic> acc) async {
    final id = acc['id'] as String? ?? '';
    final name = acc['username'] as String? ?? '';
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('accounts_delete_confirm_title')),
        content: Text(L10n.t('accounts_delete_confirm_body', {'name': name})),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(
              L10n.t('accounts_delete'),
              style: const TextStyle(color: MemoTheme.red),
            ),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(apiClientProvider).deleteAccount(id);
      await _load();
    } catch (e) {
      if (!mounted) return;
      final msg = FriendlyError.describeGeneric(e);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            msg.contains('cannot remove the last admin')
                ? L10n.t('accounts_delete_last_admin_error')
                : L10n.t('accounts_delete_failed', {'err': msg}),
          ),
        ),
      );
    }
  }

  Future<void> _changePassword(Map<String, dynamic> acc) async {
    final isSelf =
        _selfUsername != null && acc['username'] == _selfUsername;
    final messenger = ScaffoldMessenger.of(context);
    await showDialog<void>(
      context: context,
      builder: (ctx) => _ChangePasswordDialog(
        accountId: acc['id'] as String? ?? '',
        accountName: acc['username'] as String? ?? '',
        isSelf: isSelf,
        onChanged: () => messenger.showSnackBar(
          SnackBar(content: Text(L10n.t('accounts_password_changed'))),
        ),
      ),
    );
  }

  Future<void> _editPermissions(Map<String, dynamic> acc) async {
    await showDialog<void>(
      context: context,
      builder: (ctx) => _EditPermissionsDialog(
        accountId: acc['id'] as String? ?? '',
        accountName: acc['username'] as String? ?? '',
        initial: AccountPermissions.fromJson(
          acc['permissions'] as Map<String, dynamic>?,
        ),
        onDone: _load,
      ),
    );
  }

  Future<void> _signOut() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('accounts_sign_out_confirm_title')),
        content: Text(L10n.t('accounts_sign_out_confirm_body')),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(L10n.t('cancel')),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(L10n.t('accounts_sign_out')),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    final prefs = ref.read(prefsProvider);
    await prefs.remove('memo_remote_access_token');
    await prefs.remove('memo_session_role');
    await prefs.remove('memo_session_username');
    await prefs.remove(memoSessionPermissionsKey);
    if (mounted) {
      setState(() {}); // _hasSession/_selfUsername rebuild from cleared prefs
      ref.invalidate(authGateProvider);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);

    if (_error != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                L10n.t('accounts_loaded_error', {'err': _error!}),
                textAlign: TextAlign.center,
                style: TextStyle(color: theme.textDim, fontSize: 13),
              ),
              const SizedBox(height: 12),
              OutlinedButton(
                onPressed: _load,
                child: Text(L10n.t('retry')),
              ),
            ],
          ),
        ),
      );
    }
    if (_accounts == null) {
      return const Center(child: CircularProgressIndicator());
    }

    final selfUsername =
        _canManage ? null : ref.read(prefsProvider).getString('memo_session_username');
    final rows = _canManage
        ? _accounts!
        : _accounts!
            .where((a) => a['username'] == selfUsername)
            .toList();

    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        if (!_canManage) ...[
          Text(
            L10n.t('accounts_admin_only_note'),
            style: TextStyle(color: theme.textDim, fontSize: 13),
          ),
          const SizedBox(height: 16),
        ],
        if (_accounts!.isEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: 16),
            child: Text(
              L10n.t('accounts_empty'),
              style: TextStyle(color: theme.textDim, fontSize: 13),
            ),
          ),
        for (final acc in rows)
          Card(
            color: theme.bgPanel,
            margin: const EdgeInsets.only(bottom: 8),
            child: ListTile(
              leading: CircleAvatar(
                backgroundColor: theme.bgElement,
                child: Icon(Icons.person_outline,
                    size: 20, color: theme.textMain),
              ),
              title: Text(acc['username'] as String? ?? ''),
              subtitle: Chip(
                label: Text(
                  (acc['role'] ?? 'user') == 'admin'
                      ? L10n.t('accounts_role_admin')
                      : L10n.t('accounts_role_user'),
                  style: const TextStyle(fontSize: 11),
                ),
                visualDensity: VisualDensity.compact,
              ),
              trailing: _canManage
                  ? Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        if ((acc['role'] ?? 'user') == 'user')
                          IconButton(
                            icon: const Icon(Icons.tune, size: 18),
                            tooltip: L10n.t('accounts_edit_permissions'),
                            onPressed: () => _editPermissions(acc),
                          ),
                        IconButton(
                          icon: const Icon(Icons.lock_outline, size: 18),
                          tooltip: L10n.t('accounts_change_password'),
                          onPressed: () => _changePassword(acc),
                        ),
                        IconButton(
                          icon: const Icon(Icons.delete_outline, size: 18),
                          tooltip: L10n.t('accounts_delete'),
                          onPressed: () => _deleteAccount(acc),
                        ),
                      ],
                    )
                  : IconButton(
                      icon: const Icon(Icons.lock_outline, size: 18),
                      tooltip: L10n.t('accounts_change_password'),
                      onPressed: () => _changePassword(acc),
                    ),
            ),
          ),
        if (_canManage) ...[
          const SizedBox(height: 8),
          OutlinedButton.icon(
            onPressed: _addAccount,
            icon: const Icon(Icons.person_add_alt_1, size: 18),
            label: Text(L10n.t('accounts_add')),
          ),
          const SizedBox(height: 24),
        ],
        if (_hasSession)
          OutlinedButton.icon(
            onPressed: _signOut,
            icon: const Icon(Icons.logout, size: 18),
            label: Text(L10n.t('accounts_sign_out')),
          ),
      ],
    );
  }
}

/// Modal "new account" form — owns its controllers so they live and die
/// with the dialog's own widget tree (disposing them in the caller after
/// showDialog() returns races the pop animation, which still reads them).
class _AddAccountDialog extends ConsumerStatefulWidget {
  const _AddAccountDialog({required this.onDone});

  /// Called after a successful POST, so the parent tab can refresh.
  final Future<void> Function() onDone;

  @override
  ConsumerState<_AddAccountDialog> createState() => _AddAccountDialogState();
}

class _AddAccountDialogState extends ConsumerState<_AddAccountDialog> {
  final _username = TextEditingController();
  final _password = TextEditingController();
  String _role = 'user';
  AccountPermissions _permissions = const AccountPermissions();
  String? _error;

  @override
  void dispose() {
    _username.dispose();
    _password.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return AlertDialog(
      title: Text(L10n.t('accounts_add_dialog_title')),
      content: SizedBox(
        width: 360,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (_error != null)
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: Text(
                    _error!,
                    style: TextStyle(color: MemoTheme.red, fontSize: 13),
                  ),
                ),
              TextField(
                controller: _username,
                autofocus: true,
                decoration: InputDecoration(
                    labelText: L10n.t('accounts_add_username')),
              ),
              const SizedBox(height: 8),
              TextField(
                controller: _password,
                obscureText: true,
                decoration: InputDecoration(
                    labelText: L10n.t('accounts_add_password')),
              ),
              const SizedBox(height: 8),
              DropdownButtonFormField<String>(
                initialValue: _role,
                decoration:
                    InputDecoration(labelText: L10n.t('accounts_add_role')),
                items: [
                  DropdownMenuItem(
                    value: 'admin',
                    child: Text(L10n.t('accounts_role_admin')),
                  ),
                  DropdownMenuItem(
                    value: 'user',
                    child: Text(L10n.t('accounts_role_user')),
                  ),
                ],
                onChanged: (v) => setState(() => _role = v ?? 'user'),
              ),
              if (_role == 'user') ...[
                const SizedBox(height: 16),
                Text(
                  L10n.t('accounts_permissions_hint'),
                  style: TextStyle(color: theme.textDim, fontSize: 12),
                ),
                const SizedBox(height: 4),
                _PermissionCheckboxesEditor(
                  initial: _permissions,
                  onChanged: (p) => _permissions = p,
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(L10n.t('cancel')),
        ),
        TextButton(
          onPressed: () async {
            try {
              await ref.read(apiClientProvider).createAccount(
                    _username.text.trim(),
                    _password.text,
                    _role,
                    permissions: _permissions,
                  );
              if (!context.mounted) return;
              Navigator.of(context).pop();
              await widget.onDone();
            } catch (e) {
              if (!mounted) return;
              setState(() => _error = L10n.t('accounts_add_failed', {
                    'err': FriendlyError.describeGeneric(e),
                  }));
            }
          },
          child: Text(L10n.t('accounts_add_submit')),
        ),
      ],
    );
  }
}

/// Modal "edit permissions" form for an existing "user"-role account
/// (Faz 5.1.1, yapacam.md) — the create dialog above covers setting these
/// at creation time, this covers changing them afterward.
class _EditPermissionsDialog extends ConsumerStatefulWidget {
  const _EditPermissionsDialog({
    required this.accountId,
    required this.accountName,
    required this.initial,
    required this.onDone,
  });

  final String accountId;
  final String accountName;
  final AccountPermissions initial;
  final Future<void> Function() onDone;

  @override
  ConsumerState<_EditPermissionsDialog> createState() =>
      _EditPermissionsDialogState();
}

class _EditPermissionsDialogState
    extends ConsumerState<_EditPermissionsDialog> {
  late AccountPermissions _permissions = widget.initial;
  String? _error;

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(
        L10n.t('accounts_permissions_dialog_title',
            {'name': widget.accountName}),
      ),
      content: SizedBox(
        width: 360,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (_error != null)
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: Text(
                    _error!,
                    style: TextStyle(color: MemoTheme.red, fontSize: 13),
                  ),
                ),
              _PermissionCheckboxesEditor(
                initial: _permissions,
                onChanged: (p) => _permissions = p,
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(L10n.t('cancel')),
        ),
        TextButton(
          onPressed: () async {
            try {
              await ref
                  .read(apiClientProvider)
                  .updateAccountPermissions(widget.accountId, _permissions);
              if (!context.mounted) return;
              Navigator.of(context).pop();
              await widget.onDone();
            } catch (e) {
              if (!mounted) return;
              setState(() => _error = L10n.t('accounts_permissions_failed', {
                    'err': FriendlyError.describeGeneric(e),
                  }));
            }
          },
          child: Text(L10n.t('accounts_permissions_save')),
        ),
      ],
    );
  }
}

/// The seven-checkbox editor shared by the add-account and edit-permissions
/// dialogs above. Owns its own state so parent dialogs don't have to
/// individually setState() per checkbox — reports the whole
/// [AccountPermissions] back via [onChanged] on every toggle.
class _PermissionCheckboxesEditor extends StatefulWidget {
  const _PermissionCheckboxesEditor({
    required this.initial,
    required this.onChanged,
  });

  final AccountPermissions initial;
  final ValueChanged<AccountPermissions> onChanged;

  @override
  State<_PermissionCheckboxesEditor> createState() =>
      _PermissionCheckboxesEditorState();
}

class _PermissionCheckboxesEditorState
    extends State<_PermissionCheckboxesEditor> {
  late bool _models = widget.initial.models;
  late bool _memory = widget.initial.memory;
  late bool _agent = widget.initial.agent;
  late bool _calendar = widget.initial.calendar;
  late bool _whatsapp = widget.initial.whatsapp;
  late bool _telegram = widget.initial.telegram;
  late bool _routines = widget.initial.routines;

  void _emit() => widget.onChanged(AccountPermissions(
        models: _models,
        memory: _memory,
        agent: _agent,
        calendar: _calendar,
        whatsapp: _whatsapp,
        telegram: _telegram,
        routines: _routines,
      ));

  Widget _row(String label, bool value, ValueChanged<bool?> onChanged) {
    return CheckboxListTile(
      dense: true,
      contentPadding: EdgeInsets.zero,
      controlAffinity: ListTileControlAffinity.leading,
      visualDensity: VisualDensity.compact,
      title: Text(label, style: const TextStyle(fontSize: 13)),
      value: value,
      onChanged: onChanged,
    );
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        _row(L10n.t('accounts_perm_models'), _models, (v) {
          setState(() => _models = v ?? false);
          _emit();
        }),
        _row(L10n.t('accounts_perm_memory'), _memory, (v) {
          setState(() => _memory = v ?? false);
          _emit();
        }),
        _row(L10n.t('accounts_perm_agent'), _agent, (v) {
          setState(() => _agent = v ?? false);
          _emit();
        }),
        _row(L10n.t('accounts_perm_calendar'), _calendar, (v) {
          setState(() => _calendar = v ?? false);
          _emit();
        }),
        _row(L10n.t('accounts_perm_whatsapp'), _whatsapp, (v) {
          setState(() => _whatsapp = v ?? false);
          _emit();
        }),
        _row(L10n.t('accounts_perm_telegram'), _telegram, (v) {
          setState(() => _telegram = v ?? false);
          _emit();
        }),
        _row(L10n.t('accounts_perm_routines'), _routines, (v) {
          setState(() => _routines = v ?? false);
          _emit();
        }),
      ],
    );
  }
}

/// Modal password-change form. Self-changes (matched by username in the
/// saved session) ask for the current password; an admin changing another
/// account only enters the new one.
class _ChangePasswordDialog extends ConsumerStatefulWidget {
  const _ChangePasswordDialog({
    required this.accountId,
    required this.accountName,
    required this.isSelf,
    required this.onChanged,
  });

  final String accountId;
  final String accountName;
  final bool isSelf;

  /// Invoked from the owner's (still-mounted) context after a successful
  /// change — the dialog's own context is gone by then (pop animation
  /// already started), so the SnackBar must be shown via the caller.
  final VoidCallback onChanged;

  @override
  ConsumerState<_ChangePasswordDialog> createState() =>
      _ChangePasswordDialogState();
}

class _ChangePasswordDialogState extends ConsumerState<_ChangePasswordDialog> {
  final _current = TextEditingController();
  final _newPassword = TextEditingController();
  String? _error;

  @override
  void dispose() {
    _current.dispose();
    _newPassword.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(
        L10n.t('accounts_password_dialog_title', {'name': widget.accountName}),
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: Text(
                _error!,
                style: TextStyle(color: MemoTheme.red, fontSize: 13),
              ),
            ),
          if (widget.isSelf) ...[
            TextField(
              controller: _current,
              obscureText: true,
              decoration:
                  InputDecoration(labelText: L10n.t('accounts_current_password')),
            ),
            const SizedBox(height: 8),
          ],
          TextField(
            controller: _newPassword,
            obscureText: true,
            autofocus: !widget.isSelf,
            decoration:
                InputDecoration(labelText: L10n.t('accounts_new_password')),
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(L10n.t('cancel')),
        ),
        TextButton(
          onPressed: () async {
            try {
              await ref.read(apiClientProvider).changeAccountPassword(
                    widget.accountId,
                    currentPassword: widget.isSelf ? _current.text : '',
                    newPassword: _newPassword.text,
                  );
              if (!context.mounted) return;
              Navigator.of(context).pop();
              widget.onChanged();
            } catch (e) {
              if (!mounted) return;
              setState(() => _error = L10n.t('accounts_password_failed', {
                    'err': FriendlyError.describeGeneric(e),
                  }));
            }
          },
          child: Text(L10n.t('accounts_password_submit')),
        ),
      ],
    );
  }
}
