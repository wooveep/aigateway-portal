package portal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/apperr"
	"higress-portal-backend/internal/model"
)

const (
	chatSessionDefaultTitle   = "新对话"
	chatMessageStatusStreaming = "streaming"
	chatMessageStatusSucceeded = "succeeded"
	chatMessageStatusFailed    = "failed"
	chatMessageStatusCancelled = "cancelled"
)

type chatStreamEmitter interface {
	Context() context.Context
	Emit(event string, payload any) error
}

type chatSessionRow struct {
	SessionID          string     `orm:"session_id"`
	ConsumerName       string     `orm:"consumer_name"`
	OperatorConsumer   string     `orm:"operator_consumer_name"`
	Title              string     `orm:"title"`
	DefaultModelID     string     `orm:"default_model_id"`
	DefaultAPIKeyID    string     `orm:"default_api_key_id"`
	LastMessagePreview string     `orm:"last_message_preview"`
	LastMessageAt      *time.Time `orm:"last_message_at"`
	CreatedAt          time.Time  `orm:"created_at"`
}

type chatMessageRow struct {
	ID           int64      `orm:"id"`
	MessageID    string     `orm:"message_id"`
	SessionID    string     `orm:"session_id"`
	Role         string     `orm:"role"`
	Content      string     `orm:"content"`
	Status       string     `orm:"status"`
	ModelID      string     `orm:"model_id"`
	APIKeyID     string     `orm:"api_key_id"`
	RequestID    string     `orm:"request_id"`
	TraceID      string     `orm:"trace_id"`
	HTTPStatus   int        `orm:"http_status"`
	ErrorMessage string     `orm:"error_message"`
	CreatedAt    time.Time  `orm:"created_at"`
	FinishedAt   *time.Time `orm:"finished_at"`
}

type streamChunkPayload struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Text string `json:"text"`
	} `json:"choices"`
	Output []struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	OutputText string `json:"output_text"`
}

func (s *Service) ListChatSessions(ctx context.Context, consumerName string) ([]model.ChatSessionSummary, error) {
	var rows []chatSessionRow
	err := s.db.GetScan(ctx, &rows, `
		SELECT session_id, consumer_name, operator_consumer_name, title, default_model_id, default_api_key_id,
			last_message_preview, last_message_at, created_at
		FROM portal_ai_chat_session
		WHERE consumer_name = ?
		  AND deleted_at IS NULL
		ORDER BY COALESCE(last_message_at, created_at) DESC, id DESC`, consumerName)
	if err != nil {
		return nil, gerror.Wrap(err, "query chat sessions failed")
	}
	items := make([]model.ChatSessionSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, toChatSessionSummary(row))
	}
	return items, nil
}

