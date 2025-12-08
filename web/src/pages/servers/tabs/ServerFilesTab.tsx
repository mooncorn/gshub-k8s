import { FolderOpen } from "lucide-react"
import { Card, CardContent } from "@/components/ui/card"

export function ServerFilesTab() {
  return (
    <div className="space-y-6">
      <Card>
        <CardContent className="p-12 text-center">
          <FolderOpen className="mx-auto h-12 w-12 text-muted-foreground/50" />
          <h2 className="mt-4 text-lg font-medium">File Management</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            File management is coming soon. You'll be able to browse, upload,
            and download server files directly from here.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
