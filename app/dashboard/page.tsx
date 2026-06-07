import Link from "next/link";
import { logout } from "../actions/auth";

export const dynamic = "force-dynamic";

export default function DashboardPage() {
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
