package chat

import (
	"AIChatMatrix/internal/models"
	"AIChatMatrix/internal/store"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// findReferee returns the referee agent if one exists
func findReferee(agents []models.Agent) (models.Agent, bool) {
	for _, a := range agents {
		if a.IsReferee {
			return a, true
		}
	}
	return models.Agent{}, false
}

// normalAgents returns agents that are not the referee/observer
func normalAgents(agents []models.Agent) []models.Agent {
	var result []models.Agent
	for _, a := range agents {
		if !a.IsReferee && !a.IsObserver {
			result = append(result, a)
		}
	}
	return result
}

func observerAgents(agents []models.Agent) []models.Agent {
	var result []models.Agent
	for _, a := range agents {
		if a.IsObserver {
			result = append(result, a)
		}
	}
	return result
}

func normalAgentIndexByID(agents []models.Agent, agentID string) int {
	for i, a := range agents {
		if a.ID == agentID {
			return i
		}
	}
	return -1
}

func makeWaitTimer() <-chan time.Time {
	return time.After(1200 * time.Millisecond)
}

type addOp struct {
	Name        string
	Personality string
	Reason      string
	Assistant   bool
}

type refereeOps struct {
	AddItems     []addOp
	RemoveTarget string
	Directive    string
}

func normalizeRefereeText(text string) string {
	normalized := strings.TrimSpace(text)
	normalized = strings.ReplaceAll(normalized, "【", "[")
	normalized = strings.ReplaceAll(normalized, "】", "]")
	normalized = strings.ReplaceAll(normalized, "：", ":")
	normalized = strings.ReplaceAll(normalized, "｜", "|")
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	return normalized
}

func sanitizeRefereeDirectiveLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if m := regexp.MustCompile(`(?i)^\s*\[(NO_OP|NOOP)\]\s*[:：-]?\s*(.*)$`).FindStringSubmatch(line); len(m) > 2 {
		line = strings.TrimSpace(m[2])
	}
	if strings.EqualFold(strings.TrimSpace(line), "NO_OP") || strings.EqualFold(strings.TrimSpace(line), "NOOP") {
		return ""
	}
	return strings.TrimSpace(line)
}

func parseRefereeOps(text string) refereeOps {
	normalized := normalizeRefereeText(text)
	ops := refereeOps{Directive: normalized}
	if normalized == "" {
		return ops
	}
	lines := strings.Split(normalized, "\n")
	var left []string
	addRe := regexp.MustCompile(`(?i)^\s*\[(ADD_AI|添加AI|ADD)\]\s*(.+)$`)
	removeRe := regexp.MustCompile(`(?i)^\s*\[(REMOVE_AI|移除AI|REMOVE)\]\s*(.+)$`)
	for _, raw := range lines {
		line := sanitizeRefereeDirectiveLine(raw)
		if line == "" {
			continue
		}
		if m := addRe.FindStringSubmatch(line); len(m) > 2 {
			payload := strings.TrimSpace(m[2])
			parts := strings.Split(payload, "|")
			item := addOp{}
			if len(parts) > 0 {
				item.Name = strings.TrimSpace(parts[0])
			}
			if len(parts) > 1 {
				item.Personality = strings.TrimSpace(parts[1])
			}
			if len(parts) > 2 {
				rest := strings.TrimSpace(strings.Join(parts[2:], "|"))
				restLower := strings.ToLower(rest)
				if strings.Contains(restLower, "辅助") || strings.Contains(restLower, "assistant") {
					item.Assistant = true
					rest = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(rest, "辅助", ""), "assistant", ""))
					rest = strings.Trim(rest, "|：:，, ")
				}
				item.Reason = rest
			}
			if item.Name != "" {
				ops.AddItems = append(ops.AddItems, item)
			}
			continue
		}
		if m := removeRe.FindStringSubmatch(line); len(m) > 2 {
			ops.RemoveTarget = strings.TrimSpace(m[2])
			continue
		}
		left = append(left, line)
	}
	ops.Directive = strings.TrimSpace(strings.Join(left, "\n"))
	return ops
}

func parseStopDiscussionDecision(text string) (bool, string) {
	normalized := normalizeRefereeText(text)
	if normalized == "" {
		return false, ""
	}
	lines := strings.Split(normalized, "\n")
	stop := false
	var remain []string
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.EqualFold(line, "STOP_DISCUSSION") {
			stop = true
			continue
		}
		if strings.Contains(strings.ToUpper(line), "STOP_DISCUSSION") {
			stop = true
			clean := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(line, "STOP_DISCUSSION", ""), "stop_discussion", ""))
			clean = sanitizeRefereeDirectiveLine(clean)
			if clean != "" {
				remain = append(remain, clean)
			}
			continue
		}
		remain = append(remain, sanitizeRefereeDirectiveLine(line))
	}
	announcement := strings.TrimSpace(strings.Join(remain, "\n"))
	if stop {
		announcement = parseRefereeOps(announcement).Directive
	}
	return stop, announcement
}

func shouldTriggerObserverSummary(cmdText string) bool {
	return false
}

