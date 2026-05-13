export default function SettingsPage() {
  return (
    <>
      <div className="topbar">
        <div>
          <p className="eyebrow">Portal cliente</p>
          <h1>Configuracion</h1>
          <p className="subtle">Parametros de cuenta, telefonia, integraciones y cumplimiento.</p>
        </div>
        <button className="button">Guardar</button>
      </div>
      <section className="panel">
        <div className="form-grid">
          <div className="field">
            <label>Zona horaria</label>
            <select defaultValue="America/New_York">
              <option value="America/New_York">America/New_York</option>
              <option value="Europe/Madrid">Europe/Madrid</option>
            </select>
          </div>
          <div className="field">
            <label>Caller ID principal</label>
            <input defaultValue="+1 555 0199" />
          </div>
          <div className="field">
            <label>Horario permitido</label>
            <input defaultValue="Lunes a Viernes, 10:00-18:00" />
          </div>
          <div className="field">
            <label>Limite diario de llamadas</label>
            <input defaultValue="250" />
          </div>
        </div>
      </section>
    </>
  );
}

