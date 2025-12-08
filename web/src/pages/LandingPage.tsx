import { useState, useMemo } from "react"
import { Link, useNavigate } from "react-router-dom"
import { Search, ChevronDown, Gamepad2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { GAMES, GAME_ICONS } from "@/lib/constants"
import type { GameType } from "@/api/servers"

const gameList = Object.values(GAMES)

// Generate ash particles with random properties
const ashParticles = Array.from({ length: 30 }, (_, i) => ({
  id: i,
  left: Math.random() * 100,
  delay: Math.random() * 8,
  duration: 8 + Math.random() * 6,
  size: 2 + Math.random() * 3,
  drift: -20 + Math.random() * 40,
}))

export function LandingPage() {
  const navigate = useNavigate()
  const [isOpen, setIsOpen] = useState(false)
  const [search, setSearch] = useState("")
  const [selectedGame, setSelectedGame] = useState<GameType | null>(null)

  const filteredGames = useMemo(() => {
    if (!search) return gameList
    return gameList.filter((game) =>
      game.name.toLowerCase().includes(search.toLowerCase())
    )
  }, [search])

  const selectedGameData = selectedGame ? GAMES[selectedGame] : null

  const handleSelect = (gameId: GameType) => {
    setSelectedGame(gameId)
    setIsOpen(false)
    setSearch("")
  }

  const handleGetStarted = () => {
    if (selectedGame) {
      navigate(`/servers/new?game=${selectedGame}`)
    } else {
      navigate("/servers/new")
    }
  }

  return (
    <div className="relative min-h-screen overflow-hidden bg-background">
      {/* Hero background image */}
      <div className="absolute inset-0 pointer-events-none">
        <div
          className="absolute inset-0 bg-cover bg-center opacity-55"
          style={{ backgroundImage: "url(/img/enshrouded_hero.png)" }}
        />
        <div className="absolute inset-0 bg-gradient-to-b from-transparent via-background/50 to-background" />
      </div>

      {/* Ash particles */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        {ashParticles.map((particle) => (
          <div
            key={particle.id}
            className="absolute rounded-full bg-orange-300/60"
            style={{
              left: `${particle.left}%`,
              bottom: "-10px",
              width: `${particle.size}px`,
              height: `${particle.size}px`,
              animation: `float-ash ${particle.duration}s ease-out ${particle.delay}s infinite`,
              ["--drift" as string]: `${particle.drift}px`,
            }}
          />
        ))}
      </div>

      <style>{`
        @keyframes float-ash {
          0% {
            transform: translateY(0) translateX(0);
            opacity: 0.7;
          }
          50% {
            opacity: 0.5;
          }
          100% {
            transform: translateY(-100vh) translateX(var(--drift));
            opacity: 0;
          }
        }
      `}</style>

      {/* Header */}
      <header className="relative z-20 border-b border-border/30 bg-background/50 backdrop-blur-sm">
        <div className="container mx-auto flex h-14 items-center justify-between px-4">
          <Link to="/" className="flex items-center gap-2 text-lg font-semibold text-foreground">
            <Gamepad2 className="h-6 w-6" />
            GSHUB
          </Link>
          <div className="flex items-center gap-3">
            <Link to="/login">
              <Button variant="ghost" size="sm">
                Sign in
              </Button>
            </Link>
            <Link to="/register">
              <Button size="sm">Get Started</Button>
            </Link>
          </div>
        </div>
      </header>

      {/* Hero content */}
      <main className="relative z-10 flex min-h-[calc(100vh-3.5rem)] flex-col items-center justify-center px-4 py-20">
        <div className="max-w-3xl text-center">
          {/* Catchphrase */}
          <h1 className="mb-4 text-5xl font-bold tracking-tight text-foreground sm:text-6xl lg:text-7xl">
            Game Servers
            <span className="block bg-gradient-to-r from-primary to-primary/60 bg-clip-text text-transparent">
              Made Simple
            </span>
          </h1>
          <p className="mx-auto mb-12 max-w-xl text-lg text-muted-foreground sm:text-xl">
            Deploy your game server in seconds. No technical knowledge required.
            Built by gamers, for gamers.
          </p>

          {/* Game selector */}
          <div className="mx-auto flex items-center max-w-md flex-col gap-4 sm:flex-row">
            <div className="relative flex-1">
              <button
                onClick={() => setIsOpen(!isOpen)}
                className="flex w-full cursor-pointer items-center justify-between gap-3 rounded-lg border border-border bg-card/80 px-4 py-3 text-left backdrop-blur-sm transition-colors hover:bg-card"
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
                  className={`h-5 w-5 text-muted-foreground transition-transform ${isOpen ? "rotate-180" : ""}`}
                />
              </button>

              {/* Dropdown */}
              {isOpen && (
                <>
                  {/* Click outside to close */}
                  <div
                    className="fixed inset-0 z-40"
                    onClick={() => setIsOpen(false)}
                  />
                  <div className="absolute left-0 right-0 top-full z-50 mt-2 overflow-hidden rounded-lg border border-border bg-card shadow-xl">
                    {/* Search input */}
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

                    {/* Game list */}
                    <div className="max-h-64 overflow-auto p-2">
                      {filteredGames.length === 0 ? (
                        <p className="px-3 py-4 text-center text-sm text-muted-foreground">
                          No games found
                        </p>
                      ) : (
                        filteredGames.map((game) => (
                          <button
                            key={game.id}
                            onClick={() => handleSelect(game.id)}
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

            <Button
              size={"lg"}
              onClick={handleGetStarted}
              className="shrink-0 px-8 py-3"
            >
              Create Server
            </Button>
          </div>

          {/* Features hint */}
          <div className="mt-16 flex flex-wrap justify-center gap-8 text-sm text-muted-foreground">
            <div className="flex items-center gap-2">
              <div className="h-2 w-2 rounded-full bg-green-500" />
              Instant deployment
            </div>
            <div className="flex items-center gap-2">
              <div className="h-2 w-2 rounded-full bg-blue-500" />
              One-click start/stop
            </div>
            <div className="flex items-center gap-2">
              <div className="h-2 w-2 rounded-full bg-purple-500" />
              Real-time console
            </div>
          </div>
        </div>
      </main>
    </div>
  )
}
