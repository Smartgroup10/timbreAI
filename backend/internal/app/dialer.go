// Package app contiene la lógica reusable que sirve tanto al handler de test
// call manual como al worker de campañas. Si quieres saber qué pasa cuando se
// origina una llamada, este es el sitio.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"timbre/backend/internal/ari"
	"timbre/backend/internal/config"
	"timbre/backend/internal/store"
	"timbre/backend/internal/voiceagent"
)

// DialDeps son las dependencias compartidas entre test-call y worker.
type DialDeps struct {
	Store      *store.Store
	ARI        *ari.Client
	VoiceAgent *voiceagent.Client
	Cfg        config.Config
	Logger     *slog.Logger
}

// DialResult es lo que un caller (handler o worker) necesita saber tras
// originar una llamada exitosamente.
type DialResult struct {
	ChannelID      string
	VoiceSessionID string
	Endpoint       string
	CallerID       string
	DIDID          string
}

// DialCall ejecuta la marcación completa para un Call ya creado en BD:
//   - Si hay BotID asociado: resuelve bot + DID, crea sesión de voz, origina
//     vía el trunk del DID.
//   - Si no hay bot: origina contra SIP_TEST_EXTENSION (sandbox interno).
//
// Actualiza calls.channel_id y calls.voice_session_id en BD. NO maneja la
// concurrencia — eso es responsabilidad del worker (semáforo).
func DialCall(ctx context.Context, d DialDeps, call store.Call, botID string) (DialResult, error) {
	res := DialResult{}
	if d.ARI == nil {
		return res, errors.New("ari_disabled")
	}

	endpoint := d.Cfg.SIP.TestExtension
	callerID := d.Cfg.SIP.CallerID
	var bot store.Bot

	if botID != "" {
		b, err := d.Store.GetBotByID(ctx, botID)
		if err != nil {
			return res, fmt.Errorf("get bot: %w", err)
		}
		bot = b
		did, err := d.Store.LookupDIDForBot(ctx, call.TenantID, botID)
		if err != nil {
			return res, fmt.Errorf("lookup did: %w", err)
		}
		endpoint = "PJSIP/" + call.Phone + "@" + did.AsteriskEndpoint
		if did.Label != "" {
			callerID = did.Label
		} else {
			callerID = "timbre.ai <" + did.E164 + ">"
		}
		res.DIDID = did.ID
	}
	res.Endpoint = endpoint
	res.CallerID = callerID

	// Voice-agent session. Si el bot tiene provider configurado y hay creds,
	// arrancamos sesión. Si no, fallback a echo para que el bridge no muera.
	if d.VoiceAgent != nil && d.VoiceAgent.Enabled() && botID != "" {
		provider := bot.VoiceProvider
		if provider == "" {
			provider = "echo"
		}
		var creds voiceagent.Credentials
		if vc, err := d.Store.GetVoiceCredentials(ctx, call.TenantID); err == nil {
			creds = voiceagent.Credentials{
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
			}
		}
		hasCreds := true
		switch provider {
		case "openai_realtime":
			hasCreds = creds.OpenAIAPIKey != ""
		case "deepgram":
			hasCreds = creds.DeepgramAPIKey != ""
		case "assemblyai":
			hasCreds = creds.AssemblyAIAPIKey != ""
		}
		if !hasCreds {
			d.Logger.Warn("dial: no creds for provider, fallback echo", "bot", botID, "provider", provider)
			provider = "echo"
		}

		vctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		sess, err := d.VoiceAgent.CreateSession(vctx, voiceagent.Config{
			CallID:      call.ID,
			TenantID:    call.TenantID,
			BotID:       botID,
			Provider:    provider,
			Objective:   bot.Objective,
			Guardrails:  bot.Guardrails,
			Language:    bot.Language,
			Voice:       bot.Voice,
			LeadName:    call.LeadName,
			Credentials: creds,
		})
		cancel()
		if err != nil {
			d.Logger.Warn("voice-agent session create", "error", err, "call", call.ID)
		} else {
			_ = d.Store.SetCallVoiceSession(ctx, call.TenantID, call.ID, sess.ID)
			res.VoiceSessionID = sess.ID
		}
	}

	// Originate.
	octx, cancel := context.WithTimeout(ctx, d.Cfg.SIP.OriginateTimeout)
	defer cancel()
	ch, err := d.ARI.Originate(octx, ari.OriginateRequest{
		Endpoint: endpoint,
		AppArgs:  call.ID + "," + call.TenantID,
		CallerID: callerID,
		Timeout:  int(d.Cfg.SIP.OriginateTimeout.Seconds()),
		Variables: map[string]string{
			"TIMBRE_CALL_ID": call.ID,
			"TIMBRE_TENANT":  call.TenantID,
			"TIMBRE_BOT":     botID,
			"TIMBRE_DID":     res.DIDID,
		},
	})
	if err != nil {
		return res, fmt.Errorf("ari originate: %w", err)
	}
	res.ChannelID = ch.ID
	if err := d.Store.UpdateCallChannel(ctx, call.TenantID, call.ID, ch.ID, "dialing"); err != nil && !errors.Is(err, store.ErrNotFound) {
		d.Logger.Warn("update call channel", "error", err, "call", call.ID)
	}
	return res, nil
}
