import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path/path.dart' as p;

import '../core/l10n.dart';
import '../core/theme.dart';
import '../models/local_model.dart';
import '../models/swarm.dart';
import '../providers/models_provider.dart';
import '../providers/swarm_provider.dart';

/// Memo Swarm — Host / Join screen (Beta, non-macOS).
/// Visual pattern mirrors [LaunchpadView]'s feature cards.
class SwarmScreen extends ConsumerStatefulWidget {
  const SwarmScreen({super.key});

  @override
  ConsumerState<SwarmScreen> createState() => _SwarmScreenState();
}

enum _SwarmMode { pick, host, join }

class _SwarmScreenState extends ConsumerState<SwarmScreen> {
  _SwarmMode _mode = _SwarmMode.pick;

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final statusAsync = ref.watch(swarmStatusProvider);

    // If backend already has an active role, jump straight into that view.
    statusAsync.whenData((st) {
      if (_mode == _SwarmMode.pick) {
        if (st.isHost) {
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (mounted && _mode == _SwarmMode.pick) {
              setState(() => _mode = _SwarmMode.host);
            }
          });
        } else if (st.isWorker) {
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (mounted && _mode == _SwarmMode.pick) {
              setState(() => _mode = _SwarmMode.join);
            }
          });
        }
      }
    });

    return Scaffold(
      backgroundColor: Colors.transparent,
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(24, 20, 24, 8),
            child: Row(
              children: [
                if (_mode != _SwarmMode.pick)
                  IconButton(
                    tooltip: L10n.t('swarm_back'),
                    onPressed: () => setState(() => _mode = _SwarmMode.pick),
                    icon: Icon(Icons.arrow_back, color: c.textMain),
                  ),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        L10n.t('swarm_title'),
                        style: Theme.of(context).textTheme.titleLarge?.copyWith(
                              fontWeight: FontWeight.w700,
                              color: c.textMain,
                            ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        L10n.t('swarm_subtitle'),
                        style: TextStyle(fontSize: 13, color: c.textDim),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
          Expanded(
            child: switch (_mode) {
              _SwarmMode.pick => _buildPicker(c),
              _SwarmMode.host => _HostSwarmView(
                  onBack: () => setState(() => _mode = _SwarmMode.pick),
                ),
              _SwarmMode.join => _JoinSwarmView(
                  onBack: () => setState(() => _mode = _SwarmMode.pick),
                ),
            },
          ),
        ],
      ),
    );
  }

  Widget _buildPicker(ThemeColors c) {
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(32, 16, 32, 48),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 720),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              _SwarmExplainerCard(c: c),
              const SizedBox(height: 28),
              Wrap(
                spacing: 16,
                runSpacing: 16,
                alignment: WrapAlignment.center,
                children: [
                  _SwarmFeatureCard(
                    icon: Icons.dns_rounded,
                    title: L10n.t('swarm_host_title'),
                    description: L10n.t('swarm_host_desc'),
                    actionLabel: L10n.t('swarm_choose'),
                    onTap: () => setState(() => _mode = _SwarmMode.host),
                  ),
                  _SwarmFeatureCard(
                    icon: Icons.login_rounded,
                    title: L10n.t('swarm_join_title'),
                    description: L10n.t('swarm_join_desc'),
                    actionLabel: L10n.t('swarm_choose'),
                    onTap: () => setState(() => _mode = _SwarmMode.join),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Plain-language "what is Swarm / how / needs / limits" block.
/// All copy comes from [L10n] — never hardcode user-facing strings here.
class _SwarmExplainerCard extends StatelessWidget {
  final ThemeColors c;
  const _SwarmExplainerCard({required this.c});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: c.bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: c.borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _section(L10n.t('swarm_what_is_title'), L10n.t('swarm_what_is_body')),
          const SizedBox(height: 16),
          Text(
            L10n.t('swarm_how_works_title'),
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w700,
              color: c.textMain,
            ),
          ),
          const SizedBox(height: 8),
          Text(L10n.t('swarm_how_works_1'),
              style: TextStyle(fontSize: 13, height: 1.45, color: c.textSecondary)),
          const SizedBox(height: 4),
          Text(L10n.t('swarm_how_works_2'),
              style: TextStyle(fontSize: 13, height: 1.45, color: c.textSecondary)),
          const SizedBox(height: 4),
          Text(L10n.t('swarm_how_works_3'),
              style: TextStyle(fontSize: 13, height: 1.45, color: c.textSecondary)),
          const SizedBox(height: 16),
          _section(L10n.t('swarm_who_needs_title'), L10n.t('swarm_who_needs_body')),
          const SizedBox(height: 16),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: MemoTheme.warningOrange.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(
                color: MemoTheme.warningOrange.withValues(alpha: 0.35),
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  L10n.t('swarm_limits_title'),
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w700,
                    color: c.textMain,
                  ),
                ),
                const SizedBox(height: 6),
                Text(
                  L10n.t('swarm_limits_body'),
                  style: TextStyle(
                    fontSize: 12,
                    height: 1.45,
                    color: c.textSecondary,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _section(String title, String body) {
    return Column(
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
        const SizedBox(height: 6),
        Text(
          body,
          style: TextStyle(fontSize: 13, height: 1.5, color: c.textSecondary),
        ),
      ],
    );
  }
}

// ─── Feature card (mirrors launchpad_view.dart _FeatureCard) ─────

class _SwarmFeatureCard extends StatefulWidget {
  final IconData icon;
  final String title;
  final String description;
  final String actionLabel;
  final VoidCallback onTap;

  const _SwarmFeatureCard({
    required this.icon,
    required this.title,
    required this.description,
    required this.actionLabel,
    required this.onTap,
  });

  @override
  State<_SwarmFeatureCard> createState() => _SwarmFeatureCardState();
}

class _SwarmFeatureCardState extends State<_SwarmFeatureCard> {
  bool _hovering = false;

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);

    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _hovering = true),
      onExit: (_) => setState(() => _hovering = false),
      child: GestureDetector(
        onTap: widget.onTap,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          width: 240,
          padding: const EdgeInsets.all(20),
          decoration: BoxDecoration(
            color: _hovering ? c.bgElement : c.bgPanel,
            borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
            border: Border.all(
              color: _hovering
                  ? MemoTheme.accent.withValues(alpha: 0.5)
                  : c.borderSoft,
            ),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  color: MemoTheme.accent.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Icon(widget.icon, size: 24, color: MemoTheme.accent),
              ),
              const SizedBox(height: 14),
              Text(
                widget.title,
                style: TextStyle(
                  fontSize: 15,
                  fontWeight: FontWeight.w600,
                  color: c.textMain,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 6),
              Text(
                widget.description,
                style: TextStyle(fontSize: 12, color: c.textDim, height: 1.4),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 16),
              Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                decoration: BoxDecoration(
                  color: _hovering
                      ? MemoTheme.accent.withValues(alpha: 0.15)
                      : c.bgHover,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Text(
                  widget.actionLabel,
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                    color: _hovering ? MemoTheme.accent : c.textSecondary,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// ─── Host view ───────────────────────────────────────────────────

class _HostSwarmView extends ConsumerStatefulWidget {
  final VoidCallback onBack;
  const _HostSwarmView({required this.onBack});

  @override
  ConsumerState<_HostSwarmView> createState() => _HostSwarmViewState();
}

class _HostSwarmViewState extends ConsumerState<_HostSwarmView> {
  LocalModel? _selected;
  bool _busy = false;

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final statusAsync = ref.watch(swarmStatusProvider);
    final modelsAsync = ref.watch(localModelsProvider);
    final status = statusAsync.valueOrNull;
    final hasRoom = status != null && status.isHost && status.roomCode.isNotEmpty;

    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(24, 8, 24, 24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (!hasRoom) ...[
            Text(
              L10n.t('swarm_select_model'),
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: c.textMain,
              ),
            ),
            const SizedBox(height: 8),
            modelsAsync.when(
              loading: () => const LinearProgressIndicator(minHeight: 2),
              error: (e, _) => Text('$e', style: TextStyle(color: c.textDim)),
              data: (models) {
                final chatModels =
                    models.where((m) => !m.isEmbedding).toList();
                if (chatModels.isEmpty) {
                  return Text(
                    L10n.t('swarm_no_models'),
                    style: TextStyle(color: c.textDim),
                  );
                }
                return DropdownButtonFormField<LocalModel>(
                  // ignore: deprecated_member_use
                  value: _selected != null &&
                          chatModels.any((m) => m.path == _selected!.path)
                      ? chatModels
                          .firstWhere((m) => m.path == _selected!.path)
                      : null,
                  decoration: InputDecoration(
                    filled: true,
                    fillColor: c.bgPanel,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(8),
                      borderSide: BorderSide(color: c.borderSoft),
                    ),
                    contentPadding: const EdgeInsets.symmetric(
                        horizontal: 12, vertical: 10),
                  ),
                  dropdownColor: c.bgPanel,
                  style: TextStyle(color: c.textMain, fontSize: 13),
                  items: chatModels
                      .map(
                        (m) => DropdownMenuItem(
                          value: m,
                          child: Text(
                            '${m.repoId} / ${m.filename}',
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      )
                      .toList(),
                  onChanged: _busy
                      ? null
                      : (m) => setState(() => _selected = m),
                );
              },
            ),
            const SizedBox(height: 16),
            Align(
              alignment: Alignment.centerLeft,
              child: FilledButton.icon(
                onPressed: _busy || _selected == null
                    ? null
                    : () async {
                        setState(() => _busy = true);
                        await ref
                            .read(swarmStatusProvider.notifier)
                            .createHost(_selected!.path);
                        if (mounted) setState(() => _busy = false);
                      },
                icon: _busy
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Icon(Icons.add_circle_outline, size: 18),
                label: Text(L10n.t('swarm_create_room')),
              ),
            ),
          ] else ...[
            // Room code + copy
            Text(
              L10n.t('swarm_room_code'),
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: c.textMain,
              ),
            ),
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: c.bgPanel,
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: c.borderSoft),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: SelectableText(
                      status.roomCode,
                      style: TextStyle(
                        fontFamily: 'monospace',
                        fontSize: 12,
                        color: c.textMain,
                      ),
                    ),
                  ),
                  TextButton.icon(
                    onPressed: () {
                      Clipboard.setData(
                          ClipboardData(text: status.roomCode));
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(L10n.t('swarm_code_copied'))),
                      );
                    },
                    icon: const Icon(Icons.copy, size: 16),
                    label: Text(L10n.t('swarm_copy_code')),
                  ),
                ],
              ),
            ),
            if (status.modelPath.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text(
                p.basename(status.modelPath),
                style: TextStyle(fontSize: 12, color: c.textDim),
              ),
            ],
            const SizedBox(height: 20),
            Row(
              children: [
                Text(
                  L10n.t('swarm_workers'),
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: c.textMain,
                  ),
                ),
                const Spacer(),
                Text(
                  '${L10n.t('swarm_host_share')}: ${status.hostShare.toStringAsFixed(0)}%',
                  style: TextStyle(fontSize: 12, color: c.textSecondary),
                ),
              ],
            ),
            const SizedBox(height: 8),
            if (status.workers.isEmpty)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 16),
                child: Text(
                  L10n.t('swarm_no_workers'),
                  style: TextStyle(color: c.textDim, fontSize: 13),
                ),
              )
            else
              ReorderableListView.builder(
                shrinkWrap: true,
                physics: const NeverScrollableScrollPhysics(),
                itemCount: status.workers.length,
                // onReorderItem already adjusts newIndex for the remove-then-insert
                // case (Flutter 3.41+); do not manually newIndex--.
                onReorderItem: (oldIndex, newIndex) {
                  ref
                      .read(swarmStatusProvider.notifier)
                      .reorderWorkers(oldIndex, newIndex);
                },
                itemBuilder: (context, index) {
                  final w = status.workers[index];
                  return _WorkerTile(
                    key: ValueKey(w.id),
                    worker: w,
                    onRemove: status.running
                        ? null
                        : () => ref
                            .read(swarmStatusProvider.notifier)
                            .removeWorker(w.id),
                    onShareChanged: status.running
                        ? null
                        : (pct) => ref
                            .read(swarmStatusProvider.notifier)
                            .setShare(w.id, pct),
                  );
                },
              ),
            const SizedBox(height: 20),
            if (status.running)
              Container(
                padding: const EdgeInsets.all(10),
                margin: const EdgeInsets.only(bottom: 12),
                decoration: BoxDecoration(
                  color: MemoTheme.accent.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  children: [
                    Icon(Icons.check_circle,
                        size: 18, color: MemoTheme.accent),
                    const SizedBox(width: 8),
                    Text(
                      L10n.t('swarm_running'),
                      style: TextStyle(
                        color: MemoTheme.accent,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ],
                ),
              ),
            Wrap(
              spacing: 12,
              runSpacing: 8,
              children: [
                if (!status.running)
                  FilledButton.icon(
                    onPressed: _busy || status.workers.isEmpty
                        ? null
                        : () async {
                            setState(() => _busy = true);
                            await ref
                                .read(swarmStatusProvider.notifier)
                                .start();
                            if (mounted) setState(() => _busy = false);
                          },
                    icon: const Icon(Icons.play_arrow, size: 18),
                    label: Text(L10n.t('swarm_start')),
                  )
                else
                  FilledButton.tonalIcon(
                    onPressed: _busy
                        ? null
                        : () async {
                            setState(() => _busy = true);
                            await ref
                                .read(swarmStatusProvider.notifier)
                                .stop();
                            if (mounted) setState(() => _busy = false);
                          },
                    icon: const Icon(Icons.stop, size: 18),
                    label: Text(L10n.t('swarm_stop')),
                  ),
                OutlinedButton.icon(
                  onPressed: _busy
                      ? null
                      : () async {
                          setState(() => _busy = true);
                          await ref
                              .read(swarmStatusProvider.notifier)
                              .closeRoom();
                          if (mounted) setState(() => _busy = false);
                        },
                  icon: const Icon(Icons.close, size: 18),
                  label: Text(L10n.t('swarm_close_room')),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}

class _WorkerTile extends StatefulWidget {
  final SwarmWorker worker;
  final VoidCallback? onRemove;
  final ValueChanged<double>? onShareChanged;

  const _WorkerTile({
    super.key,
    required this.worker,
    this.onRemove,
    this.onShareChanged,
  });

  @override
  State<_WorkerTile> createState() => _WorkerTileState();
}

class _WorkerTileState extends State<_WorkerTile> {
  late final TextEditingController _shareCtrl;
  late final FocusNode _shareFocus;

  @override
  void initState() {
    super.initState();
    _shareCtrl = TextEditingController(
      text: widget.worker.sharePercent.toStringAsFixed(0),
    );
    _shareFocus = FocusNode();
    _shareFocus.addListener(_onShareFocusChange);
  }

  @override
  void didUpdateWidget(covariant _WorkerTile oldWidget) {
    super.didUpdateWidget(oldWidget);
    // Don't clobber in-progress typing when status poll refreshes.
    if (!_shareFocus.hasFocus &&
        oldWidget.worker.sharePercent != widget.worker.sharePercent) {
      _shareCtrl.text = widget.worker.sharePercent.toStringAsFixed(0);
    }
  }

  void _onShareFocusChange() {
    if (!_shareFocus.hasFocus) {
      _commitShare();
    }
  }

  void _commitShare() {
    if (widget.onShareChanged == null) return;
    final pct = double.tryParse(_shareCtrl.text.trim());
    if (pct == null) return;
    if (pct == widget.worker.sharePercent) return;
    widget.onShareChanged!(pct);
  }

  @override
  void dispose() {
    _shareFocus.removeListener(_onShareFocusChange);
    _shareFocus.dispose();
    _shareCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final worker = widget.worker;
    return Card(
      color: c.bgPanel,
      margin: const EdgeInsets.symmetric(vertical: 4),
      child: ListTile(
        leading: Icon(
          worker.connected ? Icons.circle : Icons.circle_outlined,
          size: 12,
          color: worker.connected ? Colors.greenAccent : c.textDim,
        ),
        title: Text(
          worker.label.isNotEmpty ? worker.label : worker.id,
          style: TextStyle(color: c.textMain, fontSize: 13),
        ),
        subtitle: Text(
          worker.address,
          style: TextStyle(color: c.textDim, fontSize: 11),
        ),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(L10n.t('swarm_share_pct'),
                style: TextStyle(fontSize: 11, color: c.textDim)),
            const SizedBox(width: 4),
            SizedBox(
              width: 56,
              child: TextFormField(
                controller: _shareCtrl,
                focusNode: _shareFocus,
                enabled: widget.onShareChanged != null,
                keyboardType: TextInputType.number,
                style: TextStyle(fontSize: 13, color: c.textMain),
                decoration: InputDecoration(
                  isDense: true,
                  contentPadding:
                      const EdgeInsets.symmetric(horizontal: 6, vertical: 6),
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(6),
                  ),
                ),
                onFieldSubmitted: (_) => _commitShare(),
                onEditingComplete: _commitShare,
              ),
            ),
            if (widget.onRemove != null)
              IconButton(
                tooltip: L10n.t('swarm_remove_worker'),
                onPressed: widget.onRemove,
                icon: Icon(Icons.delete_outline, size: 18, color: c.textDim),
              ),
            const Icon(Icons.drag_handle, size: 18),
          ],
        ),
      ),
    );
  }
}

// ─── Join view ───────────────────────────────────────────────────

class _JoinSwarmView extends ConsumerStatefulWidget {
  final VoidCallback onBack;
  const _JoinSwarmView({required this.onBack});

  @override
  ConsumerState<_JoinSwarmView> createState() => _JoinSwarmViewState();
}

class _JoinSwarmViewState extends ConsumerState<_JoinSwarmView> {
  final _codeCtrl = TextEditingController();
  bool _busy = false;

  @override
  void dispose() {
    _codeCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final c = MemoTheme.of(context);
    final statusAsync = ref.watch(swarmStatusProvider);
    final status = statusAsync.valueOrNull;
    final joined = status != null && status.isWorker;

    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(24, 8, 24, 24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (!joined) ...[
            Text(
              L10n.t('swarm_paste_code'),
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: c.textMain,
              ),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _codeCtrl,
              onChanged: (_) => setState(() {}),
              style: TextStyle(
                fontFamily: 'monospace',
                fontSize: 12,
                color: c.textMain,
              ),
              maxLines: 3,
              decoration: InputDecoration(
                filled: true,
                fillColor: c.bgPanel,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(8),
                  borderSide: BorderSide(color: c.borderSoft),
                ),
                hintText: 'swarm-…',
                hintStyle: TextStyle(color: c.textDim),
              ),
            ),
            const SizedBox(height: 16),
            Align(
              alignment: Alignment.centerLeft,
              child: FilledButton.icon(
                onPressed: _busy || _codeCtrl.text.trim().isEmpty
                    ? null
                    : () async {
                        setState(() => _busy = true);
                        await ref
                            .read(swarmStatusProvider.notifier)
                            .join(_codeCtrl.text.trim());
                        if (mounted) setState(() => _busy = false);
                      },
                icon: _busy
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Icon(Icons.login, size: 18),
                label: Text(
                  _busy
                      ? L10n.t('swarm_connecting')
                      : L10n.t('swarm_join_btn'),
                ),
              ),
            ),
          ] else ...[
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: MemoTheme.accent.withValues(alpha: 0.12),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: MemoTheme.accent.withValues(alpha: 0.4),
                ),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        status.connected
                            ? Icons.check_circle
                            : Icons.hourglass_top,
                        color: MemoTheme.accent,
                        size: 20,
                      ),
                      const SizedBox(width: 8),
                      Text(
                        status.connected
                            ? L10n.t('swarm_joined')
                            : L10n.t('swarm_connecting'),
                        style: TextStyle(
                          fontWeight: FontWeight.w600,
                          color: MemoTheme.accent,
                        ),
                      ),
                    ],
                  ),
                  if (status.hostAddr.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    Text(
                      status.hostAddr,
                      style: TextStyle(fontSize: 12, color: c.textDim),
                    ),
                  ],
                  if (status.roomCode.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text(
                      status.roomCode,
                      style: TextStyle(
                        fontFamily: 'monospace',
                        fontSize: 11,
                        color: c.textDim,
                      ),
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(height: 16),
            Align(
              alignment: Alignment.centerLeft,
              child: OutlinedButton.icon(
                onPressed: _busy
                    ? null
                    : () async {
                        setState(() => _busy = true);
                        await ref.read(swarmStatusProvider.notifier).leave();
                        if (mounted) setState(() => _busy = false);
                      },
                icon: const Icon(Icons.logout, size: 18),
                label: Text(L10n.t('swarm_leave_btn')),
              ),
            ),
          ],
        ],
      ),
    );
  }
}
