import 'package:flutter/material.dart';

class ToolNames {
  static final Map<String, _ToolInfo> _tools = {
    'read_file': _ToolInfo('Dosya Oku', 'Dosya içeriğini okur', Icons.description_outlined),
    'write_file': _ToolInfo('Dosya Yaz', 'Dosyaya yazma veya dosya oluşturma', Icons.edit_note),
    'delete_file': _ToolInfo('Dosya Sil', 'Dosya veya dizin siler', Icons.delete_outline),
    'list_directory': _ToolInfo('Dizin Listele', 'Klasör içeriğini listeler', Icons.folder_outlined),
    'run_command': _ToolInfo('Komut Çalıştır', 'Sistemde komut çalıştırır', Icons.terminal),
    'search_files': _ToolInfo('Dosya Ara', 'Dosya sisteminde arama yapar', Icons.search),
    'get_file_info': _ToolInfo('Dosya Bilgisi', 'Dosya metaverisini okur', Icons.info_outline),
    'read_env': _ToolInfo('Ortam Değişkeni Oku', 'Sistem ortam değişkenlerini okur', Icons.settings),
    'edit_file': _ToolInfo('Dosya Düzenle', 'Dosyada metin değişikliği yapar', Icons.edit),
    'insert_line': _ToolInfo('Satır Ekle', 'Dosyaya satır ekler', Icons.space_bar),
    'delete_lines': _ToolInfo('Satır Sil', 'Dosyadan satır aralığı siler', Icons.remove_circle_outline),
    'whatsapp_send': _ToolInfo('WhatsApp Mesaj Gönder', 'WhatsApp üzerinden mesaj gönderir', Icons.send),
    'whatsapp_search': _ToolInfo('WhatsApp Ara', 'WhatsApp mesajlarında arama yapar', Icons.chat),
    'whatsapp_latest': _ToolInfo('WhatsApp Sohbetler', 'En son WhatsApp sohbetlerini listeler', Icons.chat_bubble_outline),
    'whatsapp_messages': _ToolInfo('WhatsApp Geçmişi', 'WhatsApp sohbet geçmişini okur', Icons.history),
  };

  static String displayName(String? toolName) {
    if (toolName == null) return 'Bilinmeyen Araç';
    return _tools[toolName]?.name ?? toolName;
  }

  static String description(String? toolName) {
    if (toolName == null) return '';
    return _tools[toolName]?.description ?? '';
  }

  static IconData icon(String? toolName) {
    if (toolName == null) return Icons.help_outline;
    return _tools[toolName]?.icon ?? Icons.help_outline;
  }

  static String dangerLabel(String? dangerLevel) {
    switch (dangerLevel) {
      case 'dangerous':
        return 'Tehlikeli';
      case 'medium':
        return 'Dikkatli';
      case 'safe':
        return 'Güvenli';
      default:
        return 'Bilinmeyen';
    }
  }
}

class _ToolInfo {
  final String name;
  final String description;
  final IconData icon;

  const _ToolInfo(this.name, this.description, this.icon);
}
