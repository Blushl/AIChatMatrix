package chat

import (
	"AIChatMatrix/internal/config"
	"AIChatMatrix/internal/models"
	"AIChatMatrix/internal/store"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (h *Hub) broadcastSysMsg(roomID, text, senderName string) {
	room, _ := store.Get().GetRoom(roomID)
	if room != nil && len(room.Messages) > 0 {
		last := room.Messages[len(room.Messages)-1]
		if last.IsSystem && strings.TrimSpace(last.Content) == strings.TrimSpace(text) && last.AgentName == senderName {
			return
		}
	}
	sysMsg := models.Message{
		ID:        uuid.New().String(),
		RoomID:    roomID,
		AgentID:   "system",
		AgentName: senderName,
		Content:   text,
		Timestamp: time.Now(),
		IsSystem:  true,
	}
	store.Get().AddMessage(roomID, sysMsg)
	h.Broadcast(roomID, models.WSEvent{Type: models.EventMessage, Payload: sysMsg})
}

func (h *Hub) broadcastStopCondition(roomID, reason string) {
	h.Broadcast(roomID, models.WSEvent{
		Type:    models.EventStopped,
		Payload: map[string]string{"room_id": roomID, "reason": reason},
	})
}

// RefereeResult holds the referee AI's decision
type RefereeResult struct {
	Text     string
	StopRoom bool
}

// generateRefereeCommand asks the referee AI to assess the discussion
func generateRefereeCommand(room *models.Room, referee models.Agent) (RefereeResult, error) {
	provider, ok := config.Get().GetProvider(referee.ProviderID)
	if !ok {
		return RefereeResult{}, fmt.Errorf("provider '%s' not found for referee", referee.ProviderID)
	}

	stopNote := ""
	if room.StopCondition != "" {
		stopNote = fmt.Sprintf(`
Stop condition: when you judge the discussion has reached "%s", output exactly: STOP_DISCUSSION`, room.StopCondition)
	}
	existingNames := make([]string, 0, len(room.Agents))
	for _, a := range room.Agents {
		existingNames = append(existingNames, fmt.Sprintf("%s(#%d)", a.Name, a.Index))
	}
	rosterText := "(none)"
	if len(existingNames) > 0 {
		rosterText = strings.Join(existingNames, ", ")
	}

	systemPrompt := fmt.Sprintf(`You are %s, the AI referee of this discussion room.

Room: %s
Topic: %s
Rules: %s
%s

Current AI roster (do not duplicate names): %s

Your personality / referee style: %s

Your role:
1. Read the discussion history below.
2. Decide what NEW system directive to issue to guide the participants forward.
3. You can manage robots with control lines:
   - Add robot: [ADD_AI] 机器人名 | 简短人设 | 添加原因（可附加“辅助”表示裁判辅助角色）
   - Remove robot: [REMOVE_AI] 机器人名 或 [REMOVE_AI] #编号
4. If the stop condition is met OR the discussion has reached a natural conclusion, output exactly the word STOP_DISCUSSION on its own line.
5. Otherwise, output a SHORT directive (1-2 sentences) telling participants what to discuss or do next. Each directive must be DIFFERENT from any previous directive - do not repeat yourself.
6. When you add a robot, you MUST provide both:
   - a concrete reason why this role is needed now
   - a complete persona setting (expertise, speaking style, decision bias).
   If either is missing, do not add the robot.

Output format:
- You may output multiple lines.
- Control lines ([ADD_AI]/[REMOVE_AI]) can appear before the directive.
- Keep only one final directive paragraph for participants.
- Do not add extra commentary. Do not prefix with [指令] or any label.
- Before emitting [ADD_AI], you MUST check "Current AI roster"; if same name exists, do not add and propose a different unique name instead.
- Whether to create new participants must follow your referee prompt/personality and current stage; creating roles is optional, not mandatory.`,
		referee.Name, room.Name, room.Topic, room.Rules, stopNote, rosterText, referee.Personality)

	var msgs []models.AIMessage
	msgs = append(msgs, models.AIMessage{Role: "system", Content: systemPrompt})

	history := room.Messages
	if len(history) > 30 {
		history = history[len(history)-30:]
	}
	for _, m := range history {
		if m.IsSystem {
			// Skip previous directives - referee should not echo them back
			continue
		}
		msgs = append(msgs, models.AIMessage{Role: "user", Content: fmt.Sprintf("%s: %s", m.AgentName, m.Content)})
	}
	if len(history) == 0 {
		msgs = append(msgs, models.AIMessage{Role: "user", Content: "Discussion has just started. Issue the opening directive."})
	}

	req := models.AIRequest{
		Model:       provider.Model,
		Messages:    msgs,
		MaxTokens:   provider.MaxTokens,
		Temperature: 0.4,
		Stream:      false,
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 300
	}
	body, _ := json.Marshal(req)
	apiURL := strings.TrimRight(provider.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(context.Background(), "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return RefereeResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return RefereeResult{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var aiResp models.AIResponse
	if err := json.Unmarshal(respBody, &aiResp); err != nil || len(aiResp.Choices) == 0 {
		return RefereeResult{}, fmt.Errorf("referee API error")
	}
	text := strings.TrimSpace(aiResp.Choices[0].Message.Content)
	if stop, announcement := parseStopDiscussionDecision(text); stop {
		return RefereeResult{StopRoom: true, Text: announcement}, nil
	}
	if strings.ToUpper(strings.TrimSpace(text)) == "WAIT" || text == "" {
		return RefereeResult{}, nil
	}
	return RefereeResult{Text: text}, nil
}

// agentByIndex finds an agent by its 1-based Index field
func agentByIndex(agents []models.Agent, idx int) models.Agent {
	for _, a := range agents {
		if a.Index == idx {
			return a
		}
	}
	if len(agents) > 0 {
		return agents[0]
	}
	return models.Agent{}
}

func isSkipTurnContent(agent models.Agent, text string) bool {
	if !agent.IsRefereeAdded {
		return false
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	upper := strings.ToUpper(trimmed)
	if upper == "WAIT" || upper == "SKIP" || upper == "[SKIP]" {
		return true
	}
	if trimmed == "【跳过】" || strings.HasPrefix(trimmed, "【跳过】") {
		return true
	}
	return false
}

func buildPrompt(room *models.Room, agent models.Agent, pendingCmd string) []models.AIMessage {
	var msgs []models.AIMessage

	// Build stop condition note
	stopNote := ""
	if room.StopCondition != "" {
		stopNote = fmt.Sprintf(`
- 停止条件：当讨论结论包含"%s"时，在你的回复中包含该词语以示认可。`, room.StopCondition)
	}

	sourceNote := ""
	if agent.IsRefereeAdded {
		sourceNote = "\n你的来源：你是由裁判在讨论中动态新增的角色。"
	}

	systemPrompt := fmt.Sprintf(`你是%s，正在参与一场小组讨论。

聊天室：%s
话题：%s
规则：%s%s

你的性格/角色设定：%s

其他参与者：%s

核心行为准则：
- 当收到【系统指令】时，必须立即、严格地按照指令内容调整你的发言方向。
- 若系统指令要求“私聊/定向沟通”，请使用格式：@目标名 私聊内容（目标可为其他AI或裁判），系统会自动公开展示。
- 保持角色性格，自然地参与对话。
- 回复保持简洁（通常2-4句话）。
- 如果你是“裁判新增角色”（由裁判通过[ADD_AI]创建），且你判断当前轮无需你发言，请只输出：WAIT
- 不要在回复开头加上你自己的名字。%s`,
		agent.Name, room.Name, room.Topic, room.Rules, sourceNote,
		agent.Personality, agentNames(room.Agents, agent.ID),
		stopNote,
	)
	msgs = append(msgs, models.AIMessage{Role: "system", Content: systemPrompt})

	history := room.Messages
	if len(history) > 20 {
		history = history[len(history)-20:]
	}
	for _, m := range history {
		if m.IsSystem {
			// Skip if identical to current pendingCmd (will be injected as final user message)
			if pendingCmd != "" && strings.TrimSpace(m.Content) == strings.TrimSpace(pendingCmd) {
				continue
			}
			// Historical directives shown as user message with clear label
			msgs = append(msgs, models.AIMessage{Role: "user", Content: "【系统指令】" + m.Content})
			continue
		}
		role := "user"
		content := fmt.Sprintf("%s：%s", m.AgentName, m.Content)
		if m.AgentID == agent.ID {
			role = "assistant"
			content = m.Content
		}
		msgs = append(msgs, models.AIMessage{Role: role, Content: content})
	}

	// Inject the active directive as the LAST user message so it's the most recent input the model sees
	if pendingCmd != "" {
		msgs = append(msgs, models.AIMessage{
			Role:    "user",
			Content: fmt.Sprintf("【主持人指令】请按照以下要求发表你的观点：\n%s", pendingCmd),
		})
	} else if len(history) == 0 {
		msgs = append(msgs, models.AIMessage{
			Role:    "user",
			Content: fmt.Sprintf("讨论刚刚开始，话题是：%s。请分享你的开场观点。", room.Topic),
		})
	}
	return msgs
}

func agentNames(agents []models.Agent, excludeID string) string {
	var names []string
	for _, a := range agents {
		if a.ID != excludeID {
			names = append(names, fmt.Sprintf("%s(#%d)", a.Name, a.Index))
		}
	}
	if len(names) == 0 {
		return "(none yet)"
	}
	return strings.Join(names, ", ")
}

func wrapPrivateToPublic(privateText string, fromAgent, toAgent models.Agent) string {
	if privateText == "" {
		return ""
	}
	return fmt.Sprintf("【私聊公开】%s(#%d) -> %s(#%d)：%s", fromAgent.Name, fromAgent.Index, toAgent.Name, toAgent.Index, privateText)
}

func parsePrivateRecipient(room *models.Room, text string, sender models.Agent) (models.Agent, bool, string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return models.Agent{}, false, ""
	}
	findTarget := func(token string) (models.Agent, bool) {
		token = strings.TrimSpace(token)
		token = strings.Trim(token, "：:，,。.")
		for _, a := range room.Agents {
			if a.ID == sender.ID {
				continue
			}
			if strings.EqualFold(a.Name, token) {
				return a, true
			}
			if strings.HasPrefix(strings.ToLower(token), "#") {
				idxText := strings.TrimPrefix(strings.ToLower(token), "#")
				if idx, err := strconv.Atoi(idxText); err == nil && a.Index == idx {
					return a, true
				}
			}
		}
		return models.Agent{}, false
	}
	parseAtSyntax := func(s string) (models.Agent, bool, string) {
		atPos := strings.Index(s, "@")
		if atPos < 0 {
			return models.Agent{}, false, ""
		}
		payload := strings.TrimSpace(s[atPos+1:])
		if payload == "" {
			return models.Agent{}, false, ""
		}
		if m := regexp.MustCompile(`^([^\s:：，,。.!！？]+)\s*(?:[:：]|\s)\s*(.+)$`).FindStringSubmatch(payload); len(m) > 2 {
			targetToken := strings.TrimSpace(m[1])
			body := strings.TrimSpace(m[2])
			if body != "" {
				if target, ok := findTarget(targetToken); ok {
					return target, true, body
				}
			}
		}
		return models.Agent{}, false, ""
	}
	if target, ok, body := parseAtSyntax(trimmed); ok {
		return target, true, body
	}
	if m := regexp.MustCompile(`^(私聊给|对)\s*([^:：\s]+)\s*(私聊|说|:|：)\s*(.+)$`).FindStringSubmatch(trimmed); len(m) > 3 {
		if target, ok := findTarget(m[2]); ok {
			body := strings.TrimSpace(m[len(m)-1])
			if body != "" {
				return target, true, body
			}
		}
	}
	return models.Agent{}, false, trimmed
}

func generateObserverSummary(room *models.Room, observer models.Agent, cmdText string) (models.Message, error) {
	provider, ok := config.Get().GetProvider(observer.ProviderID)
	if !ok {
		return models.Message{}, fmt.Errorf("provider '%s' not found", observer.ProviderID)
	}
	var msgs []models.AIMessage
	systemPrompt := fmt.Sprintf(`你是旁观者 %s。你不会主动加入讨论，只在收到系统指令时输出观察总结。

房间：%s
话题：%s
规则：%s
你的风格：%s

请基于最近讨论，用2-3句简要总结：
1) 当前分歧/共识
2) 系统指令“%s”将如何影响后续讨论

直接输出总结，不要输出“旁观者：”前缀。`, observer.Name, room.Name, room.Topic, room.Rules, observer.Personality, cmdText)
	msgs = append(msgs, models.AIMessage{Role: "system", Content: systemPrompt})
	history := room.Messages
	if len(history) > 30 {
		history = history[len(history)-30:]
	}
	for _, m := range history {
		if m.IsSystem {
			msgs = append(msgs, models.AIMessage{Role: "user", Content: "【系统指令】" + m.Content})
			continue
		}
		msgs = append(msgs, models.AIMessage{Role: "user", Content: fmt.Sprintf("%s：%s", m.AgentName, m.Content)})
	}
	msgs = append(msgs, models.AIMessage{Role: "user", Content: "请给出旁观者总结。"})
	req := models.AIRequest{Model: provider.Model, Messages: msgs, MaxTokens: provider.MaxTokens, Temperature: 0.4, Stream: false}
	if req.MaxTokens == 0 {
		req.MaxTokens = 220
	}
	body, err := json.Marshal(req)
	if err != nil {
		return models.Message{}, err
	}
	apiURL := strings.TrimRight(provider.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(context.Background(), "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return models.Message{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(httpReq)
	if err != nil {
		return models.Message{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var aiResp models.AIResponse
	if err := json.Unmarshal(respBody, &aiResp); err != nil || len(aiResp.Choices) == 0 {
		return models.Message{}, fmt.Errorf("observer API error")
	}
	content := strings.TrimSpace(aiResp.Choices[0].Message.Content)
	return models.Message{ID: uuid.New().String(), RoomID: room.ID, AgentID: observer.ID, AgentIndex: observer.Index, AgentName: observer.Name, ProviderID: observer.ProviderID, Content: content, Timestamp: time.Now(), TokensUsed: aiResp.Usage.TotalTokens}, nil
}

func generateObserverChatReply(room *models.Room, observer models.Agent, userText string) (models.Message, error) {
	provider, ok := config.Get().GetProvider(observer.ProviderID)
	if !ok {
		return models.Message{}, fmt.Errorf("provider '%s' not found", observer.ProviderID)
	}
	var msgs []models.AIMessage
	systemPrompt := fmt.Sprintf(`你是旁观者 %s。你必须严格遵守以下角色设定并据此回答：
%s

房间：%s
话题：%s
规则：%s

你需要持续追踪讨论进程，结合最近上下文回答用户问题。
回答要求：
1) 先给结论，再给依据
2) 若信息不足，明确指出缺口
3) 不要冒充系统指令，不要输出“系统指令：”作为前缀`, observer.Name, observer.Personality, room.Name, room.Topic, room.Rules)
	msgs = append(msgs, models.AIMessage{Role: "system", Content: systemPrompt})
	history := room.Messages
	if len(history) > 40 {
		history = history[len(history)-40:]
	}
	for _, m := range history {
		if m.IsSystem {
			msgs = append(msgs, models.AIMessage{Role: "user", Content: "【系统消息】" + m.Content})
			continue
		}
		msgs = append(msgs, models.AIMessage{Role: "user", Content: fmt.Sprintf("%s：%s", m.AgentName, m.Content)})
	}
	msgs = append(msgs, models.AIMessage{Role: "user", Content: "用户提问：" + strings.TrimSpace(userText)})
	req := models.AIRequest{Model: provider.Model, Messages: msgs, MaxTokens: provider.MaxTokens, Temperature: 0.35, Stream: false}
	if req.MaxTokens == 0 {
		req.MaxTokens = 280
	}
	body, err := json.Marshal(req)
	if err != nil {
		return models.Message{}, err
	}
	apiURL := strings.TrimRight(provider.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(context.Background(), "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return models.Message{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(httpReq)
	if err != nil {
		return models.Message{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var aiResp models.AIResponse
	if err := json.Unmarshal(respBody, &aiResp); err != nil || len(aiResp.Choices) == 0 {
		return models.Message{}, fmt.Errorf("observer API error")
	}
	content := strings.TrimSpace(aiResp.Choices[0].Message.Content)
	return models.Message{ID: uuid.New().String(), RoomID: room.ID, AgentID: observer.ID, AgentIndex: observer.Index, AgentName: observer.Name, ProviderID: observer.ProviderID, Content: content, Timestamp: time.Now(), TokensUsed: aiResp.Usage.TotalTokens}, nil
}

func AskObserver(roomID, observerID, text string) (models.Message, error) {
	room, ok := store.Get().GetRoom(roomID)
	if !ok {
		return models.Message{}, fmt.Errorf("room not found")
	}
	for _, a := range room.Agents {
		if a.ID == observerID && a.IsObserver {
			return generateObserverChatReply(room, a, text)
		}
	}
	return models.Message{}, fmt.Errorf("observer not found")
}

func generateResponse(room *models.Room, agent models.Agent, pendingCmd string) (models.Message, error) {
	provider, ok := config.Get().GetProvider(agent.ProviderID)
	if !ok {
		return models.Message{}, fmt.Errorf("provider '%s' not found", agent.ProviderID)
	}

	aiMessages := buildPrompt(room, agent, pendingCmd)

	req := models.AIRequest{
		Model:       provider.Model,
		Messages:    aiMessages,
		MaxTokens:   provider.MaxTokens,
		Temperature: 0.9,
		Stream:      false,
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 500
	}

	body, err := json.Marshal(req)
	if err != nil {
		return models.Message{}, err
	}

	baseURL := strings.TrimRight(provider.BaseURL, "/")
	apiURL := baseURL + "/chat/completions"

	httpReq, err := http.NewRequestWithContext(context.Background(), "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return models.Message{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return models.Message{}, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.Message{}, err
	}

	var aiResp models.AIResponse
	if err := json.Unmarshal(respBody, &aiResp); err != nil {
		return models.Message{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if aiResp.Error != nil {
		return models.Message{}, fmt.Errorf("API error: %s", aiResp.Error.Message)
	}

	if len(aiResp.Choices) == 0 {
		return models.Message{}, fmt.Errorf("no choices returned from API (status %d): %s", resp.StatusCode, string(respBody))
	}

	content := strings.TrimSpace(aiResp.Choices[0].Message.Content)

	// Strip any leaked directive prefix the model may have echoed back
	if pendingCmd != "" {
		// Remove common patterns where the model echoes the directive
		for _, prefix := range []string{
			"【主持人指令】", "【系统指令】", "【系统指令 - 必须立即执行】",
			"主持人指令：", "系统指令：",
		} {
			if strings.HasPrefix(content, prefix) {
				content = strings.TrimSpace(strings.TrimPrefix(content, prefix))
			}
		}
		// If the content starts with the pendingCmd text itself, strip it
		if strings.HasPrefix(content, pendingCmd) {
			content = strings.TrimSpace(strings.TrimPrefix(content, pendingCmd))
		}
		// Strip leading newlines from any prefix removal
		content = strings.TrimSpace(content)
	}

	privateTo, isPrivate, privateBody := parsePrivateRecipient(room, content, agent)
	if isPrivate {
		content = wrapPrivateToPublic(privateBody, agent, privateTo)
	}

	return models.Message{
		ID:          uuid.New().String(),
		RoomID:      room.ID,
		AgentID:     agent.ID,
		AgentIndex:  agent.Index,
		AgentName:   agent.Name,
		ProviderID:  agent.ProviderID,
		Content:     content,
		Timestamp:   time.Now(),
		TokensUsed:  aiResp.Usage.TotalTokens,
		IsPrivate:   isPrivate,
		PrivateFrom: agent.ID,
		PrivateTo: func() string {
			if isPrivate {
				return privateTo.ID
			}
			return ""
		}(),
	}, nil
}
