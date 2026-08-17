import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/l10n.dart';
import '../../../core/theme.dart';
import '../../../models/gpu_info.dart';
import '../../../models/usage_stats.dart';
import '../../../providers/settings_provider.dart';
import '../../../providers/chat_provider.dart';
import '../../../core/friendly_error.dart';

class StatsTab extends ConsumerStatefulWidget {
  const StatsTab({super.key});

  @override
  ConsumerState<StatsTab> createState() => _StatsTabState();
}

class _StatsTabState extends ConsumerState<StatsTab> {
  MemoryStats? _memoryStats;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadMemoryStats());
  }

  Future<void> _loadMemoryStats() async {
    try {
      final stats = await ref.read(apiClientProvider).getMemoryStats();
      if (mounted) setState(() => _memoryStats = stats);
    } catch (_) {
      // Best-effort extra card — the usage-stats section above is the
      // primary content of this tab and must not fail because of this.
    }
  }

  @override
  Widget build(BuildContext context) {
    final statsAsync = ref.watch(usageStatsProvider);
    final selectedDays = ref.watch(statsDaysProvider);

    return ListView(
      padding: EdgeInsets.all(32),
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    L10n.t('stats_title'),
                    style: Theme.of(context).textTheme.titleLarge?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: MemoTheme.of(context).textMain,
                    ),
                  ),
                  SizedBox(height: 8),
                  Text(
                    L10n.t('stats_subtitle'),
                    style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
                  ),
                ],
              ),
            ),
            _DaysSelector(selected: selectedDays),
            SizedBox(width: 8),
            IconButton(
              tooltip: L10n.t('stats_refresh'),
              icon: Icon(Icons.refresh, color: MemoTheme.of(context).textDim),
              onPressed: () {
                ref.read(usageStatsProvider.notifier).refresh();
                _loadMemoryStats();
              },
            ),
          ],
        ),
        SizedBox(height: 20),
        statsAsync.when(
          loading: () => Padding(
            padding: EdgeInsets.symmetric(vertical: 60),
            child: Center(child: CircularProgressIndicator()),
          ),
          error: (e, _) => Padding(
            padding: EdgeInsets.symmetric(vertical: 40),
            child: Text(
              L10n.t('stats_load_error', {'e': FriendlyError.describeGeneric(e)}),
              style: TextStyle(color: MemoTheme.red),
            ),
          ),
          data: (stats) {
            if (stats.totalRequests == 0) {
              return _EmptyState();
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _StatCardsRow(stats: stats, memoryStats: _memoryStats),
                SizedBox(height: 24),
                _UsageChart(stats: stats, days: selectedDays),
                SizedBox(height: 24),
                _ModelBreakdown(stats: stats),
              ],
            );
          },
        ),
      ],
    );
  }
}

class _DaysSelector extends ConsumerWidget {
  final int selected;
  const _DaysSelector({required this.selected});

  static const _options = [7, 30, 90];

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = MemoTheme.of(context);
    return Container(
      padding: EdgeInsets.all(2),
      decoration: BoxDecoration(
        color: theme.bgApp,
        borderRadius: BorderRadius.circular(MemoTheme.radiusSm),
        border: Border.all(color: theme.borderSoft),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          for (final days in _options)
            _DaysSelectorOption(
              days: days,
              active: days == selected,
              onTap: () => ref.read(statsDaysProvider.notifier).state = days,
            ),
        ],
      ),
    );
  }
}

class _DaysSelectorOption extends StatelessWidget {
  final int days;
  final bool active;
  final VoidCallback onTap;
  const _DaysSelectorOption({
    required this.days,
    required this.active,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = MemoTheme.of(context);
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: active ? MemoTheme.accent : Colors.transparent,
          borderRadius: BorderRadius.circular(MemoTheme.radiusSm - 2),
        ),
        child: Text(
          L10n.t('stats_days_option', {'n': '$days'}),
          style: TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.w600,
            color: active ? Colors.white : theme.textDim,
          ),
        ),
      ),
    );
  }
}

class _EmptyState extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(32),
      decoration: BoxDecoration(
        color: MemoTheme.of(context).bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: MemoTheme.of(context).borderSoft),
      ),
      child: Column(
        children: [
          Icon(Icons.bar_chart, size: 40, color: MemoTheme.of(context).textDim),
          SizedBox(height: 12),
          Text(
            L10n.t('stats_empty_title'),
            style: TextStyle(fontWeight: FontWeight.w600, color: MemoTheme.of(context).textMain),
          ),
          SizedBox(height: 6),
          Text(
            L10n.t('stats_empty_body'),
            textAlign: TextAlign.center,
            style: TextStyle(color: MemoTheme.of(context).textDim, fontSize: 13),
          ),
        ],
      ),
    );
  }
}

