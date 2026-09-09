import { redirect } from "next/navigation";

import { auth } from "@/auth";
import { isAdminUser } from "@/lib/admin";

export default async function AdminLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const session = await auth();
  if (!session?.user?.id) redirect("/login");
  if (!(await isAdminUser(session.user.id))) redirect("/dashboard");

  return <>{children}</>;
}
