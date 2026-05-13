import { api, statusClass } from "../../../lib/api";

export default async function BotsPage() {
  const bots = await api.bots();

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Bots</h1>
          <p className="subtle">Identidad, objetivo, voz y reglas de seguridad de cada asistente.</p>
        </div>
        <div className="actions">
          <button className="button secondary">Simular chat</button>
          <button className="button">Crear bot</button>
        </div>
      </div>

      <div className="grid two">
        {bots.map((bot) => (
          <section className="panel" key={bot.id}>
            <div className="panel-header">
              <div>
                <p className="eyebrow">{bot.type}</p>
                <h2>{bot.name}</h2>
              </div>
              <span className={statusClass(bot.status)}>{bot.status}</span>
            </div>
            <p className="subtle">{bot.objective}</p>
            <div className="filter-row">
              <span className="chip">Idioma: {bot.language}</span>
              <span className="chip">Voz: {bot.voice}</span>
            </div>
            <div className="command-strip">
              {bot.guardrails.map((rule) => (
                <div className="command-row" key={rule}>
                  <span>{rule}</span>
                  <span className="status good">Regla</span>
                </div>
              ))}
            </div>
          </section>
        ))}
      </div>
    </>
  );
}
