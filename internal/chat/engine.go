package chat

import (
	"AIChatMatrix/internal/models"
	"AIChatMatrix/internal/store"
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Hub manages WebSocket clients and chat engine goroutines
type Hub struct {
	mu                   sync.RWMutex
	clients              map[string]map[chan models.WSEvent]bool
	engines              map[string]context.CancelFunc
	sysCmds              map[string]chan models.SystemCommand
	lastObserverCmds     map[string]string
	lastRefereeAddReject map[string]string
	rejectedAddNames     map[string]map[string]bool
	recentlyAddedNames   map[string]map[string]time.Time
	recentlyRemovedNames map[string]map[string]time.Time
}

var (
	hubInstance *Hub
	hubOnce     sync.Once
)

func GetHub() *Hub {
	hubOnce.Do(func() {
		hubInstance = &Hub{
			clients:              make(map[string]map[chan models.WSEvent]bool),
			engines:              make(map[string]context.CancelFunc),
			sysCmds:              make(map[string]chan models.SystemCommand),
			lastObserverCmds:     make(map[string]string),
			lastRefereeAddReject: make(map[string]string),
			rejectedAddNames:     make(map[string]map[string]bool),
			recentlyAddedNames:   make(map[string]map[string]time.Time),
			recentlyRemovedNames: make(map[string]map[string]time.Time),
		}
	})
	return hubInstance
}

func (h *Hub) Subscribe(roomID string) chan models.WSEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan models.WSEvent, 64)
	if h.clients[roomID] == nil {
		h.clients[roomID] = make(map[chan models.WSEvent]bool)
	}
	h.clients[roomID][ch] = true
	return ch
}

func (h *Hub) Unsubscribe(roomID string, ch chan models.WSEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs, ok := h.clients[roomID]; ok {
		delete(subs, ch)
		close(ch)
	}
}

func (h *Hub) Broadcast(roomID string, event models.WSEvent) {
	eventToSend := cloneEventForBroadcast(event)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients[roomID] {
		select {
		case ch <- eventToSend:
		default:
		}
	}
}

func cloneEventForBroadcast(event models.WSEvent) models.WSEvent {
	cloned := event
	if event.Type != models.EventRoomUpdate {
		return cloned
	}
	room, ok := event.Payload.(*models.Room)
	if !ok || room == nil {
		return cloned
	}
	roomCopy := *room
	roomCopy.Agents = append([]models.Agent(nil), room.Agents...)
	roomCopy.Messages = append([]models.Message(nil), room.Messages...)
	roomCopy.SpeakOrder = append([]int(nil), room.SpeakOrder...)
	cloned.Payload = &roomCopy
	return cloned
}

