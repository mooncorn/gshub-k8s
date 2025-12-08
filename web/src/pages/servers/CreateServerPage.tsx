import { useState, useMemo, useEffect } from "react"
import { Link, useSearchParams, useNavigate, useLocation } from "react-router-dom"
import { Search, ChevronDown, Cpu, MemoryStick, Users } from "lucide-react"
import { serversApi, type GameType, type ServerPlan } from "@/api/servers"
import { useAuth } from "@/hooks/useAuth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent } from "@/components/ui/card"
import { GAMES, GAME_ICONS, PLANS } from "@/lib/constants"
import { cn } from "@/lib/utils"

const gameList = Object.values(GAMES)

const SESSION_KEY = "pendingServerCreation"

interface PendingServer {
  game: GameType
  plan: ServerPlan
  displayName: string
}

function generateSubdomain(displayName: string): string {
  const slug = displayName
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, "")
    .replace(/\s+/g, "-")
    .substring(0, 30)
  const suffix = Math.random().toString(36).substring(2, 6)
  return `${slug}-${suffix}`
}

const PLAN_COLORS: Record<ServerPlan, { border: string; bg: string; badge: string }> = {
  small: {
    border: "border-amber-700/50 hover:border-amber-600",
    bg: "bg-gradient-to-br from-amber-950/30 to-amber-900/10",
    badge: "bg-amber-700/30 text-amber-300",
  },
  medium: {
    border: "border-slate-400/50 hover:border-slate-300",
    bg: "bg-gradient-to-br from-slate-800/30 to-slate-700/10",
    badge: "bg-slate-500/30 text-slate-200",
  },
  large: {
    border: "border-yellow-500/50 hover:border-yellow-400",
    bg: "bg-gradient-to-br from-yellow-900/30 to-yellow-800/10",
    badge: "bg-yellow-600/30 text-yellow-200",
  },
}