class _StatCardsRow extends StatelessWidget {
  final UsageStatsSummary stats;
  final MemoryStats? memoryStats;
  const _StatCardsRow({required this.stats, this.memoryStats});

  @override
  Widget build(BuildContext context) {
    return Wrap(
      spacing: 12,
      runSpacing: 12,
      children: [
        _StatCard(
          label: L10n.t('stats_total_requests'),
          value: '${stats.totalRequests}',
        ),
        _StatCard(
          label: L10n.t('stats_input_tokens'),
          value: _formatTokens(stats.totalPromptTokens),
          accent: true,
        ),
        _StatCard(
          label: L10n.t('stats_output_tokens'),
          value: _formatTokens(stats.totalCompletionTokens),
          color: MemoTheme.green,
        ),
        _StatCard(
          label: L10n.t('stats_avg_speed'),
          value: L10n.t('stats_speed_unit', {'speed': stats.avgTokensPerSecond.toStringAsFixed(1)}),
        ),
        _StatCard(
          label: L10n.t('stats_most_used_model'),
          value: stats.mostUsedModel.isEmpty ? L10n.t('stats_no_model') : stats.mostUsedModel,
          wide: true,
        ),
        // Not from usage_events (LLM call throughput) like the cards above —
        // this is the pinned-facts block's own footprint, i.e. what every
        // single one of those requests already paid as fixed system-prompt
        // overhead before the user's actual message. Shown here (not just
        // Memory tab, where the pinned *count* already lives) since this
        // tab is specifically about "how many tokens am I spending."
        if (memoryStats != null && memoryStats!.explicitCount > 0)
          _StatCard(
            label: L10n.t('stats_pinned_tokens'),
            value: _formatTokens(memoryStats!.pinnedTokens),
            color: MemoTheme.accent,
          ),
      ],
    );
  }
}

String _formatTokens(int tokens) {
  if (tokens >= 1000000) return '${(tokens / 1000000).toStringAsFixed(1)}M';
  if (tokens >= 1000) return '${(tokens / 1000).toStringAsFixed(1)}K';
  return '$tokens';
}

class _StatCard extends StatelessWidget {
  final String label;
  final String value;
  final Color? color;
  final bool accent;
  final bool wide;

  const _StatCard({
    required this.label,
    required this.value,
    this.color,
    this.accent = false,
    this.wide = false,
  });

  @override
  Widget build(BuildContext context) {
    final resolvedColor = color ?? (accent ? MemoTheme.accent : MemoTheme.of(context).textMain);
    return Container(
      width: wide ? 220 : 150,
      padding: EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: MemoTheme.of(context).bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: MemoTheme.of(context).borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            value,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold, color: resolvedColor),
          ),
          SizedBox(height: 4),
          Text(
            label,
            style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim),
          ),
        ],
      ),
    );
  }
}

/// Fills gaps in the backend's sparse daily series so the chart always shows
/// a continuous trailing window instead of jumbled/uneven x-axis spacing.
List<DailyUsage> _fillDailySeries(List<DailyUsage> daily, int days) {
  final byDate = {for (final d in daily) d.date: d};
  final now = DateTime.now();
  final filled = <DailyUsage>[];
  for (int i = days - 1; i >= 0; i--) {
    final day = now.subtract(Duration(days: i));
    final key = '${day.year.toString().padLeft(4, '0')}-'
        '${day.month.toString().padLeft(2, '0')}-'
        '${day.day.toString().padLeft(2, '0')}';
    filled.add(byDate[key] ??
        DailyUsage(date: key, promptTokens: 0, completionTokens: 0, requests: 0));
  }
  return filled;
}

class _UsageChart extends StatelessWidget {
  final UsageStatsSummary stats;
  final int days;
  const _UsageChart({required this.stats, required this.days});

