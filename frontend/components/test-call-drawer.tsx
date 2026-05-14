"use client";

import { useEffect, useState } from "react";
import { api, ApiError, Bot, TestCallResponse } from "../lib/api";
import { useTenantScope } from "../lib/auth-context";
import { useT } from "../lib/i18n";
import { useToast } from "./toast";

type Props = {
  open: boolean;
  onClose: () => void;
  defaultPhone?: string;
  defaultLeadName?: string;
  defaultBotId?: string;
  onCallCreated?: (response: TestCallResponse) => void;
};

export function TestCallDrawer({
  open,
  onClose,
  defaultPhone = "",
  defaultLeadName = "",
  defaultBotId = "",
  onCallCreated,
}: Props) {
  const tenant = useTenantScope();
  const t = useT();
  const [phone, setPhone] = useState(defaultPhone);
  const [leadName, setLeadName] = useState(defaultLeadName);
  const [botId, setBotId] = useState(defaultBotId);
  const [bots, setBots] = useState<Bot[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [response, setResponse] = useState<TestCallResponse | null>(null);
  const toast = useToast();

  useEffect(() => {
    if (!open) return;
    setPhone(defaultPhone);
    setLeadName(defaultLeadName);
    setBotId(defaultBotId);
    setResponse(null);
    api.bots(tenant).then(setBots).catch(() => setBots([]));
  }, [open, defaultPhone, defaultLeadName, defaultBotId, tenant]);

  if (!open) return null;

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!phone.trim()) {
      toast.push(t("drawer.testcall.toast.phonerequired"), "warn");
      return;
    }
    setSubmitting(true);
    try {
      const res = await api.testCall({
        phone: phone.trim(),
        leadName: leadName.trim() || undefined,
        botId: botId || undefined,
      });
      setResponse(res);
      onCallCreated?.(res);
      toast.push(
        res.channel
          ? t("drawer.testcall.toast.created.channel", { id: res.channel.id })
          : t("drawer.testcall.toast.created.nochannel"),
        "success"
      );
    } catch (err) {
      const message = err instanceof ApiError ? err.code : "Error";
      toast.push(t("drawer.testcall.toast.failed", { err: message }), "danger");
    } finally {
      setSubmitting(false);
    }
  }

  const botsWithDID = bots.filter((b) => b.didId);
  const botsWithoutDID = bots.filter((b) => !b.didId);
  const selectedBot = bots.find((b) => b.id === botId);

  return (
    <div className="drawer-overlay" role="dialog" aria-modal="true">
      <button className="drawer-backdrop" onClick={onClose} aria-label={t("btn.close")} />
      <aside className="drawer">
        <header className="drawer-header">
          <div>
            <p className="eyebrow">{t("drawer.testcall.eyebrow")}</p>
            <h2>{t("drawer.testcall.title")}</h2>
          </div>
          <button className="button secondary compact" onClick={onClose}>
            {t("btn.close")}
          </button>
        </header>

        <form className="drawer-body" onSubmit={handleSubmit}>
          <div className="field">
            <label htmlFor="phone">{t("drawer.testcall.phone")}</label>
            <input
              id="phone"
              type="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              placeholder={t("drawer.testcall.phone.placeholder")}
              required
            />
          </div>
          <div className="field">
            <label htmlFor="leadName">{t("drawer.testcall.leadname")}</label>
            <input
              id="leadName"
              type="text"
              value={leadName}
              onChange={(e) => setLeadName(e.target.value)}
              placeholder={t("drawer.testcall.leadname.placeholder")}
            />
          </div>
          <div className="field">
            <label htmlFor="bot">{t("drawer.testcall.bot")}</label>
            <select id="bot" value={botId} onChange={(e) => setBotId(e.target.value)}>
              <option value="">{t("drawer.testcall.bot.sandbox")}</option>
              {botsWithDID.length > 0 ? (
                <optgroup label={t("drawer.testcall.bot.withdid")}>
                  {botsWithDID.map((b) => (
                    <option key={b.id} value={b.id}>
                      {b.name} ({b.didE164})
                    </option>
                  ))}
                </optgroup>
              ) : null}
              {botsWithoutDID.length > 0 ? (
                <optgroup label={t("drawer.testcall.bot.nodid")}>
                  {botsWithoutDID.map((b) => (
                    <option key={b.id} value={b.id} disabled>
                      {b.name} — {t("drawer.testcall.bot.nodid.suffix")}
                    </option>
                  ))}
                </optgroup>
              ) : null}
            </select>
            {selectedBot?.didE164 ? (
              <p className="subtle" style={{ marginTop: 4 }}>
                {t("drawer.testcall.bot.willcall")} <code className="mono">{selectedBot.didE164}</code>{" "}
                {t("drawer.testcall.bot.viatrunk")}
              </p>
            ) : (
              <p className="subtle" style={{ marginTop: 4 }}>
                {t("drawer.testcall.bot.usesandbox")}
              </p>
            )}
          </div>

          <button className="button" disabled={submitting}>
            {submitting ? t("drawer.testcall.submitting") : t("drawer.testcall.submit")}
          </button>
        </form>

        {response ? (
          <div className="drawer-result">
            <p className="eyebrow">{t("drawer.testcall.result")}</p>
            <div className="kv">
              <span>{t("drawer.testcall.callid")}</span>
              <strong>{response.call.id}</strong>
            </div>
            <div className="kv">
              <span>{t("drawer.testcall.statuslabel")}</span>
              <strong>{response.call.status}</strong>
            </div>
            {response.channel ? (
              <div className="kv">
                <span>{t("drawer.testcall.channel")}</span>
                <strong>{response.channel.id}</strong>
              </div>
            ) : null}
            {response.endpoint ? (
              <div className="kv">
                <span>{t("drawer.testcall.endpoint")}</span>
                <strong>{response.endpoint}</strong>
              </div>
            ) : null}
            {response.message ? <p className="subtle">{response.message}</p> : null}
          </div>
        ) : null}
      </aside>
    </div>
  );
}
