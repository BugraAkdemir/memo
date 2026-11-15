import 'package:path/path.dart' as p;

import 'gpu_info.dart';
import 'local_model.dart';
import 'provider_config.dart';
import 'provider_models.dart';

/// What attachment types the currently active model/provider can accept in the
/// chat composer.
class ModelInputCapabilities {
  final bool supportsImages;
  final bool supportsFiles;

  const ModelInputCapabilities({
    required this.supportsImages,
    required this.supportsFiles,
  });
}

/// Resolves whether the active chat target supports image and file attachments.
///
/// Unknown/loading states default to `true` so transient startup or offline
/// test states don't unexpectedly hide controls; once a concrete non-vision or
/// image-only model is known, unsupported buttons are hidden.
ModelInputCapabilities resolveModelInputCapabilities({
  required String activeProvider,
  required String cliType,
  required List<ProviderConfig> providers,
  required ServerStatus? localStatus,
  required List<LocalModel> localModels,
  required bool orchestraEnabled,
}) {
  if (orchestraEnabled) {
    return const ModelInputCapabilities(
      supportsImages: false,
      supportsFiles: false,
    );
  }

  if (cliType.isNotEmpty) {
    return ModelInputCapabilities(
      supportsImages: cliType == 'claude-code-cli',
      supportsFiles: true,
    );
  }

  if (activeProvider == kAutoRouteProviderName) {
    return const ModelInputCapabilities(
      supportsImages: true,
      supportsFiles: true,
    );
  }

  if (activeProvider.isEmpty || activeProvider == 'local') {
    if (localStatus == null) {
      return const ModelInputCapabilities(
        supportsImages: true,
        supportsFiles: true,
      );
    }
    if (!localStatus.running) {
      return const ModelInputCapabilities(
        supportsImages: false,
        supportsFiles: false,
      );
    }
    final runningPath = localStatus.modelPath;
    final runningBase = p.basename(runningPath);
    LocalModel? match;
    for (final m in localModels) {
      if (m.path == runningPath ||
          (runningBase.isNotEmpty && p.basename(m.path) == runningBase)) {
        match = m;
        break;
      }
    }
    final supportsImages = match != null
        ? (match.isVision || match.supportsVision)
        : modelSupportsImages('custom', runningBase);
    return ModelInputCapabilities(
      supportsImages: supportsImages,
      supportsFiles: true,
    );
  }

  ProviderConfig? match;
  for (final cfg in providers) {
    if (cfg.name == activeProvider || cfg.type == activeProvider) {
      match = cfg;
      break;
    }
  }
  final providerType = match?.type ?? activeProvider;
  final modelId = match?.model ?? '';
  if (modelId.isEmpty) {
    return ModelInputCapabilities(
      supportsImages: _providerDefaultsToVision(providerType),
      supportsFiles: true,
    );
  }

  if (isImageOnlyModel(modelId)) {
    return const ModelInputCapabilities(
      supportsImages: false,
      supportsFiles: false,
    );
  }

  return ModelInputCapabilities(
    supportsImages: modelSupportsImages(providerType, modelId),
    supportsFiles: true,
  );
}

bool _providerDefaultsToVision(String providerType) {
  switch (providerType) {
    case 'gemini':
    case 'claude':
    case 'openai':
    case 'grok':
      return true;
    default:
      return false;
  }
}

/// Whether [modelId] is a dedicated image-generation model (no text/file chat).
bool isImageOnlyModel(String modelId) {
  final id = modelId.toLowerCase();
  if (id.isEmpty) return false;
  return id.contains('imagen') ||
      id.contains('dall-e') ||
      id.contains('gpt-image') ||
      id.contains('flux') ||
      id.contains('sdxl') ||
      id.contains('stable-diffusion');
}

/// Heuristic check for whether [modelId] on [providerType] accepts image input.
bool modelSupportsImages(String providerType, String modelId) {
  final id = p.basename(modelId).toLowerCase();
  if (id.isEmpty) return _providerDefaultsToVision(providerType);

  const textOnlyPatterns = [
    'deepseek-coder',
    'deepseek-r1',
    'deepseek-v2',
    'deepseek-v3',
    'moonlight',
    'codellama',
    'qwen2.5-coder',
    'qwen-coder',
    'starcoder',
    'codestral',
    'devstral',
    'phi-3',
    'phi-4-mini',
    'llama-3.1',
    'llama-3.3',
    'mistral-7b',
    'mixtral',
    'gemma-2',
    'o1-mini',
    'gpt-3.5',
    'lfm2',
  ];
  for (final pat in textOnlyPatterns) {
    if (id.contains(pat)) return false;
  }

  const visionPatterns = [
    'vision',
    '-vl',
    'vl-',
    'llava',
    'pixtral',
    'minicpm-v',
    'internvl',
    'gpt-4o',
    'gpt-4.1',
    'gpt-4.5',
    'gpt-4-turbo',
    'gpt-5',
    'chatgpt-4o',
    'o1',
    'o3',
    'o4',
    'claude-3',
    'claude-4',
    'claude-sonnet-4',
    'claude-opus-4',
    'claude-haiku-4',
    'claude-sonnet-5',
    'claude-opus-5',
    'claude-haiku-5',
    'claude-fable',
    'gemini',
    'grok-2-vision',
    'grok-3',
    'grok-4',
    'gemma-3',
    'gemma-4',
    'qwen3.5',
    'qwen3.6',
    'qwen36',
    'llama-3.2-11b',
    'llama-3.2-90b',
    'llama-4',
  ];
  for (final pat in visionPatterns) {
    if (id.contains(pat)) return true;
  }

  return _providerDefaultsToVision(providerType);
}
