package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"timbre/backend/internal/ari"
	"timbre/backend/internal/store"
	"timbre/backend/internal/voiceagent"
)

// MakeARIHandler wires Asterisk Stasis events to the voice-agent. On StasisStart we:
//   1. Look up the call by channel id (set during Originate).
//   2. Ensure the channel is answered.
//   3. Allocate an RTP transport in the voice-agent for the call's voice session.
//   4. Create an External Media channel pointed at that RTP endpoint with format=slin16.
//   5. Open a mixing bridge and add both channels (caller + ExternalMedia).
//
// All errors are logged but never crash the event loop. Failed bridges hang up the inbound
// channel so the caller hears a fast-busy instead of dead silence.
// releaseSlotFn libera el slot del semáforo de la campaña asociada a un callID.
// Se inyecta desde main.go con worker.ReleaseSlot — pasamos la función en vez
// de la dependencia entera para no acoplar app→worker.
type releaseSlotFn func(callID string)

// AudioBridgeMode decide qué mecanismo usa Asterisk para puentear el audio:
//   - "audiosocket": dialplan AudioSocket() sobre TCP (recomendado, sin
//     transcoding en el bridge).
//   - "externalmedia": ARI ExternalMedia sobre RTP/UDP (legacy).
type AudioBridgeMode string

const (
	BridgeModeAudioSocket   AudioBridgeMode = "audiosocket"
	BridgeModeExternalMedia AudioBridgeMode = "externalmedia"
)

// BridgeConfig agrupa los parámetros que el handler necesita para crear el
// canal puente con el voice-agent.
type BridgeConfig struct {
	Mode AudioBridgeMode
	// ExternalMedia path
	ExternalMediaFormat string
	// AudioSocket path
	AudioSocketHost string
	AudioSocketPort string
}

// OnCallFinishedFn se invoca cuando una llamada termina, después de
// persistir el estado final. Usado para disparar webhooks call.completed
// sin acoplar el handler ARI al dispatcher.
type OnCallFinishedFn func(ctx context.Context, callID string)

func MakeARIHandler(
	st *store.Store,
	ariClient *ari.Client,
	va *voiceagent.Client,
	bc BridgeConfig,
	releaseSlot releaseSlotFn,
	onCallFinished OnCallFinishedFn,
	logger *slog.Logger,
) ari.EventHandler {
	if releaseSlot == nil {
		releaseSlot = func(string) {}
	}
	if onCallFinished == nil {
		onCallFinished = func(context.Context, string) {}
	}
	if bc.Mode == "" {
		bc.Mode = BridgeModeAudioSocket
	}
	if bc.ExternalMediaFormat == "" {
		bc.ExternalMediaFormat = "ulaw"
	}
	return func(ctx context.Context, ev ari.Event) {
		switch ev.Type {
		case "StasisStart":
			handleStasisStart(ctx, ev, st, ariClient, va, bc, logger)
		case "ChannelDestroyed":
			handleChannelDestroyed(ctx, ev, st, releaseSlot, onCallFinished, logger)
		case "ChannelStateChange":
			logger.Debug("ari state change", "channel", channelID(ev))
		}
	}
}

