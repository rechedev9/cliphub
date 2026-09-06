/** Enrich raw process reports only with measurements from the same driven workload. */
export function attachTacticalWorkload(report, workload) {
  if (report.scenario !== workload.scenario || !['tactical-analysis', 'tactical-replay'].includes(report.scenario)) {
    throw new Error('tactical workload scenario mismatch');
  }
  if (typeof workload.id !== 'string' || workload.id === '') throw new Error('workload id is required');
  const summary = { ...report.summary };
  if (report.scenario === 'tactical-analysis') {
    if (!Number.isFinite(workload.operationMS) || workload.operationMS <= 0) throw new Error('operation duration is required');
    summary.operation_ms = workload.operationMS;
  } else {
    const intervals = workload.frameIntervalsMS;
    if (!Array.isArray(intervals) || intervals.length < 2 || intervals.some((value) => !Number.isFinite(value) || value <= 0)) {
      throw new Error('real frame intervals are required');
    }
    const sorted = intervals.toSorted((a, b) => a - b);
    summary.frame_p95_ms = percentile(sorted, 95);
    summary.frame_p99_ms = percentile(sorted, 99);
    summary.frame_count = intervals.length;
  }
  return { ...report, workload_id: workload.id, summary };
}

function percentile(sorted, percent) {
  return sorted[Math.max(0, Math.ceil(sorted.length * percent / 100) - 1)];
}
