import { redirect } from "next/navigation";
import { getDb } from "../server/db";
import { isOnboarded } from "../lib/auth";

export const dynamic = "force-dynamic";

export default function Home() {
  redirect(isOnboarded(getDb()) ? "/dashboard" : "/onboarding");
}