func (s *Service) CreateChatSession(ctx context.Context, consumerName string, operatorConsumer string,
	req model.CreateChatSessionRequest,
) (model.ChatSessionSummary, error) {
	sessionID := fmt.Sprintf("CHAT%d", time.Now().UnixNano())
	title := normalizeChatTitle(req.Title)
	now := time.Now().UTC()
	if _, err := s.db.Exec(ctx, `
		INSERT INTO portal_ai_chat_session
		(session_id, consumer_name, operator_consumer_name, title, default_model_id, default_api_key_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID,
		consumerName,
		operatorConsumer,
		title,
		strings.TrimSpace(req.DefaultModelID),
		strings.TrimSpace(req.DefaultAPIKeyID),
		now,
		now,
	); err != nil {
		return model.ChatSessionSummary{}, gerror.Wrap(err, "create chat session failed")
	}
	row, err := s.requireChatSession(ctx, consumerName, sessionID)
	if err != nil {
		return model.ChatSessionSummary{}, err
	}
	return toChatSessionSummary(row), nil
}

func (s *Service) UpdateChatSession(ctx context.Context, consumerName string, sessionID string,
	req model.UpdateChatSessionRequest,
) (model.ChatSessionSummary, error) {
	row, err := s.requireChatSession(ctx, consumerName, sessionID)
	if err != nil {
		return model.ChatSessionSummary{}, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = row.Title
	}
	if title == "" {
		title = chatSessionDefaultTitle
	}
	if _, err = s.db.Exec(ctx, `
		UPDATE portal_ai_chat_session
		SET title = ?, default_model_id = ?, default_api_key_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE session_id = ? AND consumer_name = ? AND deleted_at IS NULL`,
		title,
		strings.TrimSpace(req.DefaultModelID),
		strings.TrimSpace(req.DefaultAPIKeyID),
		sessionID,
		consumerName,
	); err != nil {
		return model.ChatSessionSummary{}, gerror.Wrap(err, "update chat session failed")
	}
	updated, err := s.requireChatSession(ctx, consumerName, sessionID)
	if err != nil {
		return model.ChatSessionSummary{}, err
	}
	return toChatSessionSummary(updated), nil
}

func (s *Service) GetChatSessionDetail(ctx context.Context, consumerName string, sessionID string) (model.ChatSessionDetail, error) {
	row, err := s.requireChatSession(ctx, consumerName, sessionID)
	if err != nil {
		return model.ChatSessionDetail{}, err
	}
	messages, err := s.listChatMessages(ctx, sessionID)
	if err != nil {
		return model.ChatSessionDetail{}, err
	}
	return model.ChatSessionDetail{
		Session:  toChatSessionSummary(row),
		Messages: messages,
	}, nil
}

func (s *Service) DeleteChatSession(ctx context.Context, consumerName string, sessionID string) error {
	if _, err := s.requireChatSession(ctx, consumerName, sessionID); err != nil {
		return err
	}
	now := time.Now().UTC()
	if _, err := s.db.Exec(ctx, `
		UPDATE portal_ai_chat_session
		SET deleted_at = ?, updated_at = ?
		WHERE session_id = ? AND consumer_name = ? AND deleted_at IS NULL`,
		now, now, sessionID, consumerName); err != nil {
		return gerror.Wrap(err, "delete chat session failed")
	}
	if _, err := s.db.Exec(ctx, `
		UPDATE portal_ai_chat_message
		SET deleted_at = ?, updated_at = ?
		WHERE session_id = ? AND deleted_at IS NULL`,
		now, now, sessionID); err != nil {
		return gerror.Wrap(err, "delete chat messages failed")
	}
	return nil
}

func (s *Service) StreamChatMessage(ctx context.Context, consumerName string, sessionID string,
	req model.ChatSendMessageRequest, emitter chatStreamEmitter,
) error {
	session, err := s.requireChatSession(ctx, consumerName, sessionID)
	if err != nil {
		return s.emitChatStreamError(emitter, "", "session_not_found", err.Error())
	}
	content, err := requireNonBlankValue(req.Content, "message content cannot be blank")
	if err != nil {
		return s.emitChatStreamError(emitter, "", "invalid_message", err.Error())
	}
	modelID := strings.TrimSpace(req.ModelID)
	if modelID == "" {
		modelID = strings.TrimSpace(session.DefaultModelID)
	}
	apiKeyID := strings.TrimSpace(req.APIKeyID)
	if apiKeyID == "" {
		apiKeyID = strings.TrimSpace(session.DefaultAPIKeyID)
	}
	modelInfo, err := s.requireVisibleModelForConsumer(ctx, consumerName, modelID)
	if err != nil {
		return s.emitChatStreamError(emitter, "", "model_unavailable", err.Error())
	}
	apiKeyRow, err := s.requireUsableAPIKey(ctx, consumerName, apiKeyID)
	if err != nil {
		return s.emitChatStreamError(emitter, "", "api_key_unavailable", err.Error())
	}

	userMessageID := fmt.Sprintf("MSG%d", time.Now().UnixNano())
	assistantMessageID := fmt.Sprintf("MSG%d", time.Now().UnixNano()+1)
	now := time.Now().UTC()
	if _, err = s.db.Exec(ctx, `
		INSERT INTO portal_ai_chat_message
		(message_id, session_id, role, content, status, model_id, api_key_id, created_at, updated_at)
		VALUES (?, ?, 'user', ?, ?, ?, ?, ?, ?)`,
		userMessageID, sessionID, content, chatMessageStatusSucceeded, modelID, apiKeyID, now, now); err != nil {
		return s.emitChatStreamError(emitter, "", "message_insert_failed", "保存用户消息失败")
	}
	if _, err = s.db.Exec(ctx, `
		INSERT INTO portal_ai_chat_message
		(message_id, session_id, role, content, status, model_id, api_key_id, created_at, updated_at)
		VALUES (?, ?, 'assistant', '', ?, ?, ?, ?, ?)`,
		assistantMessageID, sessionID, chatMessageStatusStreaming, modelID, apiKeyID, now, now); err != nil {
		return s.emitChatStreamError(emitter, "", "message_insert_failed", "创建助手消息失败")
	}
	if err = s.updateChatSessionDefaults(ctx, consumerName, sessionID, modelID, apiKeyID); err != nil {
		return s.emitChatStreamError(emitter, assistantMessageID, "session_update_failed", "更新会话默认配置失败")
	}
	if err = s.refreshChatSessionTitleAndPreview(ctx, consumerName, sessionID, content, ""); err != nil {
		return s.emitChatStreamError(emitter, assistantMessageID, "session_update_failed", "更新会话标题失败")
	}
	if err = emitter.Emit("ack", model.ChatStreamAck{
		UserMessageID:      userMessageID,
		AssistantMessageID: assistantMessageID,
		SessionID:          sessionID,
	}); err != nil {
		return err
	}

	history, err := s.listChatMessagesForUpstream(ctx, sessionID, assistantMessageID)
	if err != nil {
		_ = s.finishAssistantMessage(ctx, assistantMessageID, chatMessageStatusFailed, "", "", 0, "加载会话历史失败")
		return s.emitChatStreamError(emitter, assistantMessageID, "history_load_failed", "加载会话历史失败")
	}

	requestURL := s.buildGatewayURL(modelInfo.Endpoint, "/v1/chat/completions", true)
	payload := map[string]any{
		"model":    modelInfo.ID,
		"messages": history,
		"stream":   true,
	}
	body, _ := json.Marshal(payload)
	upstreamReq, err := http.NewRequestWithContext(emitter.Context(), http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		_ = s.finishAssistantMessage(ctx, assistantMessageID, chatMessageStatusFailed, "", "", 0, "创建上游请求失败")
		return s.emitChatStreamError(emitter, assistantMessageID, "upstream_request_failed", "创建上游请求失败")
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Accept", "text/event-stream")
	upstreamReq.Header.Set("x-api-key", apiKeyRow.RawKey)

	resp, err := s.streamClient.Do(upstreamReq)
	if err != nil {
		status := chatMessageStatusFailed
		message := "调用上游模型失败"
		if emitter.Context().Err() != nil {
			status = chatMessageStatusCancelled
			message = "会话已取消"
		}
		_ = s.finishAssistantMessage(ctx, assistantMessageID, status, "", "", 0, message)
		if status == chatMessageStatusCancelled {
			return nil
		}
		return s.emitChatStreamError(emitter, assistantMessageID, "upstream_request_failed", message)
	}
	defer resp.Body.Close()

	requestID := strings.TrimSpace(resp.Header.Get("X-Request-Id"))
	traceID := strings.TrimSpace(resp.Header.Get("X-B3-TraceId"))
	if traceID == "" {
		traceID = strings.TrimSpace(resp.Header.Get("Traceparent"))
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		message := trimPreview(string(responseBody), 240)
		if message == "" {
			message = fmt.Sprintf("上游返回异常状态码 %d", resp.StatusCode)
		}
		_ = s.finishAssistantMessage(ctx, assistantMessageID, chatMessageStatusFailed, requestID, traceID, resp.StatusCode, message)
		return s.emitChatStreamError(emitter, assistantMessageID, "upstream_http_error", message)
	}

	fullText, err := s.forwardChatStream(ctx, resp.Body, assistantMessageID, emitter)
	if err != nil {
		status := chatMessageStatusFailed
		message := "流式响应解析失败"
		if emitter.Context().Err() != nil {
			status = chatMessageStatusCancelled
			message = "会话已取消"
		}
		_ = s.finishAssistantMessage(ctx, assistantMessageID, status, requestID, traceID, resp.StatusCode, message)
		if status == chatMessageStatusCancelled {
			return nil
		}
		return s.emitChatStreamError(emitter, assistantMessageID, "stream_parse_failed", message)
	}

	if err = s.finishAssistantMessage(ctx, assistantMessageID, chatMessageStatusSucceeded, requestID, traceID,
		resp.StatusCode, "", fullText); err != nil {
		return s.emitChatStreamError(emitter, assistantMessageID, "message_finalize_failed", "保存助手消息失败")
	}
	if err = s.refreshChatSessionTitleAndPreview(ctx, consumerName, sessionID, content, fullText); err != nil {
		return s.emitChatStreamError(emitter, assistantMessageID, "session_update_failed", "更新会话摘要失败")
	}
	return emitter.Emit("done", model.ChatStreamDone{
		AssistantMessageID: assistantMessageID,
		RequestID:          requestID,
		TraceID:            traceID,
		HTTPStatus:         resp.StatusCode,
	})
}

func (s *Service) requireChatSession(ctx context.Context, consumerName string, sessionID string) (chatSessionRow, error) {
	var row chatSessionRow
	record, err := s.db.GetOne(ctx, `
		SELECT session_id, consumer_name, operator_consumer_name, title, default_model_id, default_api_key_id,
			last_message_preview, last_message_at, created_at
		FROM portal_ai_chat_session
		WHERE session_id = ?
		  AND consumer_name = ?
		  AND deleted_at IS NULL
		LIMIT 1`, sessionID, consumerName)
	if err != nil {
		return row, gerror.Wrap(err, "query chat session failed")
	}
	if record.IsEmpty() {
		return row, apperr.New(404, "chat session not found")
	}
	if err = record.Struct(&row); err != nil {
		return row, gerror.Wrap(err, "convert chat session failed")
	}
	return row, nil
}

func (s *Service) listChatMessages(ctx context.Context, sessionID string) ([]model.ChatMessageRecord, error) {
	var rows []chatMessageRow
	err := s.db.GetScan(ctx, &rows, `
		SELECT id, message_id, session_id, role, content, status, model_id, api_key_id, request_id, trace_id,
			http_status, error_message, created_at, finished_at
		FROM portal_ai_chat_message
		WHERE session_id = ?
		  AND deleted_at IS NULL
		ORDER BY created_at ASC, id ASC`, sessionID)
	if err != nil {
		return nil, gerror.Wrap(err, "query chat messages failed")
	}
	items := make([]model.ChatMessageRecord, 0, len(rows))
	for _, row := range rows {
		items = append(items, toChatMessageRecord(row))
	}
	return items, nil
}

func (s *Service) listChatMessagesForUpstream(ctx context.Context, sessionID string,
	excludedAssistantMessageID string,
) ([]map[string]string, error) {
	var rows []chatMessageRow
	err := s.db.GetScan(ctx, &rows, `
		SELECT id, message_id, session_id, role, content, status, model_id, api_key_id, request_id, trace_id,
			http_status, error_message, created_at, finished_at
		FROM portal_ai_chat_message
		WHERE session_id = ?
		  AND deleted_at IS NULL
		  AND message_id <> ?
		  AND status <> 'streaming'
		ORDER BY created_at ASC, id ASC`, sessionID, excludedAssistantMessageID)
	if err != nil {
		return nil, gerror.Wrap(err, "query chat upstream history failed")
	}
	items := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		content := strings.TrimSpace(row.Content)
		if content == "" {
			continue
		}
		items = append(items, map[string]string{
			"role":    row.Role,
			"content": content,
		})
	}
	return items, nil
}

func (s *Service) requireVisibleModelForConsumer(ctx context.Context, consumerName string, modelID string) (model.ModelInfo, error) {
	normalizedModelID := strings.TrimSpace(modelID)
	if normalizedModelID == "" {
		return model.ModelInfo{}, apperr.New(400, "modelId is required")
	}
	user, err := s.resolveScopeUser(ctx, consumerName)
	if err != nil {
		return model.ModelInfo{}, err
	}
	item, err := s.getVisibleModelFromPublishedBindings(ctx, normalizedModelID, user)
	if err != nil {
		return model.ModelInfo{}, err
	}
	if strings.TrimSpace(item.ID) == "" {
		return model.ModelInfo{}, apperr.New(404, "model not found")
	}
	return s.applyModelRequestURL(item), nil
}

func (s *Service) requireUsableAPIKey(ctx context.Context, consumerName string, apiKeyID string) (*model.APIKeyRow, error) {
	normalizedKeyID := strings.TrimSpace(apiKeyID)
	if normalizedKeyID == "" {
		return nil, apperr.New(400, "apiKeyId is required")
	}
	row, err := s.getAPIKeyRow(ctx, consumerName, normalizedKeyID, true)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, apperr.New(404, "api key not found")
	}
	if !strings.EqualFold(row.Status, "active") {
		return nil, apperr.New(403, "api key is disabled")
	}
	if row.DeletedAt != nil {
		return nil, apperr.New(403, "api key is deleted")
	}
	if row.ExpiresAt != nil && row.ExpiresAt.Before(time.Now().UTC()) {
		return nil, apperr.New(403, "api key is expired")
	}
	return row, nil
}

func (s *Service) updateChatSessionDefaults(ctx context.Context, consumerName string, sessionID string,
	defaultModelID string, defaultAPIKeyID string,
) error {
	_, err := s.db.Exec(ctx, `
		UPDATE portal_ai_chat_session
		SET default_model_id = ?, default_api_key_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE session_id = ? AND consumer_name = ? AND deleted_at IS NULL`,
		defaultModelID, defaultAPIKeyID, sessionID, consumerName)
	return wrapInternalError("update chat session defaults failed", err)
}

func (s *Service) refreshChatSessionTitleAndPreview(ctx context.Context, consumerName string, sessionID string,
	userContent string, assistantContent string,
) error {
	session, err := s.requireChatSession(ctx, consumerName, sessionID)
	if err != nil {
		return err
	}
	title := strings.TrimSpace(session.Title)
	if title == "" || title == chatSessionDefaultTitle {
		title = autoChatTitle(userContent)
	}
	preview := trimPreview(strings.TrimSpace(assistantContent), 120)
	if preview == "" {
		preview = trimPreview(strings.TrimSpace(userContent), 120)
	}
	now := time.Now().UTC()
	_, err = s.db.Exec(ctx, `
		UPDATE portal_ai_chat_session
		SET title = ?, last_message_preview = ?, last_message_at = ?, updated_at = ?
		WHERE session_id = ? AND consumer_name = ? AND deleted_at IS NULL`,
		title, preview, now, now, sessionID, consumerName)
	return wrapInternalError("refresh chat session summary failed", err)
}

func (s *Service) finishAssistantMessage(ctx context.Context, assistantMessageID string, status string, requestID string,
	traceID string, httpStatus int, errorMessage string, content ...string,
) error {
	finalContent := ""
	if len(content) > 0 {
		finalContent = content[0]
	}
	now := time.Now().UTC()
	_, err := s.db.Exec(ctx, `
		UPDATE portal_ai_chat_message
		SET content = ?, status = ?, request_id = ?, trace_id = ?, http_status = ?, error_message = ?,
			finished_at = ?, updated_at = ?
		WHERE message_id = ? AND deleted_at IS NULL`,
		finalContent, status, requestID, traceID, httpStatus, errorMessage, now, now, assistantMessageID)
	return wrapInternalError("update assistant chat message failed", err)
}

func (s *Service) forwardChatStream(ctx context.Context, body io.Reader, assistantMessageID string,
	emitter chatStreamEmitter,
) (string, error) {
	reader := bufio.NewReader(body)
	var (
		fullText  string
		dataLines []string
	)
	flushEvent := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		payload := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		if strings.TrimSpace(payload) == "[DONE]" {
			return nil
		}
		delta, nextText, err := parseOpenAIStreamChunk(payload, fullText)
		if err != nil {
			return err
		}
		if delta == "" && nextText == fullText {
			return nil
		}
		fullText = nextText
		return emitter.Emit("delta", model.ChatStreamDelta{
			AssistantMessageID: assistantMessageID,
			Delta:              delta,
			Text:               fullText,
		})
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return fullText, err
		}
		trimmedLine := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(trimmedLine, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(trimmedLine, "data:")))
		}
		if trimmedLine == "" {
			if flushErr := flushEvent(); flushErr != nil {
				return fullText, flushErr
			}
		}
		if err == io.EOF {
			break
		}
		select {
		case <-ctx.Done():
			return fullText, ctx.Err()
		default:
		}
	}
	if err := flushEvent(); err != nil {
		return fullText, err
	}
	return fullText, nil
}

func (s *Service) emitChatStreamError(emitter chatStreamEmitter, assistantMessageID string, code string, message string) error {
	if emitter == nil {
		return nil
	}
	return emitter.Emit("error", model.ChatStreamError{
		AssistantMessageID: assistantMessageID,
		Code:               code,
		Message:            message,
	})
}

func parseOpenAIStreamChunk(raw string, previousText string) (string, string, error) {
	var payload streamChunkPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", previousText, err
	}
	text := ""
	if len(payload.Choices) > 0 {
		choice := payload.Choices[0]
		if choice.Delta.Content != "" {
			text = previousText + choice.Delta.Content
			return choice.Delta.Content, text, nil
		}
		if choice.Message.Content != "" {
			text = choice.Message.Content
		} else if choice.Text != "" {
			text = choice.Text
		}
	}
	if text == "" && payload.OutputText != "" {
		text = payload.OutputText
	}
	if text == "" && len(payload.Output) > 0 && len(payload.Output[0].Content) > 0 {
		text = payload.Output[0].Content[0].Text
	}
	if text == "" {
		return "", previousText, nil
	}
	if strings.HasPrefix(text, previousText) {
		return text[len(previousText):], text, nil
	}
	return text, text, nil
}

func toChatSessionSummary(row chatSessionRow) model.ChatSessionSummary {
	lastMessageAt := ""
	if row.LastMessageAt != nil {
		lastMessageAt = model.NowText(*row.LastMessageAt)
	}
	return model.ChatSessionSummary{
		SessionID:          row.SessionID,
		ConsumerName:       row.ConsumerName,
		Title:              normalizeChatTitle(row.Title),
		DefaultModelID:     row.DefaultModelID,
		DefaultAPIKeyID:    row.DefaultAPIKeyID,
		LastMessagePreview: row.LastMessagePreview,
		LastMessageAt:      lastMessageAt,
		CreatedAt:          model.NowText(row.CreatedAt),
	}
}

func toChatMessageRecord(row chatMessageRow) model.ChatMessageRecord {
	finishedAt := ""
	if row.FinishedAt != nil {
		finishedAt = model.NowText(*row.FinishedAt)
	}
	return model.ChatMessageRecord{
		MessageID:    row.MessageID,
		SessionID:    row.SessionID,
		Role:         row.Role,
		Content:      row.Content,
		Status:       row.Status,
		ModelID:      row.ModelID,
		APIKeyID:     row.APIKeyID,
		RequestID:    row.RequestID,
		TraceID:      row.TraceID,
		HTTPStatus:   row.HTTPStatus,
		ErrorMessage: row.ErrorMessage,
		CreatedAt:    model.NowText(row.CreatedAt),
		FinishedAt:   finishedAt,
	}
}

func normalizeChatTitle(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return chatSessionDefaultTitle
	}
	return normalized
}

func autoChatTitle(userContent string) string {
	title := trimPreview(userContent, 24)
	if title == "" {
		return chatSessionDefaultTitle
	}
	return title
}
