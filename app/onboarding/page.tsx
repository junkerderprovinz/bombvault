import { redirect } from "next/navigation";
import { getDb } from "../../server/db";
import { isOnboarded } from "../../lib/auth";
import { completeOnboarding } from "../actions/auth";

export const dynamic = "force-dynamic";

export default function OnboardingPage() {
  if (isOnboarded(getDb())) redirect("/login");
  return (
    <main style={{ padding: "2rem", maxWidth: 420 }}>
      <h1>Welcome to BombVault</h1>
      <p>Set the admin password to finish setup.</p>
      <form action={completeOnboarding}>
        <input
          type="password"
          name="password"
          placeholder="Admin password (min 8 chars)"
          minLength={8}
          required
          style={{ display: "block", width: "100%", padding: 8, marginBottom: 12 }}
        />
        <button type="submit">Create admin</button>
      </form>
    </main>
  );
}
