import { redirect } from "next/navigation";
import { getDb } from "../server/db";
import { isOnboarded } from "../lib/auth";
import { getConfig } from "../lib/config";

export const dynamic = "force-dynamic";

export default function Home() {
  if (getConfig().DISABLE_AUTH) redirect("/dashboard");
  redirect(isOnboarded(getDb()) ? "/dashboard" : "/onboarding");
}
