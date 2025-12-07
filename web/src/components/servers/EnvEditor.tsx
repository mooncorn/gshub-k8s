import { useState, useEffect } from "react"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { GAME_ENV_VARS, type EnvVarDefinition } from "@/lib/constants"
import type { GameType, GameConfigInfo } from "@/api/servers"

interface EnvEditorProps {
  game: GameType
  gameConfig?: GameConfigInfo
  onSave: (envOverrides: Record<string, string>) => Promise<void>
  disabled?: boolean
}

export function EnvEditor({
  game,
  gameConfig,
  onSave,
  disabled,
}: EnvEditorProps) {
  const [envValues, setEnvValues] = useState<Record<string, string>>({})
  const [customVars, setCustomVars] = useState<{ key: string; value: string }[]>(
    []
  )
  const [isSaving, setIsSaving] = useState(false)
  const [hasChanges, setHasChanges] = useState(false)

  const gameEnvDefs = GAME_ENV_VARS[game] || []
  const defaultEnv = gameConfig?.default_env || {}
  const effectiveEnv = gameConfig?.effective_env || defaultEnv

  // Initialize values from effective env (overrides or defaults)
  useEffect(() => {
    const initial: Record<string, string> = {}

    for (const def of gameEnvDefs) {
      initial[def.key] = effectiveEnv[def.key] ?? def.default ?? ""
    }

    setEnvValues(initial)

    // Extract any custom vars not in predefined list
    const predefinedKeys = new Set(gameEnvDefs.map((d) => d.key))
    const custom = Object.entries(effectiveEnv)
      .filter(([key]) => !predefinedKeys.has(key))
      .map(([key, value]) => ({ key, value }))
    setCustomVars(custom)

    setHasChanges(false)
  }, [game, effectiveEnv, gameEnvDefs])

  const handleValueChange = (key: string, value: string) => {
    setEnvValues((prev) => ({ ...prev, [key]: value }))
    setHasChanges(true)
  }

  const handleAddCustomVar = () => {
    setCustomVars((prev) => [...prev, { key: "", value: "" }])
    setHasChanges(true)
  }

  const handleRemoveCustomVar = (index: number) => {
    setCustomVars((prev) => prev.filter((_, i) => i !== index))
    setHasChanges(true)
  }

  const handleCustomVarChange = (
    index: number,
    field: "key" | "value",
    value: string
  ) => {
    setCustomVars((prev) =>
      prev.map((v, i) => (i === index ? { ...v, [field]: value } : v))
    )
    setHasChanges(true)
  }

  const handleSave = async () => {
    setIsSaving(true)
    try {
      // Build final env overrides
      const overrides: Record<string, string> = {}

      // Add predefined vars (skip empty values for optional fields)
      for (const [key, value] of Object.entries(envValues)) {
        if (value !== "") {
          overrides[key] = value
        }
      }

      // Add custom vars (skip empty keys)
      for (const { key, value } of customVars) {
        if (key.trim()) {
          overrides[key.trim()] = value
        }
      }

      await onSave(overrides)
      setHasChanges(false)
    } finally {
      setIsSaving(false)
    }
  }

  const handleResetToDefaults = () => {
    const initial: Record<string, string> = {}
    for (const def of gameEnvDefs) {
      initial[def.key] = defaultEnv[def.key] ?? def.default ?? ""
    }
    setEnvValues(initial)
    setCustomVars([])
    setHasChanges(true)
  }

  const renderField = (def: EnvVarDefinition) => {
    const value = envValues[def.key] || ""

    if (def.type === "boolean") {
      const isTrue =
        value.toLowerCase() === "true" || value.toLowerCase() === "1"
      return (
        <button
          type="button"
          onClick={() =>
            handleValueChange(def.key, isTrue ? "FALSE" : "TRUE")
          }
          disabled={disabled}
          className={`relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 ${
            isTrue ? "bg-primary" : "bg-muted"
          } ${disabled ? "opacity-50 cursor-not-allowed" : ""}`}
        >
          <span
            className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
              isTrue ? "translate-x-5" : "translate-x-0"
            }`}
          />
        </button>
      )
    }

    if (def.type === "select" && def.options) {
      return (
        <select
          value={value}
          onChange={(e) => handleValueChange(def.key, e.target.value)}
          disabled={disabled}
          className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <option value="">Select...</option>
          {def.options.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      )
    }

    // Default to text input
    const placeholder = effectiveEnv[def.key] || def.default || `Enter ${def.label.toLowerCase()}`
    return (
      <Input
        value={value}
        onChange={(e) => handleValueChange(def.key, e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
      />
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">
          Environment Variables
        </CardTitle>
        <CardDescription>
          Configure server settings. Changes apply on next restart.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Predefined game-specific variables */}
        {gameEnvDefs.map((def) => (
          <div key={def.key} className="space-y-1">
            <Label htmlFor={def.key}>{def.label}</Label>
            {renderField(def)}
            <p className="text-xs text-muted-foreground">{def.description}</p>
          </div>
        ))}

        {/* Custom variables section */}
        <div className="space-y-2 pt-4 border-t">
          <div className="flex items-center justify-between">
            <Label>Custom Variables</Label>
            <Button
              variant="outline"
              size="sm"
              onClick={handleAddCustomVar}
              disabled={disabled}
            >
              + Add Variable
            </Button>
          </div>
          {customVars.map((cv, index) => (
            <div key={index} className="flex gap-2">
              <Input
                placeholder="KEY"
                value={cv.key}
                onChange={(e) =>
                  handleCustomVarChange(index, "key", e.target.value)
                }
                disabled={disabled}
                className="font-mono"
              />
              <Input
                placeholder="value"
                value={cv.value}
                onChange={(e) =>
                  handleCustomVarChange(index, "value", e.target.value)
                }
                disabled={disabled}
              />
              <Button
                variant="ghost"
                size="sm"
                onClick={() => handleRemoveCustomVar(index)}
                disabled={disabled}
              >
                X
              </Button>
            </div>
          ))}
        </div>

        {hasChanges && (
          <Alert>
            <AlertDescription>
              You have unsaved changes. Click Save to apply them on next server
              restart.
            </AlertDescription>
          </Alert>
        )}

        <div className="flex gap-2 pt-2">
          <Button
            onClick={handleSave}
            disabled={!hasChanges || isSaving || disabled}
          >
            {isSaving ? "Saving..." : "Save Changes"}
          </Button>
          <Button
            variant="outline"
            onClick={handleResetToDefaults}
            disabled={disabled}
          >
            Reset to Defaults
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
