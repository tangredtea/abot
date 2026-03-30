'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/stores/auth'

export default function Home() {
  const router = useRouter()
  const token = useAuthStore((s) => s.token)

  useEffect(() => {
    if (token) {
      router.replace('/chat')
    } else {
      router.replace('/login')
    }
  }, [token, router])

  return (
    <div className="flex items-center justify-center min-h-screen">
      <div className="text-gray-400">Loading...</div>
    </div>
  )
}
