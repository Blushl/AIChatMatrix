package api

import (
	"AIChatMatrix/internal/chat"
	"AIChatMatrix/internal/config"
	"AIChatMatrix/internal/models"
	"AIChatMatrix/internal/store"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool { return true },
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decode(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// ---- Provider Handlers ----

func HandleListProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, config.Get().GetAllProviders())
}

func HandleCreateProvider(w http.ResponseWriter, r *http.Request) {
	var p config.ModelProvider
	if err := decode(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if p.Name == "" || p.BaseURL == "" || p.Model == "" {
		writeError(w, http.StatusBadRequest, "name, base_url and model are required")
		return
	}
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	config.Get().AddOrUpdateProvider(p)
	_ = config.Get().Save()
	writeJSON(w, http.StatusCreated, p)
}

func HandleUpdateProvider(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/providers/")
	var p config.ModelProvider
	if err := decode(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p.ID = id
	config.Get().AddOrUpdateProvider(p)
	_ = config.Get().Save()
	writeJSON(w, http.StatusOK, p)
}

func HandleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/providers/")
	config.Get().DeleteProvider(id)
	_ = config.Get().Save()
	w.WriteHeader(http.StatusNoContent)
}

// ---- Persona Handlers ----

// GET /api/personas
func HandleListPersonas(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, config.Get().GetAllPersonas())
}

// POST /api/personas
func HandleCreatePersona(w http.ResponseWriter, r *http.Request) {
	var p models.AIPersona
	if err := decode(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if p.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	config.Get().AddOrUpdatePersona(p)
	_ = config.Get().Save()
	writeJSON(w, http.StatusCreated, p)
}

// PUT /api/personas/{id}
func HandleUpdatePersona(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/personas/")
	var p models.AIPersona
	if err := decode(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p.ID = id
	config.Get().AddOrUpdatePersona(p)
	_ = config.Get().Save()
	writeJSON(w, http.StatusOK, p)
}

// DELETE /api/personas/{id}
func HandleDeletePersona(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/personas/")
	config.Get().DeletePersona(id)
	_ = config.Get().Save()
	w.WriteHeader(http.StatusNoContent)
}

// ---- Folder Handlers ----

// GET /api/folders
func HandleListFolders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, config.Get().GetAllFolders())
}

// POST /api/folders
func HandleCreateFolder(w http.ResponseWriter, r *http.Request) {
	var f models.PersonaFolder
	if err := decode(r, &f); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if f.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	config.Get().AddOrUpdateFolder(f)
	_ = config.Get().Save()
	writeJSON(w, http.StatusCreated, f)
}

// PUT /api/folders/{id}
func HandleUpdateFolder(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/folders/")
	var f models.PersonaFolder
	if err := decode(r, &f); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	f.ID = id
	config.Get().AddOrUpdateFolder(f)
	_ = config.Get().Save()
	writeJSON(w, http.StatusOK, f)
}

// DELETE /api/folders/{id}
func HandleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/folders/")
	config.Get().DeleteFolder(id)
	_ = config.Get().Save()
	w.WriteHeader(http.StatusNoContent)
}

// ---- Template Handlers ----

// GET /api/templates
func HandleListTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, config.Get().GetAllTemplates())
}

// POST /api/templates
func HandleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var t models.RoomTemplate
	if err := decode(r, &t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if t.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	config.Get().AddOrUpdateTemplate(t)
	_ = config.Get().Save()
	writeJSON(w, http.StatusCreated, t)
}

// PUT /api/templates/{id}
func HandleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/templates/")
	var t models.RoomTemplate
	if err := decode(r, &t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t.ID = id
	config.Get().AddOrUpdateTemplate(t)
	_ = config.Get().Save()
	writeJSON(w, http.StatusOK, t)
}

// DELETE /api/templates/{id}
func HandleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/templates/")
	config.Get().DeleteTemplate(id)
	_ = config.Get().Save()
	w.WriteHeader(http.StatusNoContent)
}

// ---- Room Handlers ----

func HandleListRooms(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, store.Get().GetAllRooms())
}

func HandleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name          string `json:"name"`
		Topic         string `json:"topic"`
		Rules         string `json:"rules"`
		MaxMessages   int    `json:"max_messages"`
		StopCondition string `json:"stop_condition"`
	}
	if err := decode(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	room := store.Get().CreateRoom(body.Name, body.Topic, body.Rules, body.MaxMessages)
	room.StopCondition = body.StopCondition
	if providers := config.Get().GetAllProviders(); len(providers) > 0 {
		observer := models.Agent{
			ID:          uuid.New().String(),
			Name:        "旁观者",
			ProviderID:  providers[0].ID,
			Personality: "请保持中立，重点提炼讨论脉络与分歧点。",
			AvatarColor: "#7c3aed",
			IsObserver:  true,
		}
		_ = store.Get().AddAgent(room.ID, observer)
	}
	store.Get().UpdateRoom(room)
	writeJSON(w, http.StatusCreated, room)
}

