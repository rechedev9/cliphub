import type { ReactNode } from 'react';
import { EditorWorkspace } from '@/components/editor/editor-workspace';

export default async function EditorProjectPage({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<ReactNode> {
  const { id } = await params;
  return <EditorWorkspace projectId={id} />;
}
