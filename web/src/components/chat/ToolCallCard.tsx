'use client'

import { useState } from 'react'

interface ToolCallCardProps {
  name: string
  args?: Record<string, any>
  result?: string
}

export default function ToolCallCard({ name, args, result }: ToolCallCardProps) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="border border-gray-300 rounded-lg my-2 overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-3 py-2 bg-gray-100 text-left text-sm font-mono flex items-center justify-between hover:bg-gray-200"
      >
        <span>Tool: {name}</span>
        <span>{expanded ? '-' : '+'}</span>
      </button>
      {expanded && (
        <div className="px-3 py-2 text-sm">
          {args && (
            <div className="mb-2">
              <div className="text-gray-500 text-xs mb-1">Arguments:</div>
              <pre className="bg-gray-50 p-2 rounded text-xs overflow-auto">
                {JSON.stringify(args, null, 2)}
              </pre>
            </div>
          )}
          {result && (
            <div>
              <div className="text-gray-500 text-xs mb-1">Result:</div>
              <pre className="bg-gray-50 p-2 rounded text-xs overflow-auto">
                {result}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
