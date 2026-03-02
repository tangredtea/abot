const isProd = process.env.NODE_ENV === 'production'

/** @type {import('next').NextConfig} */
const nextConfig = isProd
  ? {
      output: 'export',
      trailingSlash: true,
      images: {
        unoptimized: true,
      },
    }
  : {
      images: {
        unoptimized: true,
      },
      async rewrites() {
        return [
          {
            source: '/api/:path*',
            destination: 'http://localhost:3001/api/:path*',
          },
        ]
      },
    }

module.exports = nextConfig
