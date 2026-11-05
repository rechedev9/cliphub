import type { ReactNode } from 'react';
import { CaptureStorage } from '@/components/storage/capture-storage';
import { StorageFilesScreen } from '@/components/storage/storage-files';

/** Demos on the PC, Studio's videos and Full Demo captures, with delete per item. */
export default function StoragePage(): ReactNode {
  return (
    <StorageFilesScreen>
      <div className="measure-read">
        <CaptureStorage />
      </div>
    </StorageFilesScreen>
  );
}
