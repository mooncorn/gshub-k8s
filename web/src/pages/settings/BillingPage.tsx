import { Link } from "react-router-dom"
import { useBilling } from "@/hooks/useBilling"
import { SubscriptionCard } from "@/components/billing/SubscriptionCard"
import { Card, CardContent } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Button } from "@/components/ui/button"

export function BillingPage() {
  const { data: subscriptions, isLoading, error } = useBilling()

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Billing & Subscriptions</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Manage your server subscriptions and billing information
        </p>
      </div>

      {isLoading && (
        <div className="space-y-4">
          {[1, 2].map((i) => (
            <Card key={i}>
              <CardContent className="p-6">
                <div className="space-y-3">
                  <Skeleton className="h-5 w-48" />
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-4 w-64" />
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {error && (
        <Card>
          <CardContent className="p-6 text-center">
            <p className="text-sm text-muted-foreground">
              Failed to load billing information. Please try again.
            </p>
          </CardContent>
        </Card>
      )}

      {!isLoading && !error && subscriptions?.length === 0 && (
        <Card>
          <CardContent className="p-6 text-center">
            <p className="text-sm text-muted-foreground">
              No subscriptions yet. Create a server to get started.
            </p>
            <Link to="/servers/new">
              <Button className="mt-4" size="sm">
                Create Server
              </Button>
            </Link>
          </CardContent>
        </Card>
      )}

      {!isLoading && !error && subscriptions && subscriptions.length > 0 && (
        <div className="space-y-4">
          {subscriptions.map((sub) => (
            <SubscriptionCard key={sub.server_id} subscription={sub} />
          ))}
        </div>
      )}
    </div>
  )
}
