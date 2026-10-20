import type { ModelItem } from "@/lib/api"

const NON_EFFORT_MODELS = [
  "deepseek-chat",
  "deepseek-r1",
  "minimax",
  "kimi",
  "qwen",
  "big-pickle",
  "nemotron",
  "mimo",
]

function buildVariants(
  reasoningOptionsJSON?: string,
  reasoning?: boolean,
  modelID: string = ""
): Record<string, { reasoningEffort: string }> | undefined {
  if (!reasoning) {
    return undefined
  }

  let effortValues: string[] = []
  if (reasoningOptionsJSON && reasoningOptionsJSON !== "[]") {
    try {
      const options = JSON.parse(reasoningOptionsJSON)
      if (Array.isArray(options)) {
        for (const opt of options) {
          if (opt.type === "effort" && Array.isArray(opt.values) && opt.values.length > 0) {
            effortValues = opt.values
            break
          }
        }
      }
    } catch {
      // Ignore JSON parsing errors and fallback
    }
  }

  if (effortValues.length === 0) {
    const idLower = modelID.toLowerCase()
    const isNonEffort = NON_EFFORT_MODELS.some((ne) => idLower.includes(ne))
    if (!isNonEffort) {
      effortValues = ["low", "medium", "high"]
    }
  }

  if (effortValues.length === 0) {
    return undefined
  }

  const variants: Record<string, { reasoningEffort: string }> = {}
  for (const v of effortValues) {
    variants[v] = { reasoningEffort: v }
  }
  return variants
}

export function generateOpenCodeSnippet(models: ModelItem[]): string {
  if (!models || models.length === 0) {
    return ""
  }

  const modelsDict: Record<string, any> = {}

  for (const mdl of models) {
    const mEntry: Record<string, any> = {
      name: mdl.name,
    }

    if (mdl.limit && mdl.limit !== "{}") {
      try {
        const limitMap = JSON.parse(mdl.limit)
        if (limitMap && typeof limitMap === "object" && Object.keys(limitMap).length > 0) {
          mEntry.limit = limitMap
        }
      } catch {
        // Ignore limit parsing error
      }
    }

    const inputModalities = mdl.modalities
      ? mdl.modalities
          .split(",")
          .map((part) => part.trim())
          .filter(Boolean)
      : ["text"]

    mEntry.modalities = {
      input: inputModalities.length > 0 ? inputModalities : ["text"],
      output: ["text"],
    }

    if (mdl.reasoning) {
      mEntry.reasoning = true
    }
    if (mdl.tool_call) {
      mEntry.tool_call = true
    }
    if (mdl.temperature) {
      mEntry.temperature = true
    }
    if (mdl.attachment) {
      mEntry.attachment = true
    }

    const variants = buildVariants(mdl.reasoning_options, mdl.reasoning, mdl.id)
    if (variants && Object.keys(variants).length > 0) {
      mEntry.variants = variants
    }

    if (mdl.provider && mdl.provider !== "{}") {
      try {
        const provMap = JSON.parse(mdl.provider)
        if (provMap && typeof provMap === "object" && Object.keys(provMap).length > 0) {
          mEntry.provider = provMap
        }
      } catch {
        // Ignore provider parsing error
      }
    } else if (mdl.id.toLowerCase().includes("muse")) {
      mEntry.provider = {
        npm: "@ai-sdk/openai",
      }
    }

    modelsDict[mdl.id] = mEntry
  }

  let baseURL = "http://localhost:9100/v1"
  if (typeof window !== "undefined" && window.location.origin) {
    if (window.location.port === "5173") {
      baseURL = "http://localhost:9100/v1"
    } else {
      baseURL = `${window.location.origin}/v1`
    }
  }

  const configSnippet = {
    provider: {
      aip: {
        npm: "@ai-sdk/openai-compatible",
        options: {
          baseURL,
          apiKey: "public",
        },
        models: modelsDict,
      },
    },
  }

  return JSON.stringify(configSnippet, null, 2)
}
