/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Native server-only modules must stay external so the bundler never bundles
  // their .node binaries (which would break them). better-sqlite3 + argon2.
  // Renamed from experimental.serverComponentsExternalPackages in Next 15+.
  serverExternalPackages: ["better-sqlite3", "argon2"],
};

export default nextConfig;