func handleStasisStart(
	ctx context.Context,
	ev ari.Event,
	st *store.Store,
	ariClient *ari.Client,
	va *voiceagent.Client,
	bc BridgeConfig,
	logger *slog.Logger,
) {
	chID := channelID(ev)
	if chID == "" {
		return
	}
	// Skip our own side-car channels (ExternalMedia UnicastRTP/ o Local/ del
	// AudioSocket bridge) — disparan StasisStart al ser añadidos al bridge,
	// pero no son llamadas reales y hay que dejarlos vivos. Si los hangup-amos
	// aquí matamos el puente de audio.
	if isSideCarChannel(ev) {
		logger.Info("stasis start side-car", "channel", chID, "name", ev.Channel.Name)
		return
	}

	logger.Info("stasis start", "channel", chID)

	// Find the call we previously originated so we know which tenant/bot to bridge.
	call, err := st.FindCallByChannel(ctx, chID)
	if errors.Is(err, store.ErrNotFound) {
		// No hay call previa → es INBOUND (alguien marcó un DID nuestro).
		// Resolvemos el DID → tenant + bot, creamos la fila call al vuelo,
		// y caemos en el mismo flow de bridging.
		call, err = handleInbound(ctx, ev, st, va, logger)
		if err != nil {
			logger.Warn("inbound setup failed; hanging up", "channel", chID, "error", err)
			_ = ariClient.HangupChannel(ctx, chID)
			return
		}
	} else if err != nil {
		logger.Warn("stasis: lookup call for channel failed", "channel", chID, "error", err)
		_ = ariClient.HangupChannel(ctx, chID)
		return
	}

	// Answer the channel so audio can actually flow.
	if err := ariClient.AnswerChannel(ctx, chID); err != nil {
		logger.Warn("answer channel", "channel", chID, "error", err)
		// Continue anyway — some Asterisk versions already auto-answer in dialplan.
	}

	if va == nil || !va.Enabled() {
		logger.Warn("stasis: voice-agent disabled, dropping call", "channel", chID)
		_ = ariClient.HangupChannel(ctx, chID)
		return
	}
	if call.VoiceSessionID == "" {
		logger.Warn("stasis: call has no voice session", "channel", chID, "call", call.ID)
		_ = ariClient.HangupChannel(ctx, chID)
		return
	}

	// 1+2) Crear el canal "side car" que reproducirá nuestro audio. Dos modos:
	//   - audiosocket: originar Local/<sessionID>@audiosocket-bridge. El
	//     dialplan ejecuta AudioSocket(${EXTEN}, host:port) y abre TCP al
	//     voice-agent. Cero transcoding.
	//   - externalmedia: ARI ExternalMedia (legacy, RTP/UDP). Lo dejamos por
	//     compat pero el path por defecto es audiosocket.
	var emCh ari.Channel
	switch bc.Mode {
	case BridgeModeAudioSocket:
		// Extensión FIJA "audiosocket" (sin patterns); el UUID viaja como
		// channel variable __TIMBRE_AS_UUID con prefijo "__" para que herede
		// al lado ;2 del Local channel (el que ejecuta el dialplan). Antes
		// metíamos el UUID en el EXTEN con pattern _. y Asterisk se quejaba.
		// host:port del voice-agent siguen hardcodeados en el dialplan.
		_ = bc.AudioSocketHost
		_ = bc.AudioSocketPort
		ch, err := ariClient.Originate(ctx, ari.OriginateRequest{
			Endpoint: "Local/audiosocket@audiosocket-bridge",
			Timeout:  10,
			Variables: map[string]string{
				"__TIMBRE_AS_UUID": call.VoiceSessionID,
			},
		})
		if err != nil {
			logger.Error("audiosocket originate", "session", call.VoiceSessionID, "error", err)
			_ = ariClient.HangupChannel(ctx, chID)
			return
		}
		emCh = ch
		logger.Info("audiosocket leg created", "session", call.VoiceSessionID, "channel", ch.ID)
	default:
		rtp, err := va.AllocateRTP(ctx, call.VoiceSessionID)
		if err != nil {
			logger.Error("allocate rtp", "session", call.VoiceSessionID, "error", err)
			_ = ariClient.HangupChannel(ctx, chID)
			return
		}
		logger.Info("rtp allocated", "session", call.VoiceSessionID, "host", rtp.Host, "port", rtp.Port)
		ch, err := ariClient.CreateExternalMedia(ctx, ari.ExternalMediaRequest{
			ExternalHost:   rtp.Host + ":" + itoa(rtp.Port),
			Format:         bc.ExternalMediaFormat,
			Encapsulation:  "rtp",
			Transport:      "udp",
			ConnectionType: "client",
			Direction:      "both",
		})
		if err != nil {
			logger.Error("create external media", "error", err)
			_ = ariClient.HangupChannel(ctx, chID)
			return
		}
		emCh = ch
	}

	// 3) Bridge inbound + ExternalMedia together.
	bridgeID, err := ariClient.CreateBridge(ctx, "mixing")
	if err != nil {
		logger.Error("create bridge", "error", err)
		_ = ariClient.HangupChannel(ctx, chID)
		_ = ariClient.HangupChannel(ctx, emCh.ID)
		return
	}
	if err := ariClient.AddChannelToBridge(ctx, bridgeID, chID); err != nil {
		logger.Error("add caller to bridge", "error", err)
		_ = ariClient.HangupChannel(ctx, chID)
		_ = ariClient.HangupChannel(ctx, emCh.ID)
		return
	}
	if err := ariClient.AddChannelToBridge(ctx, bridgeID, emCh.ID); err != nil {
		logger.Error("add external media to bridge", "error", err)
		_ = ariClient.HangupChannel(ctx, chID)
		_ = ariClient.HangupChannel(ctx, emCh.ID)
		return
	}

	if err := st.MarkCallAnswered(ctx, call.TenantID, call.ID, emCh.ID, bridgeID); err != nil {
		logger.Warn("mark call answered", "error", err)
	}
	logger.Info("stasis bridged", "call", call.ID, "bridge", bridgeID, "external_media", emCh.ID)
}

