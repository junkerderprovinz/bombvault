import Link from "next/link";
import { logout } from "../actions/auth";
import { requireSession } from "../../lib/auth-server";

// Protected by middleware (first gate) AND requireSession() (defense-in-depth,
// SEC-005): re-verifies the token and checks the session_version server-side.
export const dynamic = "force-dynamic";

export default async function DashboardPage() {
  await requireSession();
  return (
    <main style={{ padding: "2rem" }}>
      <h1>BombVault — Dashboard</h1>
      <p>P0 foundation is running.</p>
      <p>
        <Link href="/spike">Run the host-integration spike →</Link>
      </p>
      <form action={logout}>
        <button type="submit">Sign out</button>
      </form>
    </main>
  );
}
