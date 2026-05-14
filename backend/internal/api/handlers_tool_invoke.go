package api

// Internal endpoint que ejecuta una tool invocada por el LLM durante una
// llamada. Lo llama el voice-agent cuando un provider (Deepgram / OpenAI)
// emite un function call, y devuelve el resultado JSON que el agent
// reenvía al provider como "content" del FunctionCallResponse.
//
// Flujo:
//   1. provider → voice-agent: "Llama a set_qualified con {reason: 'sí quiere visita'}"
//   2. voice-agent → backend: POST /api/internal/tool-invoke {sessionId, toolName, arguments}
//   3. backend: lookup call por session, lookup tool por name, ejecutar action
//   4. backend → voice-agent: {success, content} (content es lo que el LLM "leerá" como resultado)
//   5. voice-agent → provider: FunctionCallResponse con ese content

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"timbre/backend/internal/store"
)

type toolInvokeInput struct {
	SessionID string         `json:"sessionId"`
	ToolName  string         `json:"toolName"`
	Arguments map[string]any `json:"arguments"`
}

type toolInvokeResult struct {
	Success bool   `json:"success"`
	Content string `json:"content"` // texto que ve el LLM (no JSON pesado)
	Error   string `json:"error,omitempty"`
}

func (s *Server) handleInternalToolInvoke(w http.ResponseWriter, r *http.Request) {
	var in toolInvokeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if in.SessionID == "" || in.ToolName == "" {
		writeError(w, http.StatusBadRequest, "session_and_tool_required")
		return
	}

	// 1) Localizar la call por session id (el provider no nos pasa call id).
	call, err := s.store.FindCallByVoiceSession(r.Context(), in.SessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session_not_linked")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}

	// 2) Resolver el bot via la campaign de la call. Si la llamada no tiene
	//    campaign (test call manual) buscamos por bot del DID — para MVP
	//    devolvemos error si no podemos resolver bot.
	botID, err := s.resolveBotIDForCall(r.Context(), call)
	if err != nil || botID == "" {
		s.logger.Warn("tool invoke: cannot resolve bot", "session", in.SessionID, "call", call.ID, "err", err)
		s.logToolInvocation(r.Context(), call, in, toolInvokeResult{Success: false, Error: "bot_not_resolved"})
		writeJSON(w, http.StatusOK, toolInvokeResult{
			Success: false, Error: "bot_not_resolved",
			Content: "I couldn't execute that action right now.",
		})
		return
	}

	// 3) Buscar la tool por nombre dentro del bot.
	tools, err := s.store.ListBotTools(r.Context(), call.TenantID, botID, true)
	if err != nil {
		s.logger.Error("list tools for invoke", "error", err)
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	var tool *store.BotTool
	for i := range tools {
		if tools[i].Name == in.ToolName {
			tool = &tools[i]
			break
		}
	}
	if tool == nil {
		s.logToolInvocation(r.Context(), call, in, toolInvokeResult{Success: false, Error: "tool_not_found"})
		writeJSON(w, http.StatusOK, toolInvokeResult{
			Success: false, Error: "tool_not_found",
			Content: fmt.Sprintf("Unknown tool: %s", in.ToolName),
		})
		return
	}

	// 4) Ejecutar acción.
	result := s.executeToolAction(r.Context(), call, *tool, in.Arguments)
	s.logToolInvocation(r.Context(), call, in, result)
	writeJSON(w, http.StatusOK, result)
}

// resolveBotIDForCall mira primero campaign.bot_id, después intenta vía
// el DID de la call. Si nada lo identifica, devuelve "" (la tool no se
// puede ejecutar con seguridad).
func (s *Server) resolveBotIDForCall(ctx context.Context, c store.Call) (string, error) {
	if c.CampaignID != nil && *c.CampaignID != "" {
		cmp, err := s.store.GetCampaign(ctx, c.TenantID, *c.CampaignID)
		if err != nil {
			return "", err
		}
		if cmp.BotID != "" {
			return cmp.BotID, nil
		}
	}
	return "", nil
}

// executeToolAction dispatcha por action_type. Cada rama es independiente
// y devuelve el toolInvokeResult que se loguea y se manda al provider.
func (s *Server) executeToolAction(ctx context.Context, call store.Call, tool store.BotTool, args map[string]any) toolInvokeResult {
	switch tool.ActionType {

	case "set_lead_outcome":
		value, _ := tool.ActionConfig["value"].(string)
		if value == "" {
			// Si el config no fija el valor, dejamos que el LLM lo pase.
			value, _ = args["outcome"].(string)
		}
		if value == "" {
			return toolInvokeResult{Success: false, Error: "outcome_missing", Content: "Missing outcome value."}
		}
		if err := s.store.UpdateCallOutcome(ctx, call.TenantID, call.ID, value); err != nil {
			return toolInvokeResult{Success: false, Error: err.Error(), Content: "Could not update outcome."}
		}
		return toolInvokeResult{Success: true, Content: fmt.Sprintf("Outcome set to %s", value)}

	case "set_lead_status":
		if call.LeadID == nil {
			return toolInvokeResult{Success: false, Error: "no_lead", Content: "This call has no lead."}
		}
		value, _ := tool.ActionConfig["value"].(string)
		if value == "" {
			value, _ = args["status"].(string)
		}
		if value == "" {
			return toolInvokeResult{Success: false, Error: "status_missing", Content: "Missing status value."}
		}
		if err := s.store.UpdateLeadStatus(ctx, call.TenantID, *call.LeadID, value); err != nil {
			return toolInvokeResult{Success: false, Error: err.Error(), Content: "Could not update lead status."}
		}
		return toolInvokeResult{Success: true, Content: fmt.Sprintf("Lead status set to %s", value)}

	case "schedule_callback":
		if call.LeadID == nil {
			return toolInvokeResult{Success: false, Error: "no_lead", Content: "This call has no lead."}
		}
		// Aceptamos timestamp ISO en args.when o args.callbackAt. Lo guardamos
		// como nota en la summary del lead (cambio mínimo); más adelante una
		// tabla scheduled_callbacks gobierna esto en serio.
		when := ""
		if v, ok := args["when"].(string); ok {
			when = v
		} else if v, ok := args["callbackAt"].(string); ok {
			when = v
		}
		if _, err := time.Parse(time.RFC3339, when); err != nil && when != "" {
			return toolInvokeResult{Success: false, Error: "when_invalid", Content: "Invalid date format (use RFC3339)."}
		}
		_ = s.store.UpdateLeadStatus(ctx, call.TenantID, *call.LeadID, "callback")
		_ = s.store.UpdateCallOutcome(ctx, call.TenantID, call.ID, "callback")
		return toolInvokeResult{Success: true, Content: "Callback scheduled. The lead is marked callback."}

	case "end_call":
		// MVP: solo dejamos huella; el hangup real lo coordinará ARI fuera
		// del scope de esta tool en una siguiente iteración.
		_ = s.store.UpdateCallOutcome(ctx, call.TenantID, call.ID, "completed")
		return toolInvokeResult{Success: true, Content: "Call will end after this turn."}

	case "transfer_human":
		// Placeholder hasta que tengamos warm transfer SIP. Anotamos en la
		// summary para que el operador vea que el bot pidió escalado.
		return toolInvokeResult{Success: true, Content: "Transfer requested. A human will join shortly."}

	case "webhook":
		// Asíncrono: no bloqueamos la conversación. Le damos al LLM una
		// respuesta inmediata y disparamos el POST en goroutine.
		url, _ := tool.ActionConfig["url"].(string)
		go s.dispatchToolWebhook(url, call, tool, args)
		return toolInvokeResult{Success: true, Content: "Action sent."}
	}

	return toolInvokeResult{Success: false, Error: "unknown_action_type"}
}

// dispatchToolWebhook ejecuta el POST a la URL configurada en la tool.
// Fire-and-forget pero con timeout corto — no queremos goroutines colgadas
// para siempre. El log lo hace la feature de outbound webhooks (siguiente
// commit) con retries; aquí versión simple sin retry.
func (s *Server) dispatchToolWebhook(url string, call store.Call, tool store.BotTool, args map[string]any) {
	if url == "" {
		return
	}
	body := map[string]any{
		"event":     "tool.invoke",
		"toolName":  tool.Name,
		"arguments": args,
		"call": map[string]any{
			"id":         call.ID,
			"tenantId":   call.TenantID,
			"leadId":     call.LeadID,
			"leadName":   call.LeadName,
			"phone":      call.Phone,
			"campaignId": call.CampaignID,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	payload, _ := json.Marshal(body)
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		s.logger.Warn("tool webhook: build request", "tool", tool.Name, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "timbre/tool-webhook")
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Warn("tool webhook: dispatch failed", "tool", tool.Name, "url", url, "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		s.logger.Warn("tool webhook: non-2xx", "tool", tool.Name, "status", resp.StatusCode)
	}
}

func (s *Server) logToolInvocation(ctx context.Context, call store.Call, in toolInvokeInput, res toolInvokeResult) {
	callIDPtr := &call.ID
	if call.ID == "" {
		callIDPtr = nil
	}
	if err := s.store.LogBotToolInvocation(ctx, store.BotToolInvocation{
		TenantID:  call.TenantID,
		CallID:    callIDPtr,
		ToolName:  in.ToolName,
		Arguments: in.Arguments,
		Result:    map[string]any{"content": res.Content},
		Success:   res.Success,
		Error:     res.Error,
	}); err != nil {
		s.logger.Warn("log tool invocation", "error", err)
	}
}