  @override
  Widget build(BuildContext context) {
    // Regression: this used to hardcode 30 regardless of what range was
    // actually fetched — picking "7 days" still padded the chart out to a
    // full 30-day window (mostly empty bars looking like inactivity that
    // was really just "never asked for"), and "90 days" silently dropped
    // 60 days of real data off the front. days now always matches
    // statsDaysProvider, the same value the fetch itself used.
    final series = _fillDailySeries(stats.daily, days);
    final maxY = series
        .map((d) => d.totalTokens)
        .fold<int>(0, (a, b) => a > b ? a : b)
        .toDouble();

    return Container(
      padding: EdgeInsets.all(16),
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
              Text(
                L10n.t('stats_chart_title'),
                style: TextStyle(fontWeight: FontWeight.w600, color: MemoTheme.of(context).textMain),
              ),
              Spacer(),
              _LegendDot(color: MemoTheme.accent, label: L10n.t('stats_chart_legend_input')),
              SizedBox(width: 12),
              _LegendDot(color: MemoTheme.green, label: L10n.t('stats_chart_legend_output')),
            ],
          ),
          SizedBox(height: 16),
          SizedBox(
            height: 220,
            child: maxY <= 0
                ? Center(
                    child: Text(
                      L10n.t('stats_empty_title'),
                      style: TextStyle(color: MemoTheme.of(context).textDim),
                    ),
                  )
                : BarChart(
                    BarChartData(
                      maxY: maxY * 1.15,
                      alignment: BarChartAlignment.spaceAround,
                      gridData: FlGridData(
                        show: true,
                        drawVerticalLine: false,
                        horizontalInterval: (maxY / 4).clamp(1, double.infinity),
                        getDrawingHorizontalLine: (_) => FlLine(
                          color: MemoTheme.of(context).borderSoft,
                          strokeWidth: 1,
                        ),
                      ),
                      borderData: FlBorderData(show: false),
                      titlesData: FlTitlesData(
                        topTitles: AxisTitles(sideTitles: SideTitles(showTitles: false)),
                        rightTitles: AxisTitles(sideTitles: SideTitles(showTitles: false)),
                        leftTitles: AxisTitles(
                          sideTitles: SideTitles(
                            showTitles: true,
                            reservedSize: 36,
                            getTitlesWidget: (value, meta) => Text(
                              _formatTokens(value.toInt()),
                              style: TextStyle(fontSize: 10, color: MemoTheme.of(context).textDim),
                            ),
                          ),
                        ),
                        bottomTitles: AxisTitles(
                          sideTitles: SideTitles(
                            showTitles: true,
                            reservedSize: 24,
                            interval: (series.length / 6).clamp(1, series.length).roundToDouble(),
                            getTitlesWidget: (value, meta) {
                              final i = value.toInt();
                              if (i < 0 || i >= series.length) return SizedBox.shrink();
                              final parts = series[i].date.split('-');
                              final label = parts.length == 3 ? '${parts[1]}/${parts[2]}' : '';
                              return Padding(
                                padding: EdgeInsets.only(top: 6),
                                child: Text(
                                  label,
                                  style: TextStyle(fontSize: 10, color: MemoTheme.of(context).textDim),
                                ),
                              );
                            },
                          ),
                        ),
                      ),
                      barGroups: [
                        for (int i = 0; i < series.length; i++)
                          BarChartGroupData(
                            x: i,
                            barRods: [
                              BarChartRodData(
                                toY: series[i].totalTokens.toDouble(),
                                width: (280 / series.length).clamp(3, 14),
                                borderRadius: BorderRadius.circular(2),
                                rodStackItems: [
                                  BarChartRodStackItem(0, series[i].promptTokens.toDouble(), MemoTheme.accent),
                                  BarChartRodStackItem(
                                    series[i].promptTokens.toDouble(),
                                    series[i].totalTokens.toDouble(),
                                    MemoTheme.green,
                                  ),
                                ],
                              ),
                            ],
                          ),
                      ],
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}

class _LegendDot extends StatelessWidget {
  final Color color;
  final String label;
  const _LegendDot({required this.color, required this.label});

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(width: 8, height: 8, decoration: BoxDecoration(color: color, shape: BoxShape.circle)),
        SizedBox(width: 6),
        Text(label, style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim)),
      ],
    );
  }
}

class _ModelBreakdown extends StatelessWidget {
  final UsageStatsSummary stats;
  const _ModelBreakdown({required this.stats});

  @override
  Widget build(BuildContext context) {
    if (stats.modelBreakdown.isEmpty) return SizedBox.shrink();
    final maxRequests = stats.modelBreakdown
        .map((m) => m.requests)
        .fold<int>(0, (a, b) => a > b ? a : b);

    return Container(
      padding: EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: MemoTheme.of(context).bgPanel,
        borderRadius: BorderRadius.circular(MemoTheme.radiusMd),
        border: Border.all(color: MemoTheme.of(context).borderSoft),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            L10n.t('stats_model_breakdown_title'),
            style: TextStyle(fontWeight: FontWeight.w600, color: MemoTheme.of(context).textMain),
          ),
          SizedBox(height: 12),
          for (final m in stats.modelBreakdown) ...[
            Padding(
              padding: EdgeInsets.only(bottom: 10),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          m.model,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(fontSize: 13, color: MemoTheme.of(context).textMain),
                        ),
                      ),
                      Text(
                        L10n.t('stats_model_requests', {'count': '${m.requests}'}),
                        style: TextStyle(fontSize: 12, color: MemoTheme.of(context).textDim),
                      ),
                    ],
                  ),
                  SizedBox(height: 4),
                  ClipRRect(
                    borderRadius: BorderRadius.circular(3),
                    child: LinearProgressIndicator(
                      value: maxRequests > 0 ? m.requests / maxRequests : 0,
                      minHeight: 6,
                      backgroundColor: MemoTheme.of(context).borderSoft,
                      valueColor: AlwaysStoppedAnimation(MemoTheme.accent),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}
