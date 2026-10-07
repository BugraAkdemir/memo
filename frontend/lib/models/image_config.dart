/// Automatic image routing settings (GET/PUT /api/image/config).
///
/// [autoRoute]: a chat that asks for a picture is sent to an image model for that
/// one turn. [defaultProvider]/[defaultModel]: the image model API-key providers
/// use — a Subscriptions account always uses its own account's image model.
class ImageConfig {
  final bool autoRoute;
  final String defaultProvider;
  final String defaultModel;

  const ImageConfig({
    this.autoRoute = true,
    this.defaultProvider = '',
    this.defaultModel = '',
  });

  factory ImageConfig.fromJson(Map<String, dynamic> json) => ImageConfig(
        autoRoute: json['auto_route'] as bool? ?? true,
        defaultProvider: json['default_provider'] as String? ?? '',
        defaultModel: json['default_model'] as String? ?? '',
      );

  bool get hasDefault => defaultProvider.isNotEmpty && defaultModel.isNotEmpty;

  @override
  bool operator ==(Object other) =>
      other is ImageConfig &&
      other.autoRoute == autoRoute &&
      other.defaultProvider == defaultProvider &&
      other.defaultModel == defaultModel;

  @override
  int get hashCode => Object.hash(autoRoute, defaultProvider, defaultModel);
}
