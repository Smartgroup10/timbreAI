import { api, statusClass } from "../../../lib/api";

export default async function BotsPage() {
  const bots = await api.bots();

  return (
    <>
      <div className="topbar">
        <div>
          <p className="eyebrow">Portal cliente</p>
          <h1>Bots</h1>
          <p className="subtle">Identidad, objetivos, voz y reglas de cada asistente.</p>
        </div>
        <button className="button">Crear bot</button>
      </div>
      <div className="grid two">
        {bots.map((bot) => (
          <section className="panel" key={bot.id}>
            <p className="eyebrow">{bot.type}</p>
            <h2>{bot.name}</h2>
            <p><span className={statusClass(bot.status)}>{bot.status}</span></p>
            <p className="subtle">{bot.objective}</p>
            <p>Idioma: {bot.language} · Voz: {bot.voice}</p>
            <p className="subtle">Reglas: {bot.guardrails.join(", ")}</p>
          </section>
        ))}
      </div>
    </>
  );
}

