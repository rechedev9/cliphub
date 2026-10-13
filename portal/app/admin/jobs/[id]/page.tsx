import { JobView } from "./job-view";

export default async function AdminJobPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <JobView id={id} />;
}
