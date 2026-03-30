import './globals.css'
import type { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'ABot Console',
  description: 'ABot AI Agent Web Console',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body className="bg-gray-50 text-gray-900 min-h-screen">
        {children}
      </body>
    </html>
  )
}
