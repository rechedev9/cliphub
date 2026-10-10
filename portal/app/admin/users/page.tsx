import { UsersView } from "./users-view";

export default async function AdminUsersPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { access } = await searchParams;
  return <UsersView initialAccess={typeof access === "string" ? access : ""} />;
}
