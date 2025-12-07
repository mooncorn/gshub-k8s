import { useEffect, useState, useRef } from "react"
import { Link, useSearchParams } from "react-router-dom"
import { authApi } from "@/api/auth"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"

export function VerifyEmailPage() {
  const [searchParams] = useSearchParams()
  const token = searchParams.get("token")
  const initialized = useRef(false)

  const [status, setStatus] = useState<"loading" | "success" | "error">(
    token ? "loading" : "error"
  )

  useEffect(() => {
    if (initialized.current || !token) return
    initialized.current = true

    authApi
      .verifyEmail(token)
      .then(() => setStatus("success"))
      .catch(() => setStatus("error"))
  }, [token])

  if (status === "loading") {
    return (
      <Card>
        <CardHeader className="space-y-1">
          <CardTitle className="text-xl">Verifying email...</CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-4 w-full" />
        </CardContent>
      </Card>
    )
  }

  if (status === "error") {
    return (
      <Card>
        <CardHeader className="space-y-1">
          <CardTitle className="text-xl">Verification failed</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            This verification link is invalid or has expired.
          </p>
          <Link to="/login">
            <Button variant="outline" className="w-full">
              Back to sign in
            </Button>
          </Link>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader className="space-y-1">
        <CardTitle className="text-xl">Email verified</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">
          Your email has been verified successfully. You can now sign in.
        </p>
        <Link to="/login">
          <Button className="w-full">Sign in</Button>
        </Link>
      </CardContent>
    </Card>
  )
}
