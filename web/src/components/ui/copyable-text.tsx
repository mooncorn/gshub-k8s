import { useState, useCallback } from "react"
import { Copy, Check } from "lucide-react"
import { cn } from "@/lib/utils"

interface CopyableTextProps {
  value: string
  className?: string
}

export function CopyableText({ value, className }: CopyableTextProps) {
  const [copied, setCopied] = useState(false)

  const handleCopy = useCallback(async () => {
    await navigator.clipboard.writeText(value)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }, [value])

  return (
    <div
      className={cn(
        "group flex items-center gap-2 rounded bg-muted px-3 py-2 font-mono text-sm",
        className
      )}
    >
      <span className="flex-1 truncate">{value}</span>
      <button
        onClick={handleCopy}
        className="shrink-0 cursor-pointer text-muted-foreground transition-colors hover:text-foreground"
        title={copied ? "Copied!" : "Copy to clipboard"}
      >
        {copied ? (
          <Check className="h-4 w-4 text-green-500" />
        ) : (
          <Copy className="h-4 w-4" />
        )}
      </button>
    </div>
  )
}