export function CreateServerPage() {
  const { isAuthenticated } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [searchParams] = useSearchParams()
  const [selectedGame, setSelectedGame] = useState<GameType | null>(null)
  const [selectedPlan, setSelectedPlan] = useState<ServerPlan | null>(null)
  const [displayName, setDisplayName] = useState("")
  const [error, setError] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [isDropdownOpen, setIsDropdownOpen] = useState(false)
  const [search, setSearch] = useState("")

  // Restore from session storage or URL params on mount
  useEffect(() => {
    const pending = sessionStorage.getItem(SESSION_KEY)
    if (pending) {
      try {
        const data = JSON.parse(pending) as PendingServer
        if (data.game && data.game in GAMES) {
          setSelectedGame(data.game)
        }
        if (data.plan) {
          setSelectedPlan(data.plan)
        }
        if (data.displayName) {
          setDisplayName(data.displayName)
        }
        sessionStorage.removeItem(SESSION_KEY)
      } catch {
        sessionStorage.removeItem(SESSION_KEY)
      }
    } else {
      // Fall back to URL params if no session data
      const gameParam = searchParams.get("game") as GameType | null
      if (gameParam && gameParam in GAMES) {
        setSelectedGame(gameParam)
      }
    }
  }, [searchParams])

  const filteredGames = useMemo(() => {
    if (!search) return gameList
    return gameList.filter((game) =>
      game.name.toLowerCase().includes(search.toLowerCase())
    )
  }, [search])

  const selectedGameData = selectedGame ? GAMES[selectedGame] : null
  const availablePlans = selectedGameData?.plans || []

  const handleGameSelect = (gameId: GameType) => {
    setSelectedGame(gameId)
    setSelectedPlan(null)
    setIsDropdownOpen(false)
    setSearch("")
  }

  const defaultDisplayName = selectedGame
    ? `My ${GAMES[selectedGame].name} Server`
    : ""

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedGame || !selectedPlan) return

    const finalDisplayName = displayName.trim() || defaultDisplayName
    if (!finalDisplayName) return

    setError("")
    setIsLoading(true)

    try {
      const subdomain = generateSubdomain(finalDisplayName)
      const response = await serversApi.checkout(
        finalDisplayName,
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

  const isFormValid = selectedGame && selectedPlan

  const saveAndRedirectToAuth = (path: "/login" | "/register") => {
    if (selectedGame && selectedPlan) {
      const pending: PendingServer = {
        game: selectedGame,
        plan: selectedPlan,
        displayName: displayName.trim() || defaultDisplayName,
      }
      sessionStorage.setItem(SESSION_KEY, JSON.stringify(pending))
    }
    navigate(path, { state: { from: location } })
  }

  return (
    <div className="mx-auto max-w-3xl space-y-8 py-6">
      <div className="flex items-center gap-4">
        <Link to={isAuthenticated ? "/" : "/welcome"}>
          <Button variant="ghost" size="sm">
            ← Back
          </Button>
        </Link>
      </div>

      <div>
        <h1 className="text-2xl font-bold">Create Server</h1>
        <p className="text-muted-foreground">
          Choose your game, select a plan, and start playing
        </p>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {/* Game Selection Dropdown */}
      <div className="space-y-2">
        <Label className="text-sm font-medium">Game</Label>
        <div className="relative">
          <button
            type="button"
            onClick={() => setIsDropdownOpen(!isDropdownOpen)}
            className="flex w-full cursor-pointer items-center justify-between gap-3 rounded-lg border border-border bg-card px-4 py-3 text-left transition-colors hover:bg-accent/50"
          >
            {selectedGameData ? (
              <div className="flex items-center gap-3">
                <img
                  src={GAME_ICONS[selectedGame!]}
                  alt={selectedGameData.name}
                  className="h-8 w-8 rounded"
                />
                <span className="font-medium">{selectedGameData.name}</span>
              </div>
            ) : (
              <span className="text-muted-foreground">Select a game...</span>
            )}
            <ChevronDown
              className={cn(
                "h-5 w-5 text-muted-foreground transition-transform",
                isDropdownOpen && "rotate-180"
              )}
            />
          </button>

          {isDropdownOpen && (
            <>
              <div
                className="fixed inset-0 z-40"
                onClick={() => setIsDropdownOpen(false)}
              />
              <div className="absolute left-0 right-0 top-full z-50 mt-2 overflow-hidden rounded-lg border border-border bg-card shadow-xl">
                <div className="border-b border-border p-2">
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <input
                      type="text"
                      placeholder="Search games..."
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                      className="w-full rounded-md bg-muted/50 py-2 pl-9 pr-3 text-sm outline-none placeholder:text-muted-foreground focus:bg-muted"
                      autoFocus
                    />
                  </div>
                </div>

                <div className="max-h-64 overflow-auto p-2">
                  {filteredGames.length === 0 ? (
                    <p className="px-3 py-4 text-center text-sm text-muted-foreground">
                      No games found
                    </p>
                  ) : (
                    filteredGames.map((game) => (
                      <button
                        key={game.id}
                        type="button"
                        onClick={() => handleGameSelect(game.id)}
                        className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-left transition-colors hover:bg-accent"
                      >
                        <img
                          src={GAME_ICONS[game.id]}
                          alt={game.name}
                          className="h-8 w-8 rounded"
                        />
                        <span className="font-medium">{game.name}</span>
                      </button>
                    ))
                  )}
                </div>
              </div>
            </>
          )}
        </div>
      </div>

      {/* Plan Selection */}
      <div className="space-y-3">
        <Label className="text-sm font-medium">Plan</Label>
        {!selectedGame ? (
          <p className="text-sm text-muted-foreground">
            Select a game first to see available plans
          </p>
        ) : (
          <div className="grid gap-4 sm:grid-cols-3">
            {availablePlans.map((planId) => {
              const plan = PLANS[planId]
              const colors = PLAN_COLORS[planId]
              const isSelected = selectedPlan === planId

              return (
                <Card
                  key={planId}
                  className={cn(
                    "cursor-pointer transition-all",
                    colors.border,
                    colors.bg,
                    isSelected && "ring-2 ring-primary ring-offset-2 ring-offset-background"
                  )}
                  onClick={() => setSelectedPlan(planId)}
                >
                  <CardContent className="p-4 space-y-3">
                    <div className="flex items-center justify-between">
                      <h3 className="text-lg font-semibold">{plan.name}</h3>
                      <span className={cn("text-xs px-2 py-0.5 rounded-full font-medium", colors.badge)}>
                        {planId.toUpperCase()}
                      </span>
                    </div>
                    <div className="text-2xl font-bold">{plan.price}</div>
                    <div className="pt-2 space-y-1.5 border-t border-border/50 text-sm text-muted-foreground">
                      <div className="flex items-center gap-2">
                        <Users className="h-4 w-4" />
                        <span>{plan.players} players</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <Cpu className="h-4 w-4" />
                        <span>{plan.cpu}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <MemoryStick className="h-4 w-4" />
                        <span>{plan.memory}</span>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              )
            })}
          </div>
        )}
      </div>

      {/* Display Name */}
      <div className="space-y-2">
        <Label htmlFor="displayName" className="text-sm font-medium">
          Server Name
        </Label>
        <Input
          id="displayName"
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
          placeholder={defaultDisplayName || "My Awesome Server"}
          maxLength={50}
        />
        <p className="text-xs text-muted-foreground">
          Leave empty to use the default name
        </p>
      </div>

      {/* Action Buttons */}
      {isAuthenticated ? (
        <Button
          onClick={handleSubmit}
          size="lg"
          className="w-full"
          disabled={!isFormValid || isLoading}
        >
          {isLoading ? "Processing..." : "Continue to Checkout"}
        </Button>
      ) : (
        <div className="space-y-3">
          <div className="flex gap-3">
            <Button
              onClick={() => saveAndRedirectToAuth("/login")}
              size="lg"
              variant="outline"
              className="flex-1"
              disabled={!isFormValid}
            >
              Sign In
            </Button>
            <Button
              onClick={() => saveAndRedirectToAuth("/register")}
              size="lg"
              className="flex-1"
              disabled={!isFormValid}
            >
              Sign Up
            </Button>
          </div>
          <p className="text-center text-sm text-muted-foreground">
            Create an account or sign in to continue
          </p>
        </div>
      )}
    </div>
  )
}
