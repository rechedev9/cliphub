import {
  isApprovalMode,
  isCloudAccess,
  type ApprovalMode,
  type CloudAccess,
  type CloudKind,
} from "./job-types.ts";

export interface UserLimits {
  maxActive: number;
  dailySeconds: number;
}

export interface LimitOverrides {
  maxActive: number | null;
  dailySeconds: number | null;
}

const MAX_ACTIVE_FULL_DEMO = 1;

function positiveInt(raw: string | undefined, fallback: number): number {
  const value = Number(raw);
  return Number.isInteger(value) && value > 0 ? value : fallback;
}

export function loadUserLimits(
  env: Record<string, string | undefined> = process.env,
): UserLimits {
  return {
    maxActive: positiveInt(env.MAX_ACTIVE_REQUESTS_PER_USER, 3),
    dailySeconds: positiveInt(env.MAX_DAILY_SECONDS_PER_USER, 5400),
  };
}

export function effectiveLimits(
  defaults: UserLimits,
  overrides: LimitOverrides | null,
): UserLimits {
  return {
    maxActive: overrides?.maxActive ?? defaults.maxActive,
    dailySeconds: overrides?.dailySeconds ?? defaults.dailySeconds,
  };
}

export function loadAccessDefault(
  env: Record<string, string | undefined> = process.env,
): CloudAccess {
  return isCloudAccess(env.CLOUD_ACCESS_DEFAULT) ? env.CLOUD_ACCESS_DEFAULT : "pending";
}

export interface AccessInput {
  stored: string | null;
  isAdmin: boolean;
  accessDefault: CloudAccess;
}

// Admins are always allowed; a user with no stored access gets the default.
export function resolveAccess(input: AccessInput): CloudAccess {
  if (input.isAdmin) return "allowed";
  return isCloudAccess(input.stored) ? input.stored : input.accessDefault;
}

export function loadApprovalMode(
  env: Record<string, string | undefined> = process.env,
): ApprovalMode {
  return isApprovalMode(env.CLOUD_APPROVAL) ? env.CLOUD_APPROVAL : "required";
}

export interface ApprovalInput {
  mode: ApprovalMode;
  isAdmin: boolean;
  autoApprove: boolean;
}

// An operator's own jobs and a trusted user's jobs go straight to the queue.
export function needsApproval(input: ApprovalInput): boolean {
  return input.mode === "required" && !input.isAdmin && !input.autoApprove;
}

export interface CreateLimitInput {
  limits: UserLimits;
  kind: CloudKind;
  // Jobs of the user that have not finished, waiting for approval included.
  active: number;
  activeFullDemo: number;
  usedSeconds: number;
  // Estimates of the user's jobs no worker holds yet.
  committedSeconds: number;
  estimatedSeconds: number;
}

export type LimitViolation = "limit_active" | "limit_daily" | "limit_full_demo";

export function checkCreateLimits(input: CreateLimitInput): LimitViolation | null {
  const { limits } = input;
  if (input.active >= limits.maxActive) return "limit_active";
  if (input.kind === "full_demo" && input.activeFullDemo >= MAX_ACTIVE_FULL_DEMO) {
    return "limit_full_demo";
  }
  const total = input.usedSeconds + input.committedSeconds + input.estimatedSeconds;
  if (total > limits.dailySeconds) return "limit_daily";
  return null;
}