func handleChannelDestroyed(ctx context.Context, ev ari.Event, st *store.Store, releaseSlot releaseSlotFn, onCallFinished OnCallFinishedFn, logger *slog.Logger) {
	id := channelID(ev)
	dur := extractDuration(ev.Raw)
	cause := extractCause(ev.Raw)
	status := "completed"
	outcome := ""
	low := strings.ToLower(cause)
	switch {
	case strings.Contains(low, "busy"):
		status, outcome = "completed", "busy"
	case strings.Contains(low, "no answer"):
		status, outcome = "completed", "no_answer"
	case strings.Contains(low, "congestion"), strings.Contains(low, "unavailable"):
		status, outcome = "failed", "unreachable"
	}
	// Buscamos el callID asociado al canal ANTES de marcar la llamada finished
	// (que también pone channel_id a "" en algunos paths).
	callID := ""
	if call, err := st.FindCallByChannel(ctx, id); err == nil {
		callID = call.ID
	}
	if err := st.FinishCall(ctx, id, status, outcome, "", dur); err != nil {
		logger.Warn("finish call", "channel", id, "error", err)
	}
	if callID != "" {
		releaseSlot(callID)
		// Notify post-call hooks (webhook dispatcher). Fire-and-forget en
		// goroutine para no bloquear el ARI loop ni el cleanup del canal.
		go onCallFinished(context.Background(), callID)
	}
	logger.Info("channel destroyed", "channel", id, "cause", cause, "duration_sec", dur)
}

// isSideCarChannel detecta canales que NOSOTROS creamos como puente al
// voice-agent — no son llamadas reales del caller, así que el handler de
// StasisStart debe ignorarlos (si los hangup-eamos por error matamos el
// side-car y la llamada se cae).
//
// Dos tipos:
//   - "UnicastRTP/..." → ExternalMedia (path legacy RTP)
//   - "Local/..."     → AudioSocket (path actual)
func isSideCarChannel(ev ari.Event) bool {
	if ev.Channel == nil {
		return false
	}
	n := ev.Channel.Name
	return strings.HasPrefix(n, "UnicastRTP/") || strings.HasPrefix(n, "Local/")
}

func channelID(ev ari.Event) string {
	if ev.Channel != nil {
		return ev.Channel.ID
	}
	return ""
}

