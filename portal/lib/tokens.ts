import { createHash, randomBytes, randomInt } from "node:crypto";

const DEVICE_PREFIX = "chd_";
const HEX_TOKEN = /^[0-9a-f]{64}$/;
// No I, O, 0 or 1, so a code read aloud or typed by hand is unambiguous.
const USER_CODE_ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
const USER_CODE_LENGTH = 8;

function randomHex(): string {
  return randomBytes(32).toString("hex");
}

export function newWorkerToken(): string {
  return randomHex();
}

export function newDeviceToken(): string {
  return DEVICE_PREFIX + randomHex();
}

export function newPollToken(): string {
  return randomHex();
}

export function isWorkerToken(token: string): boolean {
  return HEX_TOKEN.test(token);
}

export function isDeviceToken(token: string): boolean {
  return (
    token.startsWith(DEVICE_PREFIX) && HEX_TOKEN.test(token.slice(DEVICE_PREFIX.length))
  );
}

export function isSha256Hex(value: unknown): value is string {
  return typeof value === "string" && HEX_TOKEN.test(value);
}

export function hashToken(token: string): string {
  return createHash("sha256").update(token).digest("hex");
}

// The stored form: 8 characters, no dash.
export function newUserCode(): string {
  let code = "";
  for (let i = 0; i < USER_CODE_LENGTH; i += 1) {
    code += USER_CODE_ALPHABET[randomInt(USER_CODE_ALPHABET.length)] ?? "";
  }
  return code;
}

export function formatUserCode(code: string): string {
  return `${code.slice(0, 4)}-${code.slice(4)}`;
}

// Accepts what a person types (lowercase, dash, spaces) and returns the stored form, or null.
export function normalizeUserCode(input: string): string | null {
  const code = input.toUpperCase().replace(/[\s-]/g, "");
  if (code.length !== USER_CODE_LENGTH) return null;
  return [...code].every((char) => USER_CODE_ALPHABET.includes(char)) ? code : null;
}

export function bearerToken(request: Request): string | null {
  const header = request.headers.get("authorization") ?? "";
  const prefix = "Bearer ";
  return header.startsWith(prefix) ? header.slice(prefix.length).trim() : null;
}
