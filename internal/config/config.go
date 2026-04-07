package config

import (
	"AIChatMatrix/internal/models"
	"encoding/json"
	"os"
	"sync"
)

// ModelProvider represents an AI model provider configuration
type ModelProvider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
}

// AppConfig holds the application configuration
type AppConfig struct {
	mu        sync.RWMutex
	Port      string                   `json:"port"`
	Providers []ModelProvider          `json:"providers"`
	Personas  []models.AIPersona       `json:"personas"`
	Folders   []models.PersonaFolder   `json:"folders"`
	Templates []models.RoomTemplate    `json:"templates"`
}

var (
	instance *AppConfig
	once     sync.Once
)

func Get() *AppConfig {
	once.Do(func() {
		instance = loadConfig()
	})
	return instance
}

func loadConfig() *AppConfig {
	cfg := &AppConfig{
		Port:      "8080",
		Providers: []ModelProvider{},
		Personas:  []models.AIPersona{},
		Folders:   []models.PersonaFolder{},
		Templates: []models.RoomTemplate{},
	}
	data, err := os.ReadFile("config.json")
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, cfg)
	if cfg.Personas == nil {
		cfg.Personas = []models.AIPersona{}
	}
	if cfg.Folders == nil {
		cfg.Folders = []models.PersonaFolder{}
	}
	if cfg.Templates == nil {
		cfg.Templates = []models.RoomTemplate{}
	}
	return cfg
}

func (c *AppConfig) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("config.json", data, 0644)
}

func (c *AppConfig) GetProvider(id string) (ModelProvider, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, p := range c.Providers {
		if p.ID == id {
			return p, true
		}
	}
	return ModelProvider{}, false
}

func (c *AppConfig) AddOrUpdateProvider(p ModelProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, existing := range c.Providers {
		if existing.ID == p.ID {
			c.Providers[i] = p
			return
		}
	}
	c.Providers = append(c.Providers, p)
}

func (c *AppConfig) DeleteProvider(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, p := range c.Providers {
		if p.ID == id {
			c.Providers = append(c.Providers[:i], c.Providers[i+1:]...)
			return
		}
	}
}

func (c *AppConfig) GetAllProviders() []ModelProvider {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]ModelProvider, len(c.Providers))
	copy(out, c.Providers)
	return out
}

// ---- Persona methods ----

func (c *AppConfig) GetAllPersonas() []models.AIPersona {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]models.AIPersona, len(c.Personas))
	copy(out, c.Personas)
	return out
}

func (c *AppConfig) GetPersona(id string) (models.AIPersona, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, p := range c.Personas {
		if p.ID == id {
			return p, true
		}
	}
	return models.AIPersona{}, false
}

func (c *AppConfig) AddOrUpdatePersona(p models.AIPersona) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, existing := range c.Personas {
		if existing.ID == p.ID {
			c.Personas[i] = p
			return
		}
	}
	c.Personas = append(c.Personas, p)
}

func (c *AppConfig) DeletePersona(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, p := range c.Personas {
		if p.ID == id {
			c.Personas = append(c.Personas[:i], c.Personas[i+1:]...)
			return
		}
	}
}

// ---- Template methods ----

func (c *AppConfig) GetAllTemplates() []models.RoomTemplate {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]models.RoomTemplate, len(c.Templates))
	copy(out, c.Templates)
	return out
}

func (c *AppConfig) AddOrUpdateTemplate(t models.RoomTemplate) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, existing := range c.Templates {
		if existing.ID == t.ID {
			c.Templates[i] = t
			return
		}
	}
	c.Templates = append(c.Templates, t)
}

func (c *AppConfig) DeleteTemplate(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, t := range c.Templates {
		if t.ID == id {
			c.Templates = append(c.Templates[:i], c.Templates[i+1:]...)
			return
		}
	}
}

func (c *AppConfig) GetAllFolders() []models.PersonaFolder {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]models.PersonaFolder, len(c.Folders))
	copy(out, c.Folders)
	return out
}

func (c *AppConfig) AddOrUpdateFolder(f models.PersonaFolder) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, existing := range c.Folders {
		if existing.ID == f.ID {
			c.Folders[i] = f
			return
		}
	}
	c.Folders = append(c.Folders, f)
}

func (c *AppConfig) DeleteFolder(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, f := range c.Folders {
		if f.ID == id {
			c.Folders = append(c.Folders[:i], c.Folders[i+1:]...)
			return
		}
	}
}
