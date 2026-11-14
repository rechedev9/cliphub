// A static export served by deploy/landing/Caddyfile, which sets the security headers
// (an export ignores headers()).
/** @type {import('next').NextConfig} */
export default {
  output: 'export',
  poweredByHeader: false,
};