func extractDuration(raw []byte) int {
	var probe struct {
		Channel struct {
			Creationtime string `json:"creationtime"`
		} `json:"channel"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return 0
	}
	start, err1 := time.Parse(time.RFC3339, probe.Channel.Creationtime)
	end, err2 := time.Parse(time.RFC3339, probe.Timestamp)
	if err1 != nil || err2 != nil {
		return 0
	}
	d := int(end.Sub(start).Seconds())
	if d < 0 {
		return 0
	}
	return d
}

func extractCause(raw []byte) string {
	var probe struct {
		CauseTxt string `json:"cause_txt"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	return probe.CauseTxt
}

// handleInbound recibe un StasisStart sin call previa (la llamada
// entrante no fue originada por nosotros). Resuelve el DID destino a
// un tenant + bot, crea la fila `call` y la sesión voice-agent, y
// devuelve la call lista para el flow normal de bridging.
//
// Errores devueltos:
//   - ErrNoExten            → el StasisStart no trae exten; dialplan mal configurado.
//   - store.ErrNotFound     → el DID marcado no existe en la BD o no tiene tenant.
//   - ErrNoBotAssigned      → el DID existe pero ningún bot lo tiene asignado.
//   - errores de voice-agent → no se pudo crear sesión.
//
// El caller hace hangup en cualquier caso de error.
func handleInbound(
	ctx context.Context,
	ev ari.Event,
	st *store.Store,
	va *voiceagent.Client,
	logger *slog.Logger,
) (store.Call, error) {
	details := extractStasisStart(ev.Raw)
	if details.Exten == "" {
		return store.Call{}, ErrNoExten
	}
	logger = logger.With(
		"caller", details.CallerNumber,
		"exten", details.Exten,
		"direction", "inbound",
	)

	// 1) Resolver DID → tenant (sin escoger bot aún).
	route, err := st.LookupInboundRoute(ctx, details.Exten)
	if err != nil {
		return store.Call{}, fmt.Errorf("lookup inbound route: %w", err)
	}
	logger = logger.With("tenant", route.TenantID, "did", route.DIDID)

	// 2) Evaluar reglas de routing del DID. La decisión puede ser:
	//    - matched_rule: regla activa matchea (días, horario, prefijo
	//      del caller, idioma) → usa su target_bot_id.
	//    - default_did_bot: ninguna regla matchea PERO el DID tiene bot
	//      asignado via bots.did_id → usa ese (compat con inbound v1).
	//    - no_route: ni reglas ni bot default → rechazamos (típico:
	//      "fuera de horario y sin fallback configurado").
	decision, err := st.ResolveDIDRouting(ctx, route.TenantID, route.DIDID, details.CallerNumber, "", time.Time{})
	if err != nil {
		return store.Call{}, fmt.Errorf("resolve did routing: %w", err)
	}
	if decision.BotID == "" {
		logger.Info("inbound: no matching route, hanging up", "reason", decision.Reason)
		return store.Call{}, ErrNoBotAssigned
	}
	logger = logger.With("bot", decision.BotID, "routing_reason", decision.Reason)
	if decision.MatchedRule != "" {
		logger.Info("inbound: matched routing rule", "rule", decision.MatchedRule)
	}

	bot, err := st.GetBot(ctx, route.TenantID, decision.BotID)
	if err != nil {
		return store.Call{}, fmt.Errorf("get bot: %w", err)
	}

	// 2) Si el caller está bloqueado por DNC, rechazamos. El operador del
	//    tenant decidió que ese número no debe ser contactado — aplica
	//    tanto outbound como inbound (no queremos hablar con él).
	if details.CallerNumber != "" {
		if blocked, err := st.IsBlockedPhone(ctx, route.TenantID, details.CallerNumber); err == nil && blocked {
			return store.Call{}, fmt.Errorf("caller %s in DNC list", details.CallerNumber)
		}
	}

	// 3) Lead lookup/create — si el caller ya existe como lead reusamos
	//    su id (mantenemos historial). Si no, creamos uno "inbound"
	//    minimal para que la call tenga lead asociado.
	var leadID *string
	leadName := ""
	if details.CallerNumber != "" {
		if existing, err := st.FindLeadByPhone(ctx, route.TenantID, details.CallerNumber); err == nil {
			leadID = &existing.ID
			leadName = existing.Name
		} else {
			created, err := st.CreateLead(ctx, store.Lead{
				TenantID: route.TenantID,
				Name:     "Inbound " + details.CallerNumber,
				Phone:    details.CallerNumber,
				Type:     "renter",
				Status:   "new",
				Source:   "inbound_call",
				Consent:  "implicit_inbound",
			})
			if err == nil {
				leadID = &created.ID
				leadName = created.Name
				logger.Info("inbound: created lead", "lead", created.ID)
			} else {
				logger.Warn("inbound: lead create failed; continuing without", "error", err)
			}
		}
	}

	chID := channelID(ev)

	// 4) Crear la fila call. status="answered" porque Asterisk ya la
	//    contestó al pasar a Stasis (el dialplan típicamente ejecuta
	//    Answer antes de Stasis para llamadas inbound).
	now := time.Now().UTC()
	provider := bot.VoiceProvider
	if provider == "" {
		provider = "echo"
	}
	call, err := st.CreateCall(ctx, store.Call{
		TenantID:  route.TenantID,
		LeadID:    leadID,
		LeadName:  leadName,
		Campaign:  "Inbound",
		Phone:     details.CallerNumber,
		Status:    "answered",
		Outcome:   "pending",
		ChannelID: chID,
		StartedAt: &now,
		Summary:   "Inbound call answered by " + bot.Name + " (routing: " + decision.Reason + ").",
		Provider:  provider,
	})
	if err != nil {
		return store.Call{}, fmt.Errorf("create inbound call: %w", err)
	}
	logger.Info("inbound: call created", "call", call.ID)

	// 5) Voice-agent session — mismo flow que el dialer outbound.
	if va == nil || !va.Enabled() {
		return call, fmt.Errorf("voice-agent disabled")
	}
	creds := loadTenantVoiceCreds(ctx, st, route.TenantID, logger)
	// El agent_id de ElevenLabs es per-bot, no per-tenant — lo añadimos
	// aquí tras cargar las credenciales generales del tenant.
	creds.ElevenLabsAgentID = bot.ElevenLabsAgentID
	tools := loadBotTools(ctx, st, route.TenantID, decision.BotID, logger)
	vctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	sess, err := va.CreateSession(vctx, voiceagent.Config{
		CallID:      call.ID,
		TenantID:    route.TenantID,
		BotID:       decision.BotID,
		Provider:    provider,
		Objective:   amdAugmentObjective(bot.Objective, bot.AMDEnabled, bot.AMDAction, bot.VoicemailMessage),
		Guardrails:  bot.Guardrails,
		Language:    bot.Language,
		Voice:       bot.Voice,
		LeadName:    leadName,
		Credentials: creds,
		Tools:       tools,
		AMD: voiceagent.AMDConfig{
			Enabled: bot.AMDEnabled,
			Action:  bot.AMDAction,
			Message: bot.VoicemailMessage,
		},
	})
	if err != nil {
		return call, fmt.Errorf("voice-agent create session: %w", err)
	}
	if err := st.SetCallVoiceSession(ctx, route.TenantID, call.ID, sess.ID); err != nil {
		logger.Warn("inbound: persist voice session failed", "error", err)
	}
	call.VoiceSessionID = sess.ID
	return call, nil
}

// ErrNoExten — el StasisStart no traía exten. Indica dialplan mal
// configurado: el contexto from-pstn debe ejecutar Stasis(app, ...) con
// el exten ya resuelto al DID marcado.
var ErrNoExten = errors.New("stasis start without dialplan exten")

// ErrNoBotAssigned — el DID existe pero ningún bot lo tiene como
// did_id. La inbound se rechaza con fast-busy hasta que el operador
// asigne un bot al DID en el panel.
var ErrNoBotAssigned = errors.New("DID has no bot assigned")

// loadTenantVoiceCreds carga las credenciales del tenant para los
// providers de voz. Extraído para reusarlo entre outbound y inbound;
// hoy el dialer hace lo mismo inline.
func loadTenantVoiceCreds(ctx context.Context, st *store.Store, tenantID string, logger *slog.Logger) voiceagent.Credentials {
	vc, err := st.GetVoiceCredentials(ctx, tenantID)
	if err != nil {
		logger.Warn("inbound: voice credentials lookup failed", "error", err)
		return voiceagent.Credentials{}
	}
	return voiceagent.Credentials{
		OpenAIAPIKey:          vc.OpenAIAPIKey,
		OpenAIRealtimeModel:   vc.OpenAIRealtimeModel,
		OpenAIRealtimeVoice:   vc.OpenAIRealtimeVoice,
		DeepgramAPIKey:        vc.DeepgramAPIKey,
		DeepgramListenModel:   vc.DeepgramListenModel,
		DeepgramThinkProvider: vc.DeepgramThinkProvider,
		DeepgramThinkModel:    vc.DeepgramThinkModel,
		DeepgramSpeakModel:    vc.DeepgramSpeakModel,
		DeepgramGreeting:      vc.DeepgramGreeting,
		AssemblyAIAPIKey:      vc.AssemblyAIAPIKey,
		AssemblyAIVoice:       vc.AssemblyAIVoice,
		AssemblyAIGreeting:    vc.AssemblyAIGreeting,
		ElevenLabsAPIKey:      vc.ElevenLabsAPIKey,
		// ElevenLabsAgentID se setea por separado tras este return,
		// con el agent_id del bot resuelto en el caller.
	}
}

// loadBotTools carga las tools enabled del bot. Misma rutina que el
// dialer outbound; centralizar evita drift entre los dos paths.
func loadBotTools(ctx context.Context, st *store.Store, tenantID, botID string, logger *slog.Logger) []voiceagent.Tool {
	rows, err := st.ListBotTools(ctx, tenantID, botID, true)
	if err != nil {
		logger.Warn("inbound: list bot tools failed", "error", err)
		return nil
	}
	tools := make([]voiceagent.Tool, 0, len(rows))
	for _, t := range rows {
		tools = append(tools, voiceagent.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.ParametersSchema,
		})
	}
	return tools
}

// stasisStartDetails extrae los campos que necesitamos para enrutar
// inbound calls desde el StasisStart event. ARI ya nos da el Channel
// con su Caller (número de origen) y Dialplan.Exten (número marcado).
// args son los strings que el dialplan pasa al Stasis() — los usamos
// para distinguir inbound vs outbound explícitamente.
type stasisStartDetails struct {
	CallerNumber string
	Exten        string
	Args         []string
}

func extractStasisStart(raw []byte) stasisStartDetails {
	var probe struct {
		Channel struct {
			Caller struct {
				Number string `json:"number"`
				Name   string `json:"name"`
			} `json:"caller"`
			Dialplan struct {
				Context string `json:"context"`
				Exten   string `json:"exten"`
			} `json:"dialplan"`
		} `json:"channel"`
		Args []string `json:"args"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return stasisStartDetails{}
	}
	return stasisStartDetails{
		CallerNumber: probe.Channel.Caller.Number,
		Exten:        probe.Channel.Dialplan.Exten,
		Args:         probe.Args,
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
