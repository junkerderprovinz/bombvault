import type { ReactNode } from "react";
import "./globals.css";

export const metadata = {
  title: "BombVault",
  description: "Backup & disaster recovery for Docker containers and KVM/libvirt VMs.",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
