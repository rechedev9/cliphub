/** @type {import('next').NextConfig} */
const nextConfig = {
  // Do not recreate portal/AGENTS.md and portal/CLAUDE.md on `next dev`.
  agentRules: false,
  output: "standalone",
};

export default nextConfig;
