import 'package:flutter/material.dart';

import '../providers/provider_provider.dart' show providerLogoWidget;

/// A model id shaped for people: "gemini-3.7-flash-high" reads as the title
/// "Gemini 3.7 Flash" with the tag "High". The raw id stays the value that is
/// sent and stored; this is display only.
class ModelDisplay {
  final String title;
  final String tag;
  const ModelDisplay(this.title, [this.tag = '']);

  /// Title and tag on one line, for places with room for only a label.
  String get label => tag.isEmpty ? title : '$title · $tag';
}

/// Trailing words that describe HOW a model is run rather than which model it
/// is — shown as a small tag instead of making the name longer.
const _tagWords = {'high', 'medium', 'low', 'minimal', 'thinking', 'agent', 'preview', 'latest'};

const _upperWords = {'gpt': 'GPT', 'oss': 'OSS', 'ai': 'AI', 'llm': 'LLM'};

final _digits = RegExp(r'^\d{1,2}$');
final _sizeToken = RegExp(r'^\d+(\.\d+)?[bkm]$', caseSensitive: false);

/// Splits [id] into a readable title and an optional tag. An id it cannot make
/// sense of (a path, an unusual scheme) comes back unchanged as the title.
ModelDisplay prettyModelName(String id) {
  final raw = id.trim();
  if (raw.isEmpty) return const ModelDisplay('');
  // "vendor/model" style ids (OpenRouter, Kilo …): prettify the model part only.
  final slash = raw.lastIndexOf('/');
  final base = slash >= 0 ? raw.substring(slash + 1) : raw;
  if (base.isEmpty || base.contains(RegExp(r'[^A-Za-z0-9._:-]'))) return ModelDisplay(raw);

  final tokens = base.split('-').where((t) => t.isNotEmpty).toList();
  if (tokens.isEmpty) return ModelDisplay(raw);

  // Pull trailing descriptor words off the end (gemini-3.1-pro-low → tag "Low").
  final tags = <String>[];
  while (tokens.length > 1 && _tagWords.contains(tokens.last.toLowerCase())) {
    tags.insert(0, _cap(tokens.removeLast()));
  }

  // "4" "6" → "4.6": consecutive short numbers are one version split by dashes.
  final words = <String>[];
  for (final t in tokens) {
    if (_digits.hasMatch(t) && words.isNotEmpty && _digits.hasMatch(words.last.replaceAll('.', ''))) {
      words[words.length - 1] = '${words.last}.$t';
    } else {
      words.add(t);
    }
  }

  final title = words.map((w) {
    final lower = w.toLowerCase();
    if (_upperWords.containsKey(lower)) return _upperWords[lower]!;
    if (_sizeToken.hasMatch(w)) return w.toUpperCase();
    return _cap(w);
  }).join(' ').replaceAll('GPT OSS', 'GPT-OSS');
  return ModelDisplay(title, tags.join(' '));
}

String _cap(String w) => w.isEmpty ? w : w[0].toUpperCase() + w.substring(1);

/// The family a model id belongs to, which decides its logo. Null when unknown.
String? modelFamily(String id) {
  final s = id.toLowerCase();
  if (s.contains('claude')) return 'claude';
  if (s.contains('gemini') || s.contains('gemma')) return 'gemini';
  if (s.contains('gpt') || s.contains('codex') || RegExp(r'(^|[^a-z])o[134](-|$)').hasMatch(s)) return 'openai';
  return null;
}

/// Logo of the model's family (Google for Gemini, Anthropic for Claude, OpenAI
/// for GPT), or a neutral mark for anything else.
Widget modelFamilyLogo(String id, {double size = 16}) {
  final family = modelFamily(id);
  if (family == null) return Icon(Icons.auto_awesome_outlined, size: size);
  return providerLogoWidget(family, size: size);
}

/// Logo for a sidecar vendor key (antigravity is Google's).
Widget vendorLogo(String ownedBy, {double size = 16}) {
  final type = switch (ownedBy) {
    'antigravity' => 'gemini',
    'claude' => 'claude',
    'codex' => 'openai',
    _ => '',
  };
  return type.isEmpty ? SizedBox(width: size, height: size) : providerLogoWidget(type, size: size);
}
