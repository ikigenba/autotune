package config

import "github.com/ikigenba/agentkit"

// ProviderFactory is the model-provider construction seam supplied by the
// composition root. It returns a fresh conversation because conversations
// retain message history and cannot be shared between independent calls.
type ProviderFactory func(section Section, system string) (*agentkit.Conversation, error)
