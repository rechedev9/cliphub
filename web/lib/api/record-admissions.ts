import { recordAdmissionPending } from './reel-reconcile.ts';
import type { GenerateFailure, JobStatusView } from './types.ts';

type JobRead = { videoId: string; variant: string; job: JobStatusView };

export class RecordAdmissions {
  private readonly pendingSince = new Map<string, number>();
  private readonly ownFailuresSince = new Map<string, number>();

  accept(videoId: string, sentAt: number): void {
    this.pendingSince.set(videoId, sentAt);
    this.ownFailuresSince.set(videoId, sentAt);
  }

  settle(read: JobRead): GenerateFailure | undefined {
    const failure = this.ownFailure(read);
    const admittedAt = this.pendingSince.get(read.videoId);
    if (admittedAt === undefined) return failure;
    const waiting = failure === undefined && recordAdmissionPending({ jobStatus: read.job.status, admittedAt, now: Date.now() });
    if (!waiting) this.pendingSince.delete(read.videoId);
    return failure;
  }

  pending(videoId: string): boolean {
    return this.pendingSince.has(videoId);
  }

  userActed(videoId: string): void {
    this.ownFailuresSince.delete(videoId);
  }

  forget(videoId: string): void {
    this.pendingSince.delete(videoId);
    this.ownFailuresSince.delete(videoId);
  }

  private ownFailure(read: JobRead): GenerateFailure | undefined {
    const failure = read.job.generateFailure;
    const since = this.ownFailuresSince.get(read.videoId);
    if (failure === undefined || failure.variant !== read.variant || since === undefined) return undefined;
    if (failure.failedAt === undefined) return failure;
    return Date.parse(failure.failedAt) < since ? undefined : failure;
  }
}
