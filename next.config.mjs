/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Native / server-only modules must stay external so the bundler never tries to
  // bundle their .node binaries or non-ESM assets (Turbopack errors on ssh2's
  // cpu-features asset otherwise). dockerode pulls in docker-modem -> ssh2 ->
  // cpu-features. Renamed from experimental.serverComponentsExternalPackages in Next 15+.
  serverExternalPackages: ["better-sqlite3", "argon2", "dockerode", "ssh2", "cpu-features"],
};

export default nextConfig;
