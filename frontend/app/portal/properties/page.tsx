import { api } from "../../../lib/api";

export default async function PropertiesPage() {
  const properties = await api.properties();

  return (
    <>
      <div className="topbar">
        <div className="page-title">
          <p className="eyebrow">Portal cliente</p>
          <h1>Propiedades</h1>
          <p className="subtle">Conocimiento verificado que el bot puede usar en llamada sin inventar condiciones.</p>
        </div>
        <div className="actions">
          <button className="button secondary">Importar</button>
          <button className="button">Nueva propiedad</button>
        </div>
      </div>

      <div className="grid two">
        {properties.map((property) => (
          <section className="panel" key={property.id}>
            <div className="panel-header">
              <div>
                <p className="eyebrow">{property.address}</p>
                <h2>{property.name}</h2>
              </div>
              <span className="status good">{property.availability}</span>
            </div>
            <p><strong>{property.price}</strong></p>
            <div className="command-strip">
              <div>
                <h3>Requisitos</h3>
                <p className="subtle">{property.requirements.join(", ")}</p>
              </div>
              <div>
                <h3>FAQs</h3>
                <p className="subtle">{property.faqs.join(", ")}</p>
              </div>
            </div>
          </section>
        ))}
      </div>
    </>
  );
}
