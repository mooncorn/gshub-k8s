import { useState } from "react"
import { Link } from "react-router-dom"
import { serversApi, type GameType, type ServerPlan } from "@/api/servers"
import { GameSelector } from "@/components/create-server/GameSelector"
import { PlanSelector } from "@/components/create-server/PlanSelector"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { GAMES } from "@/lib/constants"

type Step = "game" | "plan" | "name"

function generateSubdomain(displayName: string): string {
  const slug = displayName
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, "")
    .replace(/\s+/g, "-")
    .substring(0, 30)
  const suffix = Math.random().toString(36).substring(2, 6)
  return `${slug}-${suffix}`
}

export function CreateServerPage() {
  const [step, setStep] = useState<Step>("game")
  const [selectedGame, setSelectedGame] = useState<GameType | null>(null)
  const [selectedPlan, setSelectedPlan] = useState<ServerPlan | null>(null)
  const [displayName, setDisplayName] = useState("")
  const [error, setError] = useState("")
  const [isLoading, setIsLoading] = useState(false)

  const handleGameSelect = (game: GameType) => {
    setSelectedGame(game)
    setSelectedPlan(null)
    setStep("plan")
  }

  const handlePlanSelect = (plan: ServerPlan) => {
    setSelectedPlan(plan)
    setStep("name")
    if (!displayName && selectedGame) {
      setDisplayName(`My ${GAMES[selectedGame].name} Server`)
    }
  }

  const handleBack = () => {
    if (step === "plan") {
      setStep("game")
    } else if (step === "name") {
      setStep("plan")
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedGame || !selectedPlan || !displayName.trim()) return

    setError("")
    setIsLoading(true)

    try {
      const subdomain = generateSubdomain(displayName)
      const response = await serversApi.checkout(
        displayName.trim(),
        subdomain,
        selectedGame,
        selectedPlan
      )
      window.location.href = response.data.checkout_url
    } catch (err) {
      if (err instanceof Error && "response" in err) {
        const axiosError = err as { response?: { data?: { error?: string } } }
        setError(axiosError.response?.data?.error || "Failed to create server")
      } else {
        setError("Failed to create server. Please try again.")
      }
      setIsLoading(false)
    }
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/">
          <Button variant="ghost" size="sm">
            ← Back
          </Button>
        </Link>
      </div>

      <div>
        <h1 className="text-xl font-semibold">Create Server</h1>
        <p className="text-sm text-muted-foreground">
          {step === "game" && "Select a game"}
          {step === "plan" && "Select a plan"}
          {step === "name" && "Name your server"}
        </p>
      </div>

      {step === "game" && (
        <GameSelector selected={selectedGame} onSelect={handleGameSelect} />
      )}

      {step === "plan" && selectedGame && (
        <div className="space-y-4">
          <PlanSelector
            game={selectedGame}
            selected={selectedPlan}
            onSelect={handlePlanSelect}
          />
          <Button variant="ghost" size="sm" onClick={handleBack}>
            ← Back to game selection
          </Button>
        </div>
      )}

      {step === "name" && selectedGame && selectedPlan && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Server Details</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              {error && (
                <div className="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                  {error}
                </div>
              )}

              <div className="rounded-lg bg-muted/50 p-3 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Game</span>
                  <span>{GAMES[selectedGame].name}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Plan</span>
                  <span className="capitalize">{selectedPlan}</span>
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="displayName">Server Name</Label>
                <Input
                  id="displayName"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder="My Awesome Server"
                  required
                  minLength={3}
                  maxLength={50}
                />
              </div>

              <div className="flex gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={handleBack}
                >
                  ← Back
                </Button>
                <Button
                  type="submit"
                  className="flex-1"
                  disabled={isLoading || !displayName.trim()}
                >
                  {isLoading ? "Creating..." : "Continue to Payment"}
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
