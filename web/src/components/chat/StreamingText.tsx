'use client'

interface StreamingTextProps {
  content: string
}

export default function StreamingText({ content }: StreamingTextProps) {
  return (
    <div className="flex justify-start">
      <div className="max-w-[80%] rounded-lg px-4 py-2 bg-gray-200 text-gray-900 whitespace-pre-wrap">
        {content}
        <span className="inline-block w-2 h-4 bg-gray-500 ml-0.5 animate-pulse" />
      </div>
    </div>
  )
}
