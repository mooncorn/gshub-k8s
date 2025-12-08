import { useState } from "react"
import { AlertCircle, Calendar, CreditCard, RefreshCw } from "lucide-react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { useCancelSubscription, useResumeSubscription, useResubscribe } from "@/hooks/useBilling"
import { GAMES, PLANS } from "@/lib/constants"
import type { ServerSubscription } from "@/api/billing"

interface SubscriptionCardProps {
  subscription: ServerSubscription
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString("en-US", {
    year: "numeric",
    month: "long",
    day: "numeric",
  })
}

export function SubscriptionCard({ subscription }: SubscriptionCardProps) {
  const [showCancelConfirm, setShowCancelConfirm] = useState(false)
  const cancelMutation = useCancelSubscription()
  const resumeMutation = useResumeSubscription()
  const resubscribeMutation = useResubscribe()

  const game = GAMES[subscription.game]
  const plan = PLANS[subscription.plan]
  const isExpired = subscription.status === "expired"
  const isCancelling = subscription.subscription?.cancel_at_period_end

  const getStatusBadge = () => {
    if (isExpired) {
      return <Badge variant="destructive">Expired</Badge>
    }
    if (isCancelling) {
      return <Badge variant="secondary">Cancelling</Badge>
    }
    if (subscription.subscription?.status === "active") {
      return <Badge variant="default">Active</Badge>
    }
    if (subscription.subscription?.status === "past_due") {
      return <Badge variant="destructive">Past Due</Badge>
    }
    return (
      <Badge variant="outline">
        {subscription.subscription?.status || "Unknown"}
      </Badge>
    )
  }

  const handleCancel = async () => {
    await cancelMutation.mutateAsync(subscription.server_id)
    setShowCancelConfirm(false)
  }

  const handleResubscribe = () => {
    resubscribeMutation.mutate(subscription.server_id)
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <CardTitle className="text-lg">{subscription.display_name}</CardTitle>
            {getStatusBadge()}
          </div>
          <span className="text-sm text-muted-foreground">
            {game?.name || subscription.game} - {plan?.name || subscription.plan}
          </span>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Subscription Details */}
        {subscription.subscription && !isExpired && (
          <div className="space-y-3">
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Calendar className="h-4 w-4" />
              <span>
                {isCancelling ? "Ends on" : "Renews on"}:{" "}
                <span className="text-foreground">
                  {formatDate(subscription.subscription.current_period_end)}
                </span>
              </span>
            </div>
            {isCancelling && (
              <div className="space-y-3">
                <Alert className="bg-yellow-500/10 border-yellow-500/20">
                  <AlertCircle className="h-4 w-4 text-yellow-500" />
                  <AlertDescription className="text-sm">
                    Your subscription will be cancelled on{" "}
                    {formatDate(subscription.subscription.current_period_end)}. Your server
                    will continue running until then.
                  </AlertDescription>
                </Alert>
                <Button
                  variant="default"
                  size="sm"
                  onClick={() => resumeMutation.mutate(subscription.server_id)}
                  disabled={resumeMutation.isPending}
                >
                  {resumeMutation.isPending ? "Resuming..." : "Resume Subscription"}
                </Button>
              </div>
            )}
          </div>
        )}

        {/* Expired Server Info */}
        {isExpired && (
          <div className="space-y-3">
            <Alert variant="destructive" className="bg-destructive/10">
              <AlertCircle className="h-4 w-4" />
              <AlertDescription className="text-sm">
                This server's subscription has expired.
                {subscription.delete_after && (
                  <>
                    {" "}
                    Server data will be deleted on{" "}
                    {formatDate(subscription.delete_after)}.
                  </>
                )}
              </AlertDescription>
            </Alert>
            <Button
              onClick={handleResubscribe}
              disabled={resubscribeMutation.isPending}
              className="w-full"
            >
              {resubscribeMutation.isPending ? (
                <>
                  <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                  Processing...
                </>
              ) : (
                <>
                  <CreditCard className="mr-2 h-4 w-4" />
                  Resubscribe
                </>
              )}
            </Button>
          </div>
        )}

        {/* Cancel Subscription */}
        {subscription.subscription && !isExpired && !isCancelling && (
          <div className="pt-2 border-t">
            {!showCancelConfirm ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setShowCancelConfirm(true)}
              >
                Cancel Subscription
              </Button>
            ) : (
              <div className="space-y-3">
                <p className="text-sm text-muted-foreground">
                  Are you sure? Your server will keep running until the end of
                  your billing period.
                </p>
                <div className="flex gap-2">
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={handleCancel}
                    disabled={cancelMutation.isPending}
                  >
                    {cancelMutation.isPending ? "Cancelling..." : "Yes, Cancel"}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setShowCancelConfirm(false)}
                    disabled={cancelMutation.isPending}
                  >
                    Keep Subscription
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
