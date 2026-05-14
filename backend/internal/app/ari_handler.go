package app

import (
	"context"
	"encoding/json"
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

func MakeARIHandler(
	st *store.Store,
	ariClient *ari.Client,
	va *voiceagent.Client,
	bc BridgeConfig,
	releaseSlot releaseSlotFn,
	logger *slog.Logger,
) ari.EventHandler {
	if releaseSlot == nil {
		releaseSlot = func(string) {}
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
			handleChannelDestroyed(ctx, ev, st, releaseSlot, logger)
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
	// Skip ExternalMedia channels — they fire their own StasisStart we don't act on.
	if isExternalMediaChannel(ev) {
		logger.Info("stasis start external media", "channel", chID)
		return
	}

	logger.Info("stasis start", "channel", chID)

	// Find the call we previously originated so we know which tenant/bot to bridge.
	call, err := st.FindCallByChannel(ctx, chID)
	if err != nil {
		logger.Warn("stasis: no call for channel; hanging up", "channel", chID, "error", err)
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

func handleChannelDestroyed(ctx context.Context, ev ari.Event, st *store.Store, releaseSlot releaseSlotFn, logger *slog.Logger) {
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
	}
	logger.Info("channel destroyed", "channel", id, "cause", cause, "duration_sec", dur)
}

func isExternalMediaChannel(ev ari.Event) bool {
	if ev.Channel == nil {
		return false
	}
	return strings.HasPrefix(ev.Channel.Name, "UnicastRTP/")
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
