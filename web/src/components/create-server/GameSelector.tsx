import { Card, CardContent } from "@/components/ui/card"
import { GAMES } from "@/lib/constants"
import type { GameType } from "@/api/servers"
import { cn } from "@/lib/utils"

interface GameSelectorProps {
  selected: GameType | null
  onSelect: (game: GameType) => void
}

export function GameSelector({ selected, onSelect }: GameSelectorProps) {
  const games = Object.values(GAMES)

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      {games.map((game) => (
        <Card
          key={game.id}
          className={cn(
            "cursor-pointer transition-colors hover:bg-accent/50",
            selected === game.id && "border-primary bg-accent/50"
          )}
          onClick={() => onSelect(game.id)}
        >
          <CardContent className="p-4">
            <h3 className="font-medium">{game.name}</h3>
            <p className="text-sm text-muted-foreground">{game.description}</p>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
