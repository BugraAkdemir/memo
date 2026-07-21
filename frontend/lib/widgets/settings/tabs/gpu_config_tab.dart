import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme.dart';
import '../../../core/l10n.dart';
import '../../../providers/chat_provider.dart';
import '../../../providers/models_provider.dart';
import '../../../providers/settings_provider.dart';

class GpuConfigTab extends ConsumerStatefulWidget {
  const GpuConfigTab({super.key});

  @override
  ConsumerState<GpuConfigTab> createState() => GpuConfigTabState();
}

class GpuConfigTabState extends ConsumerState<GpuConfigTab> {
  bool _installing = false;
  String _error = '';

  Future<void> _startInstall() async {
    setState(() {
      _installing = true;
      _error = '';
    });

    try {
      await ref.read(apiClientProvider).installLlamaServer();
      ref.invalidate(llamaInstalledProvider);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
      });
    } finally {
      if (mounted) {
        setState(() {
          _installing = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final installedAsync = ref.watch(llamaInstalledProvider);
    final gpuAsync = ref.watch(gpuInfoProvider);
    final hasGpu = gpuAsync.whenOrNull(data: (g) => g.hasGpu) ?? false;
    final gpuName = gpuAsync.whenOrNull(data: (g) => g.name) ?? '';

    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Text(
          L10n.t('gpu_section_title'),
          style: Theme.of(context).textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: MemoTheme.of(context).textMain,
          ),
        ),
        SizedBox(height: 12),
        Text(
          L10n.t('gpu_section_desc'),
          style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
        ),
        SizedBox(height: 32),

        Container(
          padding: EdgeInsets.all(20),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                L10n.t('hardware_status'),
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              SizedBox(height: 12),
              Row(
                children: [
                  Icon(
                    hasGpu ? Icons.memory : Icons.developer_board,
                    size: 20,
                    color: hasGpu
                        ? MemoTheme.accent
                        : MemoTheme.of(context).textDim,
                  ),
                  SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      hasGpu
                          ? L10n.t('gpu_detected_name', {'name': gpuName})
                          : L10n.t('cpu_only'),
                      style: TextStyle(
                        fontSize: 14,
                        color: MemoTheme.of(context).textMain,
                      ),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
        SizedBox(height: 24),

        installedAsync.when(
          loading: () => Center(child: CircularProgressIndicator()),
          error: (e, _) => Text('${L10n.t('error')}: $e'),
          data: (installed) {
            final llamaSettings = ref.watch(llamaSettingsProvider);
            return Column(
              children: [
                // Engine Mode Selection
                Container(
                  padding: EdgeInsets.all(20),
                  decoration: BoxDecoration(
                    color: MemoTheme.of(context).bgPanel,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: MemoTheme.of(context).borderSoft),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        L10n.t('engine_mode'),
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: MemoTheme.of(context).textMain,
                        ),
                      ),
                      SizedBox(height: 12),
                      llamaSettings.when(
                        loading: () => CircularProgressIndicator(),
                        error: (e, _) => Text('${L10n.t('error')}: $e'),
                        data: (settings) => DropdownButton<String>(
                          value: settings.engineMode,
                          isExpanded: true,
                          dropdownColor: MemoTheme.of(context).bgPanel.withValues(alpha: 1.0),
                          elevation: 8,
                          items: [
                            DropdownMenuItem(
                              value: 'auto',
                              child: Text(L10n.t('engine_auto')),
                            ),
                            DropdownMenuItem(
                              value: 'cpu',
                              child: Text(L10n.t('engine_cpu')),
                            ),
                            DropdownMenuItem(
                              value: 'nvidia',
                              child: Text(L10n.t('engine_nvidia')),
                            ),
                            DropdownMenuItem(
                              value: 'amd',
                              child: Text(L10n.t('engine_amd')),
                            ),
                            DropdownMenuItem(
                              value: 'metal',
                              child: Text(L10n.t('engine_metal')),
                            ),
                          ],
                          onChanged: (mode) {
                            if (mode != null) {
                              final cur = llamaSettings.valueOrNull;
                              if (cur != null) {
                                ref
                                    .read(llamaSettingsProvider.notifier)
                                    .save(
                                      LlamaSettings(
                                        engineMode: mode,
                                        binaryPath: cur.binaryPath,
                                        port: cur.port,
                                        ctxSize: cur.ctxSize,
                                        temperature: cur.temperature,
                                        topP: cur.topP,
                                        maxTokens: cur.maxTokens,
                                      ),
                                    );
                              }
                            }
                          },
                        ),
                      ),
                    ],
                  ),
                ),
                SizedBox(height: 24),
                ModelParametersCard(),
                SizedBox(height: 24),
                // Installation Status Card
                Container(
                  padding: EdgeInsets.all(20),
                  decoration: BoxDecoration(
                    color: MemoTheme.of(context).bgPanel,
                    borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
                    border: Border.all(color: MemoTheme.of(context).borderSoft),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Icon(
                            installed
                                ? Icons.check_circle
                                : Icons.warning_amber_rounded,
                            color: installed ? MemoTheme.green : MemoTheme.warningOrange,
                            size: 24,
                          ),
                          SizedBox(width: 12),
                          Text(
                            installed
                                ? L10n.t('llama_installed')
                                : L10n.t('llama_not_installed_status'),
                            style: TextStyle(
                              fontSize: 16,
                              fontWeight: FontWeight.bold,
                              color: MemoTheme.of(context).textMain,
                            ),
                          ),
                        ],
                      ),
                      SizedBox(height: 12),
                      Text(
                        installed
                            ? L10n.t('llama_installed_desc')
                            : L10n.t('llama_not_installed_desc'),
                        style: TextStyle(
                          color: MemoTheme.of(context).textDim,
                          fontSize: 13,
                        ),
                      ),
                      SizedBox(height: 20),
                      if (_error.isNotEmpty)
                        Padding(
                          padding: EdgeInsets.only(bottom: 16),
                          child: Text(
                            _error,
                            style: TextStyle(
                              color: MemoTheme.red,
                              fontSize: 13,
                            ),
                          ),
                        ),
                      SizedBox(
                        width: double.infinity,
                        height: 44,
                        child: ElevatedButton(
                          onPressed: _installing ? null : _startInstall,
                          style: ElevatedButton.styleFrom(
                            backgroundColor: MemoTheme.accent,
                            foregroundColor: MemoTheme.of(context).textInverse,
                          ),
                          child: _installing
                              ? SizedBox(
                                  width: 20,
                                  height: 20,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                    color: MemoTheme.of(context).textInverse,
                                  ),
                                )
                              : Text(
                                  installed
                                      ? L10n.t('llama_reinstall')
                                      : (hasGpu
                                            ? L10n.t('llama_install_gpu')
                                            : L10n.t('llama_install')),
                                  style: TextStyle(fontWeight: FontWeight.w600),
                                ),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            );
          },
        ),
      ],
    );
  }
}

