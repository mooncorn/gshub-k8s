import { Card, CardContent } from "@/components/ui/card"
import { GAMES, PLANS } from "@/lib/constants"
import type { GameType, ServerPlan } from "@/api/servers"
import { cn } from "@/lib/utils"

interface PlanSelectorProps {
  game: GameType
  selected: ServerPlan | null
  onSelect: (plan: ServerPlan) => void
}

export function PlanSelector({ game, selected, onSelect }: PlanSelectorProps) {
  const gameConfig = GAMES[game]
  const availablePlans = gameConfig.plans

  return (
    <div className="grid gap-4 sm:grid-cols-3">
      {availablePlans.map((planId) => {
        const plan = PLANS[planId]
        return (
          <Card
            key={planId}
            className={cn(
              "cursor-pointer transition-colors hover:bg-accent/50",
              selected === planId && "border-primary bg-accent/50"
            )}
            onClick={() => onSelect(planId)}
          >
            <CardContent className="p-4">
              <h3 className="font-medium">{plan.name}</h3>
              <p className="text-sm text-muted-foreground">
                {plan.players} players
              </p>
            </CardContent>
          </Card>
        )
      })}
    </div>
  )
}
