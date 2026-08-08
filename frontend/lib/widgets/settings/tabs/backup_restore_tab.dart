import 'package:flutter/material.dart';
import 'dart:io';
import 'package:url_launcher/url_launcher.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import 'dart:async';
import 'package:file_picker/file_picker.dart';
import '../../../providers/chat_provider.dart';
import '../../../core/friendly_error.dart';

class BackupRestoreTab extends ConsumerStatefulWidget {
  const BackupRestoreTab({super.key});

  @override
  ConsumerState<BackupRestoreTab> createState() => BackupRestoreTabState();
}

class BackupRestoreTabState extends ConsumerState<BackupRestoreTab> {
  // ── Yerel yedekleme durumu ──────────────────────────────────────
  bool _exporting = false;
  bool _importing = false;
  bool _includeModels = false;
  bool _wiping = false;
  bool _wipeConfirm1 = false;
  bool _wipeConfirm2 = false;

  // ── Bulut yedekleme durumu ──────────────────────────────────────
  bool _cloudConnected = false;
  String _cloudName = '';
  String _cloudEmail = '';
  String _cloudOp = ''; // 'connecting' | 'saving' | 'backup' | 'restore' | ''
  bool _showCredSetup = false;
  final _clientIdCtrl = TextEditingController();
  final _clientSecretCtrl = TextEditingController();
  final _passphraseCtrl = TextEditingController();
  Timer? _authPollTimer;

  @override
  void initState() {
    super.initState();
    _loadCloudStatus();
  }

  @override
  void dispose() {
    _authPollTimer?.cancel();
    _clientIdCtrl.dispose();
    _clientSecretCtrl.dispose();
    _passphraseCtrl.dispose();
    super.dispose();
  }

  // ── Bulut metodları ─────────────────────────────────────────────

  Future<void> _loadCloudStatus() async {
    try {
      final api = ref.read(apiClientProvider);
      final connected = await api.checkSyncAuth();
      if (!mounted) return;
      if (connected) {
        final account = await api.getSyncAccount();
        final settings = await api.getSyncSettings();
        if (!mounted) return;
        setState(() {
          _cloudConnected = true;
          _cloudName = account['name'] as String? ?? '';
          _cloudEmail = account['email'] as String? ?? '';
          _clientIdCtrl.text = settings['client_id'] as String? ?? '';
          _clientSecretCtrl.text = settings['client_secret'] as String? ?? '';
          _passphraseCtrl.text = settings['passphrase'] as String? ?? '';
        });
      } else {
        final settings = await api.getSyncSettings();
        if (!mounted) return;
        setState(() {
          _cloudConnected = false;
          _clientIdCtrl.text = settings['client_id'] as String? ?? '';
          _clientSecretCtrl.text = settings['client_secret'] as String? ?? '';
          _passphraseCtrl.text = settings['passphrase'] as String? ?? '';
        });
      }
    } catch (_) {}
  }

