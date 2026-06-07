/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  experimental: {
    // Native server-only modules must stay external so webpack never bundles
    // their .node binaries (which would break them). better-sqlite3 + argon2.
    serverComponentsExternalPackages: ["better-sqlite3", "argon2"],
  },
};

export default nextConfig;
