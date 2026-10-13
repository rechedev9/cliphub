import { mkdir, statfs } from "node:fs/promises";

import { loadCloudConfig } from "./cloud-config.ts";

export interface StorageStats {
  freeBytes: number;
  totalBytes: number;
}

// Free and total bytes of the volume that holds UPLOAD_DIR.
export async function storageStats(): Promise<StorageStats> {
  const { uploadDir } = loadCloudConfig();
  await mkdir(uploadDir, { recursive: true });
  const stats = await statfs(uploadDir);
  return {
    freeBytes: stats.bavail * stats.bsize,
    totalBytes: stats.blocks * stats.bsize,
  };
}

export interface FreeSpaceOptions {
  // Defaults to MIN_FREE_BYTES.
  floorBytes?: number;
  // Bytes already promised to uploads in flight, which the volume does not show yet.
  reservedBytes?: number;
}

// False when the volume, less what is reserved, is under the floor.
export async function hasFreeSpace(options: FreeSpaceOptions = {}): Promise<boolean> {
  const floorBytes = options.floorBytes ?? loadCloudConfig().minFreeBytes;
  const { freeBytes } = await storageStats();
  return freeBytes - (options.reservedBytes ?? 0) >= floorBytes;
}
