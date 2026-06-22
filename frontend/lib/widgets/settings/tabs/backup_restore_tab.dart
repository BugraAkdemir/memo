import 'package:flutter/material.dart';
import 'dart:io';
import 'package:url_launcher/url_launcher.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import 'dart:async';
import 'package:file_picker/file_picker.dart';
import '../../../providers/chat_provider.dart';

class BackupRestoreTab extends ConsumerStatefulWidget {
  BackupRestoreTab();

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

  Future<void> _saveCredentials() async {
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
          const SnackBar(content: Text('Kimlik bilgileri kaydedildi')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Kaydetme hatası: $e')),
        );
      }
    } finally {
      if (mounted) setState(() { _cloudOp = ''; });
    }
  }

  Future<void> _connectDrive() async {
    if (_clientIdCtrl.text.trim().isEmpty || _clientSecretCtrl.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Lütfen önce Client ID ve Client Secret girin')),
      );
      return;
    }
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
      _authPollTimer?.cancel();
      _authPollTimer = Timer.periodic(const Duration(seconds: 3), (t) async {
        attempts++;
        if (attempts > 40) {
          t.cancel();
          if (mounted) setState(() { _cloudOp = ''; });
          return;
        }
        try {
          final done = await ref.read(apiClientProvider).checkSyncAuth();
          if (done) {
            t.cancel();
            await _loadCloudStatus();
            if (mounted) setState(() { _cloudOp = ''; });
          }
        } catch (_) {}
      });
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Bağlantı hatası: $e')),
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
          const SnackBar(content: Text('Drive yedeklemesi başlatıldı (arka planda çalışıyor)')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Yedekleme hatası: $e')),
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
        title: const Text('Buluttan Geri Yükle'),
        content: const Text(
          "Drive'daki son yedek geri yüklenecek.\n"
          'Mevcut hafıza verilerinin üzerine yazılacak.\n'
          'Devam etmek istiyor musunuz?',
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('İptal')),
          TextButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Geri Yükle')),
        ],
      ),
    );
    if (confirm != true) return;
    setState(() { _cloudOp = 'restore'; });
    try {
      await ref.read(apiClientProvider).pullSync();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Geri yükleme başlatıldı. Tamamlandığında uygulamayı yeniden başlatın.')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Geri yükleme hatası: $e')),
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
        title: const Text('Drive Bağlantısını Kes'),
        content: const Text('Google Drive bağlantısı kesilecek. Yerel yedekler korunur.'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('İptal')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: TextButton.styleFrom(foregroundColor: MemoTheme.red),
            child: const Text('Bağlantıyı Kes'),
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
          const SnackBar(content: Text('Drive bağlantısı kesildi')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Hata: $e')),
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
        dialogTitle: 'Memo Yedekle',
        fileName: 'memo_backup.memo',
        type: FileType.any,
      );
      if (path != null) {
        await File(path).writeAsBytes(data);
        if (mounted) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text('Yedek kaydedildi: $path')));
        }
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Dışa aktarma hatası: $e')));
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
        dialogTitle: 'Memo Yedek İçe Aktar',
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
            content: Text(
              'Yedek başarıyla içe aktarıldı. Uygulamayı yeniden başlatın.',
            ),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('İçe aktarma hatası: $e')));
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
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Tüm veriler silindi. Uygulamayı yeniden başlatın.'),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Silme hatası: $e')));
      }
    } finally {
      if (mounted)
        setState(() {
          _wiping = false;
          _wipeConfirm1 = false;
          _wipeConfirm2 = false;
        });
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Text(
          'Yedekleme',
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Text(
          'Tüm sohbet geçmişi, yapılandırma ve WhatsApp mesajlarınızı .memo dosyasına aktarın veya geri yükleyin.',
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 24),

        // Include models toggle
        Card(
          child: SwitchListTile(
            title: Text('Modelleri dahil et'),
            subtitle: Text(
              'GGUF modelleri (büyük boyut)',
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
            title: Text('Dışa Aktar'),
            subtitle: Text(
              'Tüm verileri .memo dosyasına kaydeder',
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
            title: Text('İçe Aktar'),
            subtitle: Text(
              '.memo dosyasından verileri geri yükler',
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
          'Tüm Verileri Sil',
          style: TextStyle(
            fontWeight: FontWeight.bold,
            fontSize: 16,
            color: MemoTheme.warmBrown,
          ),
        ),
        SizedBox(height: 8),
        Text(
          'Sohbet geçmişi, WhatsApp mesajları, hafıza ve yapılandırma kalıcı olarak silinir.',
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 12),
        if (!_wipeConfirm1)
          Card(
            child: ListTile(
              leading: Icon(Icons.delete_forever, color: MemoTheme.warmBrown),
              title: Text(
                'Tüm Verileri Sil',
                style: TextStyle(color: MemoTheme.warmBrown),
              ),
              subtitle: Text(
                'Bu işlem geri alınamaz',
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
                'Emin misiniz?',
                style: TextStyle(color: Colors.redAccent),
              ),
              subtitle: Text(
                'Tüm verileriniz silinecek. Onaylamak için tekrar tıklayın.',
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
                'Sil',
                style: TextStyle(
                  color: MemoTheme.red,
                  fontWeight: FontWeight.bold,
                ),
              ),
              subtitle: Text(
                'Bu işlem geri alınamaz. Tüm veriler silinecek.',
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
              'Bulut Yedekleme (Google Drive)',
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
          'Hafıza verilerini AES-256 şifreli olarak Google Drive\'a yedekle ve '
          'farklı cihazlara geri yükle. Sadece bu uygulamanın oluşturduğu '
          'dosyalara erişim sağlanır.',
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
                      _cloudConnected ? 'Drive Bağlı' : 'Bağlı Değil',
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
                          'Kimlik bilgilerini girin ve bağlan',
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
                              'Tarayıcıda yetkilendirme bekleniyor...',
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
                  child: Text('Kes', style: TextStyle(fontSize: 12)),
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
                'Google OAuth Kimlik Bilgileri',
                style: TextStyle(
                  fontWeight: FontWeight.w600,
                  fontSize: 13,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              if (_cloudConnected)
                TextButton(
                  onPressed: () => setState(() => _showCredSetup = false),
                  child: Text('Kapat', style: TextStyle(fontSize: 12)),
                ),
            ],
          ),
          SizedBox(height: 4),
          Text(
            'Google Cloud Console\'dan bir OAuth 2.0 Desktop App kimlik bilgisi oluşturun.',
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
            label: 'Şifreleme Parolası',
            controller: _passphraseCtrl,
            hint: 'Opsiyonel — boş bırakırsanız cihaz kimliği kullanılır',
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
                      : Text('Kaydet'),
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
                  label: Text('Google Drive\'a Bağlan'),
                  style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
                ),
              ] else ...[
                FilledButton(
                  onPressed: _cloudOp.isNotEmpty ? null : _saveCredentials,
                  child: _cloudOp == 'saving'
                      ? SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        )
                      : Text('Kimlik Bilgilerini Güncelle'),
                  style: FilledButton.styleFrom(backgroundColor: MemoTheme.accent),
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
                'Yedekleme İşlemleri',
                style: TextStyle(
                  fontWeight: FontWeight.w600,
                  fontSize: 13,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              TextButton(
                onPressed: () => setState(() => _showCredSetup = !_showCredSetup),
                child: Text(
                  _showCredSetup ? 'Ayarları Kapat' : 'Kimlik Bilgilerini Düzenle',
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
                    title: Text('Şimdi Yedekle', style: TextStyle(fontSize: 14)),
                    subtitle: Text(
                      'Hafızayı Drive\'a gönder',
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
                    title: Text('Geri Yükle', style: TextStyle(fontSize: 14)),
                    subtitle: Text(
                      'Son yedeği indir ve uygula',
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

  const CloudTextField({
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
