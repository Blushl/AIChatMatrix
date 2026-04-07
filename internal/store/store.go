package store

import (
	"AIChatMatrix/internal/models"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store holds all in-memory state
type Store struct {
	mu    sync.RWMutex
	rooms map[string]*models.Room
}

var (
	instance *Store
	once     sync.Once
)

func Get() *Store {
	once.Do(func() {
		instance = &Store{
			rooms: make(map[string]*models.Room),
		}
	})
	return instance
}

func (s *Store) CreateRoom(name, topic, rules string, maxMessages int) *models.Room {
	s.mu.Lock()
	defer s.mu.Unlock()
	room := &models.Room{
		ID:          uuid.New().String(),
		Name:        name,
		Topic:       topic,
		Rules:       rules,
		Agents:      []models.Agent{},
		Messages:    []models.Message{},
		IsRunning:   false,
		CreatedAt:   time.Now(),
		MaxMessages: maxMessages,
	}
	s.rooms[room.ID] = room
	return room
}

func (s *Store) GetRoom(id string) (*models.Room, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rooms[id]
	return r, ok
}

func (s *Store) GetAllRooms() []*models.Room {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rooms := make([]*models.Room, 0, len(s.rooms))
	for _, r := range s.rooms {
		rooms = append(rooms, r)
	}
	return rooms
}

func (s *Store) DeleteRoom(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rooms, id)
}

func (s *Store) UpdateRoom(room *models.Room) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rooms[room.ID] = room
}

func (s *Store) AddAgent(roomID string, agent models.Agent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.rooms[roomID]
	if !ok {
		return false
	}
	// Check if agent already exists
	for i, a := range room.Agents {
		if a.ID == agent.ID {
			room.Agents[i] = agent
			return true
		}
	}
	room.Agents = append(room.Agents, agent)
	return true
}

func (s *Store) RemoveAgent(roomID, agentID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.rooms[roomID]
	if !ok {
		return false
	}
	for i, a := range room.Agents {
		if a.ID == agentID {
			room.Agents = append(room.Agents[:i], room.Agents[i+1:]...)
			return true
		}
	}
	return false
}

func (s *Store) AddMessage(roomID string, msg models.Message) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.rooms[roomID]
	if !ok {
		return false
	}
	room.Messages = append(room.Messages, msg)
	// Keep last 500 messages in memory
	if len(room.Messages) > 500 {
		room.Messages = room.Messages[len(room.Messages)-500:]
	}
	return true
}

func (s *Store) SetRoomRunning(roomID string, running bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.rooms[roomID]
	if !ok {
		return false
	}
	room.IsRunning = running
	return true
}

// CloneRoom creates a deep copy of a room (without messages, not running)
func (s *Store) CloneRoom(sourceID, newName string) (*models.Room, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src, ok := s.rooms[sourceID]
	if !ok {
		return nil, false
	}
	// Deep copy agents
	agents := make([]models.Agent, len(src.Agents))
	for i, a := range src.Agents {
		newAgent := a
		newAgent.ID = uuid.New().String()
		agents[i] = newAgent
	}
	name := newName
	if name == "" {
		name = src.Name + " (副本)"
	}
	clone := &models.Room{
		ID:            uuid.New().String(),
		Name:          name,
		Topic:         src.Topic,
		Rules:         src.Rules,
		Agents:        agents,
		Messages:      []models.Message{},
		IsRunning:     false,
		CreatedAt:     time.Now(),
		MaxMessages:   src.MaxMessages,
		SpeakOrder:    append([]int{}, src.SpeakOrder...),
		StopCondition: src.StopCondition,
		ClonedFrom:    sourceID,
	}
	s.rooms[clone.ID] = clone
	return clone, true
}