func (h *Hub) applyRefereeOps(roomID string, referee models.Agent, rawText string) string {
	ops := parseRefereeOps(rawText)
	if len(ops.AddItems) > 0 {
		room, ok := store.Get().GetRoom(roomID)
		if ok {
			for _, item := range ops.AddItems {
				newName := strings.TrimSpace(item.Name)
				if newName == "" {
					continue
				}
				exists := false
				for _, existing := range room.Agents {
					if strings.EqualFold(strings.TrimSpace(existing.Name), newName) {
						exists = true
						break
					}
				}
				if exists {
					// Same-name role already exists in room; treat repeated ADD as idempotent no-op.
					continue
				}
				colors := []string{"#7c6af7", "#22d3ee", "#f97316", "#22c55e", "#ec4899", "#a855f7", "#eab308", "#06b6d4"}
				personality := strings.TrimSpace(item.Personality)
				if personality == "" {
					personality = "请基于当前讨论补齐关键盲区，给出可执行建议，并与其他角色形成互补。"
				}
				nextIndex := 1
				for _, existing := range room.Agents {
					if existing.Index >= nextIndex {
						nextIndex = existing.Index + 1
					}
				}
				agent := models.Agent{
					ID:             uuid.New().String(),
					Index:          nextIndex,
					Name:           newName,
					ProviderID:     referee.ProviderID,
					Personality:    personality,
					AvatarColor:    colors[len(newName)%len(colors)],
					IsReferee:      false,
					IsObserver:     false,
					IsAssistant:    item.Assistant,
					IsRefereeAdded: true,
				}
				store.Get().AddAgent(roomID, agent)
				h.mu.Lock()
				h.lastRefereeAddReject[roomID] = ""
				if h.rejectedAddNames[roomID] != nil {
					delete(h.rejectedAddNames[roomID], strings.ToLower(strings.TrimSpace(agent.Name)))
				}
				if h.recentlyAddedNames[roomID] == nil {
					h.recentlyAddedNames[roomID] = map[string]time.Time{}
				}
				h.recentlyAddedNames[roomID][strings.ToLower(strings.TrimSpace(agent.Name))] = time.Now()
				h.mu.Unlock()
				h.Broadcast(roomID, models.WSEvent{Type: models.EventRoomUpdate, Payload: room})
				reason := strings.TrimSpace(item.Reason)
				if reason == "" {
					reason = "补齐当前讨论盲区并提升结论质量"
				}
				roleLabel := "普通角色"
				if agent.IsAssistant {
					roleLabel = "裁判辅助角色"
				}
				h.broadcastSysMsg(roomID, fmt.Sprintf("裁判新增机器人：%s(#%d)\n角色类型：%s\n添加原因：%s\n角色设定：%s", agent.Name, agent.Index, roleLabel, reason, agent.Personality), referee.Name)
			}
		}
	}
	if ops.RemoveTarget != "" {
		room, ok := store.Get().GetRoom(roomID)
		if ok {
			target := strings.TrimSpace(ops.RemoveTarget)
			target = strings.TrimPrefix(target, "移除")
			target = strings.TrimPrefix(strings.TrimSpace(target), "删除")
			target = strings.Trim(target, "：:，,。.")
			targetKey := strings.ToLower(strings.TrimSpace(target))
			parsedIdx := -1
			if m := regexp.MustCompile(`#\s*(\d+)`).FindStringSubmatch(target); len(m) > 1 {
				if idx, err := strconv.Atoi(m[1]); err == nil {
					parsedIdx = idx
				}
			}
			removed := false
			for _, a := range room.Agents {
				if a.IsReferee || a.IsObserver {
					continue
				}
				matched := strings.EqualFold(strings.TrimSpace(a.Name), strings.TrimSpace(target))
				if !matched && parsedIdx > 0 && a.Index == parsedIdx {
					matched = true
				}
				if !matched && strings.Contains(strings.ToLower(target), strings.ToLower(a.Name)) {
					matched = true
				}
				if !matched && strings.Contains(strings.ToLower(a.Name), strings.ToLower(target)) {
					matched = true
				}
				if matched {
					store.Get().RemoveAgent(roomID, a.ID)
					h.mu.Lock()
					if h.recentlyRemovedNames[roomID] == nil {
						h.recentlyRemovedNames[roomID] = map[string]time.Time{}
					}
					h.recentlyRemovedNames[roomID][targetKey] = time.Now()
					h.recentlyRemovedNames[roomID][strings.ToLower(strings.TrimSpace(a.Name))] = time.Now()
					h.mu.Unlock()
					h.Broadcast(roomID, models.WSEvent{Type: models.EventRoomUpdate, Payload: room})
					h.broadcastSysMsg(roomID, fmt.Sprintf("裁判移除机器人：%s(#%d)", a.Name, a.Index), referee.Name)
					removed = true
					break
				}
			}
			if !removed {
				h.mu.Lock()
				recentlyRemovedAt := time.Time{}
				if h.recentlyRemovedNames[roomID] != nil {
					recentlyRemovedAt = h.recentlyRemovedNames[roomID][targetKey]
				}
				h.mu.Unlock()
				if !recentlyRemovedAt.IsZero() && time.Since(recentlyRemovedAt) < 90*time.Second {
					// 同名目标刚被移除，重复 REMOVE 静默忽略
				} else {
					h.broadcastSysMsg(roomID, fmt.Sprintf("裁判移除机器人失败：未找到目标“%s”，未执行兜底删除。", strings.TrimSpace(ops.RemoveTarget)), referee.Name)
				}
			}
		}
	}
	return ops.Directive
}

func (h *Hub) generateObserverSummaries(roomID, cmdText string) {
	if !shouldTriggerObserverSummary(cmdText) {
		return
	}
	room, ok := store.Get().GetRoom(roomID)
	if !ok || cmdText == "" {
		return
	}
	h.mu.Lock()
	if h.lastObserverCmds[roomID] == strings.TrimSpace(cmdText) {
		h.mu.Unlock()
		return
	}
	h.lastObserverCmds[roomID] = strings.TrimSpace(cmdText)
	h.mu.Unlock()
	observers := observerAgents(room.Agents)
	for _, observer := range observers {
		_, err := generateObserverSummary(room, observer, cmdText)
		if err != nil {
			log.Printf("[Engine] Observer %s error: %v", observer.Name, err)
			continue
		}
	}
}
