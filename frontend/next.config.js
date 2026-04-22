/** @type {import('next').NextConfig} */
const nextConfig = {
  // Go backend URL
  async rewrites() {
    return [
      {
        source: '/api/backend/:path*',
        destination: `${process.env.BACKEND_URL || 'http://localhost:8080'}/:path*`,
      },
    ]
  },
}

module.exports = nextConfig
