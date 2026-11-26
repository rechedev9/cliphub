import { recordAdmissionPending } from './reel-reconcile.ts';
import type { GenerateFailure, JobStatusView } from './types.ts';

type JobRead = { videoId: string; variant: string; job: JobStatusView; readAt: number };
type Admission = { sentAt: number; acceptedAt: number };

export class RecordAdmissions {
  private readonly pendingSince = new Map<string, Admission>();
  private readonly ownFailuresSince = new Map<string, number>();

  accept(videoId: string, sentAt: number): void {
    this.pendingSince.set(videoId, { sentAt, acceptedAt: Date.now() });
    this.ownFailuresSince.set(videoId, sentAt);
  }

  settle(read: JobRead): GenerateFailure | undefined {
    const failure = this.ownFailure(read);
    const admission = this.pendingSince.get(read.videoId);
    if (admission === undefined) return failure;
    if (failure !== undefined || !this.stillAdmitted(read, admission)) this.pendingSince.delete(read.videoId);
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

  private stillAdmitted(read: JobRead, admission: Admission): boolean {
    if (read.job.status !== 'failed') return recordAdmissionPending({ jobStatus: read.job.status, admittedAt: admission.sentAt, now: Date.now() });
    if (read.readAt <= admission.acceptedAt) return true;
    const run = read.job.generateRun;
    return run?.active === true && run.variant === read.variant;
  }

  private ownFailure(read: JobRead): GenerateFailure | undefined {
    const run = read.job.generateRun;
    const since = this.ownFailuresSince.get(read.videoId);
    if (run === undefined || run.active || run.variant !== read.variant || since === undefined) return undefined;
    if (run.failedAt === undefined) return run;
    return Date.parse(run.failedAt) < since ? undefined : run;
  }
}
