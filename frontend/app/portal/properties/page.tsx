import { api } from "../../../lib/api";

export default async function PropertiesPage() {
  const properties = await api.properties();

  return (
    <>
      <div className="topbar">
        <div>
          <p className="eyebrow">Portal cliente</p>
          <h1>Propiedades</h1>
          <p className="subtle">Conocimiento verificado que el bot puede usar en llamada.</p>
        </div>
        <button className="button">Nueva propiedad</button>
      </div>
      <div className="grid two">
        {properties.map((property) => (
          <section className="panel" key={property.id}>
            <p className="eyebrow">{property.address}</p>
            <h2>{property.name}</h2>
            <p><strong>{property.price}</strong> · {property.availability}</p>
            <p className="subtle">Requisitos: {property.requirements.join(", ")}</p>
            <p className="subtle">FAQs: {property.faqs.join(", ")}</p>
          </section>
        ))}
      </div>
    </>
  );
}