class ModelParametersCard extends ConsumerStatefulWidget {
  const ModelParametersCard({super.key});

  @override
  ConsumerState<ModelParametersCard> createState() =>
      ModelParametersCardState();
}

class ModelParametersCardState extends ConsumerState<ModelParametersCard> {
  late double _temperature;
  late double _topP;
  late int _maxTokens;
  late int _ctxSize;
  bool _loaded = false;
  bool _dirty = false;

  @override
  Widget build(BuildContext context) {
    final llamaSettings = ref.watch(llamaSettingsProvider);
    return llamaSettings.when(
      loading: () => SizedBox.shrink(),
      error: (e, _) => Text('${L10n.t('error')}: $e'),
      data: (settings) {
        if (!_loaded) {
          _temperature = settings.temperature;
          _topP = settings.topP;
          _maxTokens = settings.maxTokens;
          _ctxSize = settings.ctxSize;
          _loaded = true;
        } else if (!_dirty && (_temperature != settings.temperature ||
            _topP != settings.topP ||
            _maxTokens != settings.maxTokens ||
            _ctxSize != settings.ctxSize)) {
          _temperature = settings.temperature;
          _topP = settings.topP;
          _maxTokens = settings.maxTokens;
          _ctxSize = settings.ctxSize;
        }
        return Container(
          padding: EdgeInsets.all(20),
          decoration: BoxDecoration(
            color: MemoTheme.of(context).bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(color: MemoTheme.of(context).borderSoft),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                L10n.t('model_params'),
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: MemoTheme.of(context).textMain,
                ),
              ),
              SizedBox(height: 16),
              ParamSlider(
                label: 'Temperature',
                value: _temperature,
                min: 0.0,
                max: 2.0,
                divisions: 40,
                displayValue: _temperature.toStringAsFixed(2),
                onChanged: (v) => setState(() { _temperature = v; _dirty = true; }),
              ),
              SizedBox(height: 12),
              ParamSlider(
                label: 'Top P',
                value: _topP,
                min: 0.0,
                max: 1.0,
                divisions: 20,
                displayValue: _topP.toStringAsFixed(2),
                onChanged: (v) => setState(() { _topP = v; _dirty = true; }),
              ),
              SizedBox(height: 12),
              ParamIntInput(
                label: 'Max Tokens',
                value: _maxTokens,
                min: 0,
                max: 65536,
                step: 256,
                displaySuffix: L10n.t('param_unlimited'),
                onChanged: (v) => setState(() { _maxTokens = v; _dirty = true; }),
              ),
              SizedBox(height: 12),
              ParamIntInput(
                label: 'Context Size',
                value: _ctxSize,
                min: 512,
                max: 65536,
                step: 512,
                onChanged: (v) => setState(() { _ctxSize = v; _dirty = true; }),
              ),
              SizedBox(height: 16),
              SizedBox(
                width: double.infinity,
                height: 44,
                child: ElevatedButton(
                  onPressed: () async {
                    await ref
                        .read(llamaSettingsProvider.notifier)
                        .save(
                          LlamaSettings(
                            engineMode: settings.engineMode,
                            binaryPath: settings.binaryPath,
                            port: settings.port,
                            ctxSize: _ctxSize,
                            temperature: _temperature,
                            topP: _topP,
                            maxTokens: _maxTokens,
                          ),
                        );
                    setState(() => _dirty = false);
                    if (context.mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(
                          content: Text(L10n.t('params_saved')),
                          duration: Duration(seconds: 2),
                          backgroundColor: MemoTheme.green,
                        ),
                      );
                    }
                  },
                  style: ElevatedButton.styleFrom(
                    backgroundColor: MemoTheme.accent,
                    foregroundColor: MemoTheme.of(context).textInverse,
                  ),
                  child: Text(
                    L10n.t('apply'),
                    style: TextStyle(fontWeight: FontWeight.w600),
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class ParamSlider extends StatelessWidget {
  final String label;
  final double value;
  final double min;
  final double max;
  final int divisions;
  final String displayValue;
  final ValueChanged<double> onChanged;

  const ParamSlider({super.key, 
    required this.label,
    required this.value,
    required this.min,
    required this.max,
    required this.divisions,
    required this.displayValue,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              label,
              style: TextStyle(
                fontSize: 13,
                color: MemoTheme.of(context).textMain,
              ),
            ),
            Text(
              displayValue,
              style: TextStyle(
                fontSize: 13,
                color: MemoTheme.accent,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ),
        Slider(
          value: value,
          min: min,
          max: max,
          divisions: divisions,
          activeColor: MemoTheme.accent,
          inactiveColor: MemoTheme.of(context).borderSoft,
          onChanged: onChanged,
        ),
      ],
    );
  }
}

class ParamIntInput extends StatefulWidget {
  final String label;
  final int value;
  final int min;
  final int max;
  final int step;
  final String? displaySuffix;
  final ValueChanged<int> onChanged;

  const ParamIntInput({super.key, 
    required this.label,
    required this.value,
    required this.min,
    required this.max,
    required this.step,
    this.displaySuffix,
    required this.onChanged,
  });

  @override
  State<ParamIntInput> createState() => ParamIntInputState();
}

class ParamIntInputState extends State<ParamIntInput> {
  late TextEditingController _controller;
  final _focusNode = FocusNode();
  bool _isEditing = false;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(
      text: widget.value == 0 && widget.min == 0
          ? '0'
          : widget.value.toString(),
    );
    _focusNode.addListener(() {
      _isEditing = _focusNode.hasFocus;
    });
  }

  @override
  void didUpdateWidget(ParamIntInput oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.value != oldWidget.value && !_isEditing) {
      _controller.text = widget.value == 0 && widget.min == 0
          ? '0'
          : widget.value.toString();
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        SizedBox(
          width: 140,
          child: Text(
            widget.label,
            style: TextStyle(
              fontSize: 13,
              color: MemoTheme.of(context).textMain,
            ),
          ),
        ),
        SizedBox(
          width: 120,
          child: TextField(
            controller: _controller,
            focusNode: _focusNode,
            keyboardType: TextInputType.number,
            decoration: InputDecoration(
              isDense: true,
              contentPadding: EdgeInsets.symmetric(horizontal: 8, vertical: 6),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.all(Radius.circular(4)),
              ),
            ),
            onChanged: (v) {
              final parsed = int.tryParse(v);
              if (parsed != null) {
                widget.onChanged(parsed.clamp(widget.min, widget.max));
              }
            },
          ),
        ),
        if (widget.displaySuffix != null) ...[
          SizedBox(width: 8),
          Text(
            widget.displaySuffix!,
            style: TextStyle(
              fontSize: 11,
              color: MemoTheme.of(context).textDim,
            ),
          ),
        ],
      ],
    );
  }
}
