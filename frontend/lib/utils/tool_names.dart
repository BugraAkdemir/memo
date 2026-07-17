import 'package:flutter/material.dart';

import '../core/l10n.dart';

class ToolNames {
  /// Tool id → (name key, description key, icon).
  static final Map<String, _ToolMeta> _tools = {
    'read_file': _ToolMeta('tool_read_file', 'tool_read_file_desc', Icons.description_outlined),
    'write_file': _ToolMeta('tool_write_file', 'tool_write_file_desc', Icons.edit_note),
    'delete_file': _ToolMeta('tool_delete_file', 'tool_delete_file_desc', Icons.delete_outline),
    'list_directory': _ToolMeta('tool_list_directory', 'tool_list_directory_desc', Icons.folder_outlined),
    'run_command': _ToolMeta('tool_run_command', 'tool_run_command_desc', Icons.terminal),
    'search_files': _ToolMeta('tool_search_files', 'tool_search_files_desc', Icons.search),
    'get_file_info': _ToolMeta('tool_get_file_info', 'tool_get_file_info_desc', Icons.info_outline),
    'read_env': _ToolMeta('tool_read_env', 'tool_read_env_desc', Icons.settings),
    'edit_file': _ToolMeta('tool_edit_file', 'tool_edit_file_desc', Icons.edit),
    'insert_line': _ToolMeta('tool_insert_line', 'tool_insert_line_desc', Icons.space_bar),
    'delete_lines': _ToolMeta('tool_delete_lines', 'tool_delete_lines_desc', Icons.remove_circle_outline),
    'whatsapp_send': _ToolMeta('tool_whatsapp_send', 'tool_whatsapp_send_desc', Icons.send),
    'whatsapp_search': _ToolMeta('tool_whatsapp_search', 'tool_whatsapp_search_desc', Icons.chat),
    'whatsapp_latest': _ToolMeta('tool_whatsapp_latest', 'tool_whatsapp_latest_desc', Icons.chat_bubble_outline),
    'whatsapp_messages': _ToolMeta('tool_whatsapp_messages', 'tool_whatsapp_messages_desc', Icons.history),
  };

  static String displayName(String? toolName) {
    if (toolName == null) return L10n.t('unknown_tool');
    final meta = _tools[toolName];
    if (meta == null) return toolName;
    return L10n.t(meta.nameKey);
  }

  static String description(String? toolName) {
    if (toolName == null) return '';
    final meta = _tools[toolName];
    if (meta == null) return '';
    return L10n.t(meta.descKey);
  }

  static IconData icon(String? toolName) {
    if (toolName == null) return Icons.help_outline;
    return _tools[toolName]?.icon ?? Icons.help_outline;
  }

  static String dangerLabel(String? dangerLevel) {
    switch (dangerLevel) {
      case 'dangerous':
        return L10n.t('danger_dangerous');
      case 'medium':
        return L10n.t('danger_medium');
      case 'safe':
        return L10n.t('danger_safe');
      default:
        return L10n.t('danger_unknown');
    }
  }
}

class _ToolMeta {
  final String nameKey;
  final String descKey;
  final IconData icon;

  const _ToolMeta(this.nameKey, this.descKey, this.icon);
}
