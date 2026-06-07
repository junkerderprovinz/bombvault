import { redirect } from "next/navigation";
import { getDb } from "../../server/db";
import { isOnboarded } from "../../lib/auth";
import { login } from "../actions/auth";

export const dynamic = "force-dynamic";

export default function LoginPage({
  searchParams,
}: {
  searchParams: { error?: string };
}) {
  if (!isOnboarded(getDb())) redirect("/onboarding");
  return (
    <main style={{ padding: "2rem", maxWidth: 420 }}>
      <h1>BombVault — Sign in</h1>
      {searchParams.error ? <p style={{ color: "#fa4d56" }}>Invalid password.</p> : null}
      <form action={login}>
        <input
          type="password"
          name="password"
          placeholder="Admin password"
          required
          style={{ display: "block", width: "100%", padding: 8, marginBottom: 12 }}
        />
        <button type="submit">Sign in</button>
      </form>
    </main>
  );
}