func HandleGetRoom(w http.ResponseWriter, r *http.Request) {
	id := extractRoomID(r.URL.Path, "")
	room, ok := store.Get().GetRoom(id)
	if !ok {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func HandleUpdateRoom(w http.ResponseWriter, r *http.Request) {
	id := extractRoomID(r.URL.Path, "")
	room, ok := store.Get().GetRoom(id)
	if !ok {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	var body struct {
		Name          string `json:"name"`
		Topic         string `json:"topic"`
		Rules         string `json:"rules"`
		MaxMessages   int    `json:"max_messages"`
		SpeakOrder    []int  `json:"speak_order"`
		StopCondition string `json:"stop_condition"`
	}
	if err := decode(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name != "" {
		room.Name = body.Name
	}
	room.Topic = body.Topic
	room.Rules = body.Rules
	room.MaxMessages = body.MaxMessages
	room.SpeakOrder = body.SpeakOrder
	room.StopCondition = body.StopCondition
	store.Get().UpdateRoom(room)
	writeJSON(w, http.StatusOK, room)
}

func HandleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	id := extractRoomID(r.URL.Path, "")
	chat.GetHub().StopEngine(id)
	store.Get().DeleteRoom(id)
	w.WriteHeader(http.StatusNoContent)
}

// ---- Agent Handlers ----

func HandleAddAgent(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/rooms/"), "/")
	if len(parts) < 2 {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	roomID := parts[0]
	var agent models.Agent
	if err := decode(r, &agent); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if agent.Name == "" || agent.ProviderID == "" {
		writeError(w, http.StatusBadRequest, "name and provider_id are required")
		return
	}
	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}
	if agent.AvatarColor == "" {
		colors := []string{"#7c6af7", "#22d3ee", "#f97316", "#22c55e", "#ec4899", "#a855f7", "#eab308", "#06b6d4"}
		agent.AvatarColor = colors[len(agent.Name)%len(colors)]
	}
	// Assign index
	room, ok := store.Get().GetRoom(roomID)
	if !ok {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	if agent.Index <= 0 {
		nextIndex := 1
		for _, existing := range room.Agents {
			if existing.Index >= nextIndex {
				nextIndex = existing.Index + 1
			}
		}
		agent.Index = nextIndex
	}
	if !store.Get().AddAgent(roomID, agent) {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	writeJSON(w, http.StatusCreated, agent)
}

func HandleRemoveAgent(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/rooms/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	roomID := parts[0]
	agentID := parts[2]
	if !store.Get().RemoveAgent(roomID, agentID) {
		writeError(w, http.StatusNotFound, "room or agent not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Engine Control ----

func HandleStartRoom(w http.ResponseWriter, r *http.Request) {
	id := extractRoomID(r.URL.Path, "/start")
	room, ok := store.Get().GetRoom(id)
	if !ok {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	if len(room.Agents) == 0 {
		writeError(w, http.StatusBadRequest, "add at least one agent before starting")
		return
	}
	if err := chat.GetHub().StartEngine(id); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started", "room_id": id})
}

func HandleStopRoom(w http.ResponseWriter, r *http.Request) {
	id := extractRoomID(r.URL.Path, "/stop")
	chat.GetHub().StopEngine(id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped", "room_id": id})
}

// POST /api/rooms/{id}/restart — clear messages then start
func HandleRestartRoom(w http.ResponseWriter, r *http.Request) {
	id := extractRoomID(r.URL.Path, "/restart")
	room, ok := store.Get().GetRoom(id)
	if !ok {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	if len(room.Agents) == 0 {
		writeError(w, http.StatusBadRequest, "add at least one agent before starting")
		return
	}
	// Stop any running engine first and wait briefly for it to settle
	chat.GetHub().StopEngine(id)
	time.Sleep(500 * time.Millisecond)
	// Clear messages
	room.Messages = []models.Message{}
	store.Get().UpdateRoom(room)
	// Start fresh
	if err := chat.GetHub().StartEngine(id); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted", "room_id": id})
}

func HandleGetMessages(w http.ResponseWriter, r *http.Request) {
	id := extractRoomID(r.URL.Path, "/messages")
	room, ok := store.Get().GetRoom(id)
	if !ok {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	writeJSON(w, http.StatusOK, room.Messages)
}

// POST /api/rooms/{id}/syscmd
func HandleSystemCommand(w http.ResponseWriter, r *http.Request) {
	id := extractRoomID(r.URL.Path, "/syscmd")
	var cmd models.SystemCommand
	if err := decode(r, &cmd); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cmd.RoomID = id
	if !chat.GetHub().SendSystemCommand(id, cmd) {
		// Engine not running — still save directive as message if text provided
		if cmd.Text != "" {
			sysMsg := models.Message{
				ID:        uuid.New().String(),
				RoomID:    id,
				AgentID:   "system",
				AgentName: "系统指令",
				Content:   cmd.Text,
				Timestamp: time.Now(),
				IsSystem:  true,
			}
			store.Get().AddMessage(id, sysMsg)
			chat.GetHub().Broadcast(id, models.WSEvent{Type: models.EventMessage, Payload: sysMsg})
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /api/rooms/{id}/observer-chat
func HandleObserverChat(w http.ResponseWriter, r *http.Request) {
	id := extractRoomID(r.URL.Path, "/observer-chat")
	var body struct {
		ObserverID string `json:"observer_id"`
		Text       string `json:"text"`
	}
	if err := decode(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(body.ObserverID) == "" || strings.TrimSpace(body.Text) == "" {
		writeError(w, http.StatusBadRequest, "observer_id and text are required")
		return
	}
	msg, err := chat.AskObserver(id, body.ObserverID, body.Text)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, msg)
}
// WS /api/rooms/{id}/ws
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	id := extractRoomID(r.URL.Path, "/ws")
	_, ok := store.Get().GetRoom(id)
	if !ok {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	ch := chat.GetHub().Subscribe(id)
	defer chat.GetHub().Unsubscribe(id, ch)

	room, _ := store.Get().GetRoom(id)
	_ = conn.WriteJSON(models.WSEvent{Type: models.EventRoomUpdate, Payload: room})

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteJSON(event); err != nil {
				log.Printf("[WS] Write error: %v", err)
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// POST /api/rooms/{id}/clone
func HandleCloneRoom(w http.ResponseWriter, r *http.Request) {
	id := extractRoomID(r.URL.Path, "/clone")
	var body struct {
		Name string `json:"name"`
	}
	_ = decode(r, &body)
	clone, ok := store.Get().CloneRoom(id, body.Name)
	if !ok {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	writeJSON(w, http.StatusCreated, clone)
}

// GET /api/rooms/{id}/export?format=json|markdown
func HandleExportRoom(w http.ResponseWriter, r *http.Request) {
	id := extractRoomID(r.URL.Path, "/export")
	room, ok := store.Get().GetRoom(id)
	if !ok {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "markdown"
	}
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="discussion.json"`)
		_ = json.NewEncoder(w).Encode(room)
	default:
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="discussion.md"`)
		md := buildMarkdownExport(room)
		_, _ = w.Write([]byte(md))
	}
}

func buildMarkdownExport(room *models.Room) string {
	var sb strings.Builder
	sb.WriteString("# " + room.Name + "\n\n")
	if room.Topic != "" {
		sb.WriteString("> **话题：** " + room.Topic + "\n\n")
	}
	if room.Rules != "" {
		sb.WriteString("> **规则：** " + room.Rules + "\n\n")
	}
	if room.StopCondition != "" {
		sb.WriteString("> **停止条件：** " + room.StopCondition + "\n\n")
	}
	sb.WriteString("---\n\n")
	for _, m := range room.Messages {
		ts := m.Timestamp.Format("15:04:05")
		if m.IsSystem {
			sb.WriteString("*⚡ [系统指令 " + ts + "] " + m.Content + "*\n\n")
			continue
		}
		tokStr := ""
		if m.TokensUsed > 0 {
			tokStr = fmt.Sprintf(" (%dt)", m.TokensUsed)
		}
		sb.WriteString(fmt.Sprintf("### #%d %s `%s`%s\n\n%s\n\n",
			m.AgentIndex, m.AgentName, ts, tokStr, m.Content))
	}
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("*导出时间：%s | 共 %d 条消息*\n",
		time.Now().Format("2006-01-02 15:04:05"), len(room.Messages)))
	return sb.String()
}

func extractRoomID(path, suffix string) string {
	path = strings.TrimPrefix(path, "/api/rooms/")
	if suffix != "" {
		return strings.TrimSuffix(path, suffix)
	}
	// strip any sub-path
	if idx := strings.Index(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return path
}