  /// If the passphrase field is empty, warns the user that their backup will
  /// be encrypted with a key derived from this device's machine ID and will
  /// not be restorable from any other device. Returns true if the user
  /// explicitly chooses to proceed anyway (or if a passphrase is set).
  Future<bool> _confirmEmptyPassphraseIfNeeded() async {
    if (_passphraseCtrl.text.trim().isNotEmpty) return true;
    final proceed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('backup_passphrase_warning_title')),
        content: Text(L10n.t('backup_passphrase_warning_body')),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: Text(L10n.t('backup_set_passphrase_btn')),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: TextButton.styleFrom(foregroundColor: MemoTheme.warmBrown),
            child: Text(L10n.t('backup_device_specific_btn')),
          ),
        ],
      ),
    );
    return proceed == true;
  }

  Future<void> _saveCredentials() async {
    if (!await _confirmEmptyPassphraseIfNeeded()) return;
    setState(() { _cloudOp = 'saving'; });
    try {
      await ref.read(apiClientProvider).updateSyncSettings(
        enabled: true,
        clientId: _clientIdCtrl.text.trim(),
        clientSecret: _clientSecretCtrl.text.trim(),
        passphrase: _passphraseCtrl.text.trim(),
        tokenPath: '',
        intervalMessages: 50,
      );
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('backup_creds_saved'))),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('backup_save_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
    } finally {
      if (mounted) setState(() { _cloudOp = ''; });
    }
  }

  Future<void> _connectDrive() async {
    if (_clientIdCtrl.text.trim().isEmpty || _clientSecretCtrl.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(L10n.t('backup_enter_creds_first'))),
      );
      return;
    }
    if (!await _confirmEmptyPassphraseIfNeeded()) return;
    setState(() { _cloudOp = 'connecting'; });
    try {
      await ref.read(apiClientProvider).updateSyncSettings(
        enabled: true,
        clientId: _clientIdCtrl.text.trim(),
        clientSecret: _clientSecretCtrl.text.trim(),
        passphrase: _passphraseCtrl.text.trim(),
        tokenPath: '',
        intervalMessages: 50,
      );
      final api = ref.read(apiClientProvider);
      final url = await api.startSyncAuth();
      if (url.isNotEmpty) {
        await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
      }
      // Yetkilendirme tamamlanana kadar her 3 saniyede bir sorgula (max 2 dk)
      int attempts = 0;
      int consecutiveFailures = 0;
      const maxConsecutiveFailures = 4;
      _authPollTimer?.cancel();
      _authPollTimer = Timer.periodic(const Duration(seconds: 3), (t) async {
        attempts++;
        if (attempts > 40) {
          t.cancel();
          if (mounted) {
            setState(() { _cloudOp = ''; });
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text(L10n.t('backup_auth_timeout'))),
            );
          }
          return;
        }
        try {
          final done = await ref.read(apiClientProvider).checkSyncAuth();
          consecutiveFailures = 0;
          if (done) {
            t.cancel();
            await _loadCloudStatus();
            if (mounted) setState(() { _cloudOp = ''; });
          }
        } catch (_) {
          consecutiveFailures++;
          if (consecutiveFailures >= maxConsecutiveFailures) {
            t.cancel();
            if (mounted) {
              setState(() { _cloudOp = ''; });
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(
                  content: Text(L10n.t('backup_auth_check_failed')),
                ),
              );
            }
          }
        }
      });
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('backup_connection_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
        setState(() { _cloudOp = ''; });
      }
    }
  }

  Future<void> _backupNow() async {
    setState(() { _cloudOp = 'backup'; });
    try {
      await ref.read(apiClientProvider).triggerSync();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('backup_drive_started'))),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('backup_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
    } finally {
      if (mounted) setState(() { _cloudOp = ''; });
    }
  }

  Future<void> _restoreCloud() async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('backup_restore_cloud_title')),
        content: Text(L10n.t('backup_restore_cloud_confirm')),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: Text(L10n.t('cancel'))),
          TextButton(onPressed: () => Navigator.pop(ctx, true), child: Text(L10n.t('backup_restore_btn_short'))),
        ],
      ),
    );
    if (confirm != true) return;
    setState(() { _cloudOp = 'restore'; });
    try {
      await ref.read(apiClientProvider).pullSync();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('backup_restore_started'))),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('backup_restore_error', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
    } finally {
      if (mounted) setState(() { _cloudOp = ''; });
    }
  }

  Future<void> _disconnectCloud() async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(L10n.t('backup_disconnect_drive_title')),
        content: Text(L10n.t('backup_disconnect_drive_body')),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: Text(L10n.t('cancel'))),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: TextButton.styleFrom(foregroundColor: MemoTheme.red),
            child: Text(L10n.t('backup_disconnect_btn')),
          ),
        ],
      ),
    );
    if (confirm != true) return;
    try {
      await ref.read(apiClientProvider).disconnectSync();
      if (mounted) {
        setState(() {
          _cloudConnected = false;
          _cloudName = '';
          _cloudEmail = '';
        });
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('backup_disconnected'))),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(L10n.t('backup_error_generic', {'e': FriendlyError.describeGeneric(e)}))),
        );
      }
    }
  }

  // ── Yerel yedekleme metodları ───────────────────────────────────

  Future<void> _export() async {
    if (_exporting) return;
    setState(() => _exporting = true);
    try {
      final api = ref.read(apiClientProvider);
      final data = await api.exportData(includeModels: _includeModels);
      if (!mounted) return;

      final path = await FilePicker.platform.saveFile(
        dialogTitle: L10n.t('backup_export_dialog_title'),
        fileName: 'memo_backup.memo',
        type: FileType.any,
      );
      if (path != null) {
        await File(path).writeAsBytes(data);
        if (mounted) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text(L10n.t('backup_export_saved', {'path': path}))));
        }
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(L10n.t('backup_export_error', {'e': FriendlyError.describeGeneric(e)}))));
      }
    } finally {
      if (mounted) setState(() => _exporting = false);
    }
  }

  Future<void> _import() async {
    if (_importing) return;
    setState(() => _importing = true);
    try {
      final result = await FilePicker.platform.pickFiles(
        dialogTitle: L10n.t('backup_import_dialog_title'),
        type: FileType.any,
      );
      if (result == null || result.files.isEmpty) return;

      final bytes = result.files.first.bytes;
      if (bytes == null) {
        final path = result.files.first.path;
        if (path == null) return;
        final file = File(path);
        if (!await file.exists()) return;
        final data = await file.readAsBytes();
        await ref.read(apiClientProvider).importData(data);
      } else {
        await ref.read(apiClientProvider).importData(bytes);
      }

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(L10n.t('backup_import_success')),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(L10n.t('backup_import_error', {'e': FriendlyError.describeGeneric(e)}))));
      }
    } finally {
      if (mounted) setState(() => _importing = false);
    }
  }

  Future<void> _wipe() async {
    if (_wiping) return;
    setState(() => _wiping = true);
    try {
      await ref.read(apiClientProvider).wipeData();
      if (mounted) {
        await _showRestartDialog();
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(L10n.t('backup_wipe_error', {'e': FriendlyError.describeGeneric(e)}))));
      }
    } finally {
      if (mounted) {
        setState(() {
          _wiping = false;
          _wipeConfirm1 = false;
          _wipeConfirm2 = false;
        });
      }
    }
  }

  /// Shown after a successful wipe. Some backend subsystems (mood, calendar,
  /// observer, WhatsApp) keep their SQLite files open, so a full restart is the
  /// only way to guarantee a clean state. Exiting with code 42 signals the
  /// launcher (run_memo.sh) to relaunch the backend + frontend from scratch.
  Future<void> _showRestartDialog() async {
    final theme = MemoTheme.of(context);
    await showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => AlertDialog(
        backgroundColor: theme.bgPanel,
        title: Text(
          L10n.t('backup_restart_title'),
          style: TextStyle(
            color: theme.textMain,
            fontWeight: FontWeight.bold,
          ),
        ),
        content: Text(
          L10n.t('backup_restart_body'),
          style: TextStyle(color: theme.textDim, fontSize: 13),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(
              L10n.t('backup_restart_later'),
              style: TextStyle(color: theme.textDim),
            ),
          ),
          TextButton(
            // Signal the backend to clean up (llama, STT, ngrok, etc.)
            // before exiting. Exit code 42 → run_memo.sh relaunches.
            onPressed: () async {
              await ref.read(apiClientProvider).shutdown();
              exit(42);
            },
            child: Text(
              L10n.t('backup_restart_now'),
              style: TextStyle(
                color: MemoTheme.accent,
                fontWeight: FontWeight.bold,
              ),
            ),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('backup_section_title'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Text(
          L10n.t('backup_section_desc'),
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 24),

        // Include models toggle
        Card(
          child: SwitchListTile(
            title: Text(L10n.t('backup_include_models')),
            subtitle: Text(
              L10n.t('backup_include_models_sub'),
              style: TextStyle(
                fontSize: 12,
                color: MemoTheme.of(context).textDim,
              ),
            ),
            value: _includeModels,
            onChanged: (v) => setState(() => _includeModels = v),
            secondary: Icon(Icons.model_training, color: MemoTheme.accent),
          ),
        ),
        SizedBox(height: 12),

        // Export
        Card(
          child: ListTile(
            leading: Icon(Icons.file_upload_outlined, color: MemoTheme.accent),
            title: Text(L10n.t('backup_export_btn')),
            subtitle: Text(
              L10n.t('backup_export_desc'),
              style: TextStyle(
                fontSize: 12,
                color: MemoTheme.of(context).textDim,
              ),
            ),
            trailing: _exporting
                ? SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : Icon(Icons.download, color: MemoTheme.of(context).textDim),
            onTap: _exporting ? null : _export,
          ),
        ),
        SizedBox(height: 12),

        // Import
        Card(
          child: ListTile(
            leading: Icon(
              Icons.file_download_outlined,
              color: MemoTheme.warmBrown,
            ),
            title: Text(L10n.t('backup_import_btn')),
            subtitle: Text(
              L10n.t('backup_import_desc'),
              style: TextStyle(
                fontSize: 12,
                color: MemoTheme.of(context).textDim,
              ),
            ),
            trailing: _importing
                ? SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : Icon(Icons.upload, color: MemoTheme.of(context).textDim),
            onTap: _importing ? null : _import,
          ),
        ),
        SizedBox(height: 32),

        // Wipe All Data
        Text(
          L10n.t('backup_wipe_title'),
          style: TextStyle(
            fontWeight: FontWeight.bold,
            fontSize: 16,
            color: MemoTheme.warmBrown,
          ),
        ),
        SizedBox(height: 8),
        Text(
          L10n.t('backup_wipe_desc'),
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 12),
        if (!_wipeConfirm1)
          Card(
            child: ListTile(
              leading: Icon(Icons.delete_forever, color: MemoTheme.warmBrown),
              title: Text(
                L10n.t('backup_wipe_btn'),
                style: TextStyle(color: MemoTheme.warmBrown),
              ),
              subtitle: Text(
                L10n.t('backup_wipe_irreversible'),
                style: TextStyle(
                  fontSize: 12,
                  color: MemoTheme.of(context).textDim,
                ),
              ),
              trailing: Icon(Icons.warning_amber, color: MemoTheme.warmBrown),
              onTap: () => setState(() => _wipeConfirm1 = true),
            ),
          ),
        if (_wipeConfirm1 && !_wipeConfirm2)
          Card(
            color: MemoTheme.warmBrown.withValues(alpha: 0.08),
            child: ListTile(
              leading: Icon(Icons.delete_forever, color: Colors.redAccent),
              title: Text(
                L10n.t('backup_wipe_confirm_title'),
                style: TextStyle(color: Colors.redAccent),
              ),
              subtitle: Text(
                L10n.t('backup_wipe_confirm_body'),
                style: TextStyle(
                  fontSize: 12,
                  color: MemoTheme.of(context).textDim,
                ),
              ),
              trailing: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  IconButton(
                    icon: Icon(
                      Icons.close,
                      color: MemoTheme.of(context).textDim,
                    ),
                    onPressed: () => setState(() {
                      _wipeConfirm1 = false;
                      _wipeConfirm2 = false;
                    }),
                  ),
                  Icon(Icons.warning, color: Colors.redAccent),
                ],
              ),
              onTap: () => setState(() => _wipeConfirm2 = true),
            ),
          ),
        if (_wipeConfirm2)
          Card(
            color: MemoTheme.red.withValues(alpha: 0.12),
            child: ListTile(
              leading: _wiping
                  ? SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Icon(Icons.delete_sweep, color: MemoTheme.red),
              title: Text(
                L10n.t('delete'),
                style: TextStyle(
                  color: MemoTheme.red,
                  fontWeight: FontWeight.bold,
                ),
              ),
              subtitle: Text(
                L10n.t('backup_wipe_final_confirm'),
                style: TextStyle(
                  fontSize: 12,
                  color: MemoTheme.of(context).textDim,
                ),
              ),
              trailing: IconButton(
                icon: Icon(Icons.close, color: MemoTheme.of(context).textDim),
                onPressed: () => setState(() {
                  _wipeConfirm1 = false;
                  _wipeConfirm2 = false;
                }),
              ),
              onTap: _wiping ? null : _wipe,
            ),
          ),

        // ── Bulut Yedekleme ────────────────────────────────────────
        SizedBox(height: 40),
        Divider(color: MemoTheme.of(context).borderSoft),
        SizedBox(height: 24),

        Row(
          children: [
            Icon(Icons.cloud_outlined, color: MemoTheme.accent, size: 22),
            SizedBox(width: 10),
            Text(
              L10n.t('backup_cloud_title'),
              style: TextStyle(
                fontWeight: FontWeight.bold,
                fontSize: 16,
                color: MemoTheme.of(context).textMain,
              ),
            ),
          ],
        ),
        SizedBox(height: 8),
        Text(
          L10n.t('backup_cloud_desc'),
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 20),

        // Bağlantı durumu kartı
        Container(
          padding: EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: _cloudConnected
                ? MemoTheme.green.withValues(alpha: 0.08)
                : MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(
              color: _cloudConnected
                  ? MemoTheme.green.withValues(alpha: 0.3)
                  : MemoTheme.of(context).borderSoft,
            ),
          ),
          child: Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: _cloudConnected
                      ? MemoTheme.green.withValues(alpha: 0.15)
                      : MemoTheme.of(context).bgElement,
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  _cloudConnected ? Icons.cloud_done : Icons.cloud_off,
                  color: _cloudConnected
                      ? MemoTheme.green
                      : MemoTheme.of(context).textDim,
                  size: 22,
                ),
              ),
              SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      _cloudConnected ? L10n.t('backup_drive_connected') : L10n.t('backup_drive_not_connected'),
                      style: TextStyle(
                        fontWeight: FontWeight.w600,
                        fontSize: 14,
                        color: _cloudConnected
                            ? MemoTheme.green
                            : MemoTheme.of(context).textMain,
                      ),
                    ),
                    if (_cloudConnected && _cloudEmail.isNotEmpty)
                      Padding(
                        padding: EdgeInsets.only(top: 2),
                        child: Text(
                          '$_cloudName • $_cloudEmail',
                          style: TextStyle(
                            fontSize: 12,
                            color: MemoTheme.of(context).textDim,
                          ),
                        ),
                      ),
                    if (!_cloudConnected)
                      Padding(
                        padding: EdgeInsets.only(top: 2),
                        child: Text(
                          L10n.t('backup_enter_creds_to_connect'),
                          style: TextStyle(
                            fontSize: 12,
                            color: MemoTheme.of(context).textDim,
                          ),
                        ),
                      ),
                    if (_cloudOp == 'connecting')
                      Padding(
                        padding: EdgeInsets.only(top: 4),
                        child: Row(
                          children: [
                            SizedBox(
                              width: 12,
                              height: 12,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            ),
                            SizedBox(width: 6),
                            Text(
                              L10n.t('backup_auth_waiting'),
                              style: TextStyle(
                                fontSize: 11,
                                color: MemoTheme.accent,
                              ),
                            ),
                          ],
                        ),
                      ),
                  ],
                ),
              ),
              if (_cloudConnected)
                TextButton(
                  onPressed: _cloudOp.isNotEmpty ? null : _disconnectCloud,
                  style: TextButton.styleFrom(foregroundColor: MemoTheme.red),
                  child: Text(L10n.t('backup_disconnect_short'), style: TextStyle(fontSize: 12)),
                ),
            ],
          ),
        ),

        // Kimlik bilgileri formu (bağlı değilse veya ayarlar açıksa)
        if (!_cloudConnected || _showCredSetup) ...[
          SizedBox(height: 16),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                L10n.t('backup_oauth_creds_title'),
                style: TextStyle(
                  fontWeight: FontWeight.w600,
                  fontSize: 13,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              if (_cloudConnected)
                TextButton(
                  onPressed: () => setState(() => _showCredSetup = false),
                  child: Text(L10n.t('close'), style: TextStyle(fontSize: 12)),
                ),
            ],
          ),
          SizedBox(height: 4),
          Text(
            L10n.t('backup_oauth_creds_hint'),
            style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim),
          ),
          SizedBox(height: 12),
          CloudTextField(
            label: 'Client ID',
            controller: _clientIdCtrl,
            hint: 'xxxx.apps.googleusercontent.com',
          ),
          SizedBox(height: 8),
          CloudTextField(
            label: 'Client Secret',
            controller: _clientSecretCtrl,
            hint: 'GOCSPX-...',
            obscure: true,
          ),
          SizedBox(height: 8),
          CloudTextField(
            label: L10n.t('backup_encryption_passphrase'),
            controller: _passphraseCtrl,
            hint: L10n.t('backup_passphrase_hint'),
            obscure: true,
          ),
          SizedBox(height: 12),
          Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              if (!_cloudConnected) ...[
                OutlinedButton(
                  onPressed: _cloudOp.isNotEmpty ? null : _saveCredentials,
                  child: _cloudOp == 'saving'
                      ? SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : Text(L10n.t('save')),
                ),
                SizedBox(width: 8),
                FilledButton.icon(
                  onPressed: _cloudOp.isNotEmpty ? null : _connectDrive,
                  icon: _cloudOp == 'connecting'
                      ? SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        )
                      : Icon(Icons.login, size: 16),
                  label: Text(L10n.t('backup_connect_drive_btn')),
                  style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
                ),
              ] else ...[
                FilledButton(
                  onPressed: _cloudOp.isNotEmpty ? null : _saveCredentials,
                  style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
                  child: _cloudOp == 'saving'
                      ? SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        )
                      : Text(L10n.t('backup_update_creds_btn')),
                ),
              ],
            ],
          ),
        ],

        // Bağlı iken yedekleme eylemleri
        if (_cloudConnected) ...[
          SizedBox(height: 16),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                L10n.t('backup_operations_title'),
                style: TextStyle(
                  fontWeight: FontWeight.w600,
                  fontSize: 13,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              TextButton(
                onPressed: () => setState(() => _showCredSetup = !_showCredSetup),
                child: Text(
                  _showCredSetup ? L10n.t('backup_close_settings') : L10n.t('backup_edit_creds'),
                  style: TextStyle(fontSize: 12),
                ),
              ),
            ],
          ),
          SizedBox(height: 8),

          Row(
            children: [
              Expanded(
                child: Card(
                  child: ListTile(
                    leading: _cloudOp == 'backup'
                        ? SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : Icon(Icons.cloud_upload_outlined, color: MemoTheme.accent),
                    title: Text(L10n.t('backup_backup_now'), style: TextStyle(fontSize: 14)),
                    subtitle: Text(
                      L10n.t('backup_backup_now_desc'),
                      style: TextStyle(fontSize: 11, color: MemoTheme.of(context).textDim),
                    ),
                    onTap: _cloudOp.isNotEmpty ? null : _backupNow,
                  ),
                ),
              ),
              SizedBox(width: 8),
              Expanded(
                child: Card(
                  child: ListTile(
                    leading: _cloudOp == 'restore'
                        ? SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : Icon(Icons.cloud_download_outlined, color: MemoTheme.warmBrown),
                    title: Text(L10n.t('backup_restore_btn_short'), style: TextStyle(fontSize: 14)),
                    subtitle: Text(
                      L10n.t('backup_restore_desc'),
                      style: TextStyle(fontSize: 11, color: MemoTheme.of(context).textDim),
                    ),
                    onTap: _cloudOp.isNotEmpty ? null : _restoreCloud,
                  ),
                ),
              ),
            ],
          ),
        ],
        SizedBox(height: 24),
      ],
    );
  }
}

class CloudTextField extends StatelessWidget {
  final String label;
  final TextEditingController controller;
  final String hint;
  final bool obscure;

  const CloudTextField({super.key,
    required this.label,
    required this.controller,
    required this.hint,
    this.obscure = false,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        SizedBox(
          width: 160,
          child: Text(
            label,
            style: TextStyle(fontSize: 13, color: MemoTheme.of(context).textMain),
          ),
        ),
        Expanded(
          child: SizedBox(
            height: 36,
            child: TextField(
              controller: controller,
              obscureText: obscure,
              style: TextStyle(fontSize: 13),
              decoration: InputDecoration(
                hintText: hint,
                hintStyle: TextStyle(fontSize: 12),
                contentPadding: EdgeInsets.symmetric(horizontal: 12),
                filled: true,
                fillColor: MemoTheme.of(context).bgApp,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(color: MemoTheme.of(context).borderSoft),
                ),
                enabledBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(color: MemoTheme.of(context).borderSoft),
                ),
                focusedBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
                  borderSide: BorderSide(color: MemoTheme.accent),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}