// SendSystemCommand delivers a system command into the running engine
func (h *Hub) SendSystemCommand(roomID string, cmd models.SystemCommand) bool {
	h.mu.RLock()
	ch, ok := h.sysCmds[roomID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	select {
	case ch <- cmd:
		return true
	default:
		return false
	}
}

func (h *Hub) StartEngine(roomID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, running := h.engines[roomID]; running {
		return fmt.Errorf("engine already running for room %s", roomID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.engines[roomID] = cancel
	h.sysCmds[roomID] = make(chan models.SystemCommand, 16)
	go h.runEngine(ctx, roomID)
	return nil
}

func (h *Hub) StopEngine(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cancel, ok := h.engines[roomID]; ok {
		cancel()
		delete(h.engines, roomID)
	}
	if ch, ok := h.sysCmds[roomID]; ok {
		close(ch)
		delete(h.sysCmds, roomID)
	}
	delete(h.lastObserverCmds, roomID)
	delete(h.lastRefereeAddReject, roomID)
	delete(h.rejectedAddNames, roomID)
	delete(h.recentlyAddedNames, roomID)
	delete(h.recentlyRemovedNames, roomID)
}

func (h *Hub) IsEngineRunning(roomID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.engines[roomID]
	return ok
}

func (h *Hub) runEngine(ctx context.Context, roomID string) {
	log.Printf("[Engine] Started for room %s", roomID)
	store.Get().SetRoomRunning(roomID, true)
	h.Broadcast(roomID, models.WSEvent{
		Type:    models.EventStatus,
		Payload: map[string]interface{}{"running": true, "room_id": roomID},
	})
	defer func() {
		store.Get().SetRoomRunning(roomID, false)
		h.Broadcast(roomID, models.WSEvent{
			Type:    models.EventStatus,
			Payload: map[string]interface{}{"running": false, "room_id": roomID},
		})
		log.Printf("[Engine] Stopped for room %s", roomID)
	}()
	h.mu.RLock()
	sysCmdCh := h.sysCmds[roomID]
	h.mu.RUnlock()
	pendingCmdText := ""
	agentCursor := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		room, ok := store.Get().GetRoom(roomID)
		if !ok {
			return
		}
		if len(room.Agents) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}
		if room.MaxMessages > 0 && len(room.Messages) >= room.MaxMessages {
			h.mu.Lock()
			delete(h.engines, roomID)
			h.mu.Unlock()
			return
		}
		referee, hasReferee := findReferee(room.Agents)
		normals := normalAgents(room.Agents)
		// ── REFEREE TURN ──
		if hasReferee {
			refereeResult, err := generateRefereeCommand(room, referee)
			if err != nil {
				log.Printf("[Engine] Referee error: %v", err)
			} else if refereeResult.StopRoom {
				stopMsg := strings.TrimSpace(refereeResult.Text)
				if stopMsg == "" {
					stopMsg = "裁判判定讨论已收敛，触发 STOP_DISCUSSION，房间停止。"
				}
				h.broadcastSysMsg(roomID, stopMsg, referee.Name)
				h.broadcastStopCondition(roomID, "STOP_DISCUSSION")
				h.mu.Lock()
				delete(h.engines, roomID)
				h.mu.Unlock()
				return
			} else if refereeResult.Text != "" {
				directive := h.applyRefereeOps(roomID, referee, refereeResult.Text)
				if directive == "" {
					continue
				}
				pendingCmdText = directive
				h.broadcastSysMsg(roomID, directive, referee.Name)
				h.generateObserverSummaries(roomID, directive)
				room, _ = store.Get().GetRoom(roomID)
				// Let ALL normal agents respond to this directive before referee speaks again
				for i := 0; i < len(normals); i++ {
					select {
					case <-ctx.Done():
						return
					default:
					}
					room, ok = store.Get().GetRoom(roomID)
					if !ok {
						return
					}
					normals = normalAgents(room.Agents)
					if len(normals) == 0 {
						break
					}
					agent := normals[agentCursor%len(normals)]
					agentCursor++
					// interval wait
					waitDone := makeWaitTimer()
				waitRefereeInterval:
					for {
						select {
						case <-ctx.Done():
							return
						case cmd, ok := <-sysCmdCh:
							if !ok {
								return
							}
							if cmd.StopRoom {
								if cmd.Text != "" {
									h.broadcastSysMsg(roomID, cmd.Text, "系统")
								}
								h.broadcastStopCondition(roomID, "系统指令")
								h.mu.Lock()
								delete(h.engines, roomID)
								h.mu.Unlock()
								return
							}
							if cmd.Text != "" {
								pendingCmdText = cmd.Text
								h.broadcastSysMsg(roomID, cmd.Text, "系统指令")
								h.generateObserverSummaries(roomID, cmd.Text)
								room, _ = store.Get().GetRoom(roomID)
							}
						case <-waitDone:
							break waitRefereeInterval
						}
					}
					room, _ = store.Get().GetRoom(roomID)
					if room == nil {
						return
					}
					stillExists := false
					for _, live := range room.Agents {
						if live.ID == agent.ID {
							stillExists = true
							agent = live
							break
						}
					}
					if !stillExists {
						continue
					}
					h.Broadcast(roomID, models.WSEvent{Type: "thinking", Payload: map[string]string{"agent": agent.Name, "agent_id": agent.ID}})
					msg, err := generateResponse(room, agent, pendingCmdText)
					if err != nil {
						log.Printf("[Engine] Error for agent %s: %v", agent.Name, err)
						h.Broadcast(roomID, models.WSEvent{
							Type:    models.EventError,
							Payload: map[string]string{"agent": agent.Name, "error": err.Error()},
						})
						time.Sleep(3 * time.Second)
						continue
					}
					if isSkipTurnContent(agent, msg.Content) {
						continue
					}
					store.Get().AddMessage(roomID, msg)
					h.Broadcast(roomID, models.WSEvent{Type: models.EventMessage, Payload: msg})
					if msg.IsPrivate && msg.PrivateTo != "" {
						if nextIdx := normalAgentIndexByID(normals, msg.PrivateTo); nextIdx >= 0 {
							agentCursor = nextIdx
						}
					}
					// Check max messages
					room, _ = store.Get().GetRoom(roomID)
					if room.MaxMessages > 0 && len(room.Messages) >= room.MaxMessages {
						h.mu.Lock()
						delete(h.engines, roomID)
						h.mu.Unlock()
						return
					}
				}
				// All agents responded — referee will assess next iteration
				pendingCmdText = ""
				continue
			} else {
				// Referee said WAIT - keep chat flowing and re-assess quickly
				time.Sleep(1200 * time.Millisecond)
				continue
			}
		}
		// ── PICK NORMAL AGENT ──
		if len(normals) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}
		agent := normals[agentCursor%len(normals)]
		agentCursor++
		// ── INTERVAL WAIT (also drains any incoming syscmds) ──
		waitDone := makeWaitTimer()
	waitInterval:
		for {
			select {
			case <-ctx.Done():
				return
			case cmd, ok := <-sysCmdCh:
				if !ok {
					return
				}
				if cmd.StopRoom {
					if cmd.Text != "" {
						h.broadcastSysMsg(roomID, cmd.Text, "系统")
					}
					h.broadcastStopCondition(roomID, "系统指令")
					h.mu.Lock()
					delete(h.engines, roomID)
					h.mu.Unlock()
					return
				}
				if cmd.Text != "" {
					pendingCmdText = cmd.Text
					h.broadcastSysMsg(roomID, cmd.Text, "系统指令")
					h.generateObserverSummaries(roomID, cmd.Text)
					room, _ = store.Get().GetRoom(roomID)
				}
			case <-waitDone:
				break waitInterval
			}
		}
		room, ok = store.Get().GetRoom(roomID)
		if !ok {
			return
		}
		normals = normalAgents(room.Agents)
		if len(normals) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}
		agent = normals[agentCursor%len(normals)]
		stillExists := false
		for _, live := range room.Agents {
			if live.ID == agent.ID {
				stillExists = true
				agent = live
				break
			}
		}
		if !stillExists {
			continue
		}
		// ── GENERATE AGENT RESPONSE ──
		h.Broadcast(roomID, models.WSEvent{Type: "thinking", Payload: map[string]string{"agent": agent.Name, "agent_id": agent.ID}})
		msg, err := generateResponse(room, agent, pendingCmdText)
		if err != nil {
			log.Printf("[Engine] Error for agent %s: %v", agent.Name, err)
			h.Broadcast(roomID, models.WSEvent{
				Type:    models.EventError,
				Payload: map[string]string{"agent": agent.Name, "error": err.Error()},
			})
			time.Sleep(3 * time.Second)
			continue
		}
		if isSkipTurnContent(agent, msg.Content) {
			continue
		}
		store.Get().AddMessage(roomID, msg)
		h.Broadcast(roomID, models.WSEvent{Type: models.EventMessage, Payload: msg})
		if msg.IsPrivate && msg.PrivateTo != "" {
			if nextIdx := normalAgentIndexByID(normals, msg.PrivateTo); nextIdx >= 0 {
				agentCursor = nextIdx
			}
		}
		// If no referee, clear pending command after each agent turn (next turn waits again)
		if !hasReferee {
			pendingCmdText = ""
		}
		// Keyword stop condition (fallback when no referee)
		if !hasReferee && room.StopCondition != "" &&
			strings.Contains(strings.ToLower(msg.Content), strings.ToLower(room.StopCondition)) {
			log.Printf("[Engine] Stop condition met: %q", room.StopCondition)
			h.broadcastSysMsg(roomID, fmt.Sprintf("停止条件已触发：检测到关键词 \"%s\"，讨论结束。", room.StopCondition), "系统")
			h.broadcastStopCondition(roomID, room.StopCondition)
			h.mu.Lock()
			delete(h.engines, roomID)
			h.mu.Unlock()
			return
		}
	}
}
