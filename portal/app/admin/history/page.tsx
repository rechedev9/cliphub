import { HistoryView } from "./history-view";

function first(value: string | string[] | undefined): string {
  return typeof value === "string" ? value : "";
}

export default async function AdminHistoryPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  return (
    <HistoryView
      initial={{ status: first(params.status), kind: first(params.kind), userId: first(params.userId) }}
    />
  );
}
