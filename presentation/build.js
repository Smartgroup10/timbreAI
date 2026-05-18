// Presentación de cliente de timbre.ai.
// Estilo coherente con la app real: paleta paper / ink / coral.

const pptxgen = require("pptxgenjs");
const React = require("react");
const ReactDOMServer = require("react-dom/server");
const sharp = require("sharp");

const {
  FaPhoneAlt,
  FaUsers,
  FaRobot,
  FaCalendarAlt,
  FaShieldAlt,
  FaMicrophone,
  FaCodeBranch,
  FaBookOpen,
  FaBolt,
  FaChartLine,
  FaCheckCircle,
  FaArrowRight,
  FaPhoneVolume,
  FaPlug,
  FaWaveSquare,
  FaServer,
  FaSearch,
  FaCog,
  FaListUl,
  FaDollarSign,
  FaClock,
  FaRegSnowflake,
  FaTools,
  FaCloudUploadAlt,
  FaShareAlt,
  FaUserShield,
  FaEye,
  FaInbox,
  FaPaperPlane,
  FaQuoteLeft,
  FaLayerGroup,
} = require("react-icons/fa");

// ─── Paleta — extraída de globals.css del app ──────────────────────────
const COLOR = {
  paper: "F5F3EE",
  paper2: "DFD9CD",
  paper3: "EBE7DF",
  surface: "FBFAF6",
  ink: "15171A",
  inkSoft: "2A2D33",
  coral: "E85D3C",
  coralSoft: "F9E4DD",
  coralDeep: "C64A2C",
  muted: "8A8780",
  quiet: "A8A59F",
  success: "1F8A5B",
  warning: "B46A1C",
  danger: "B94326",
  border: "D8D3C5",
};

const FONT_HEAD = "Georgia"; // serif distintivo para títulos
const FONT_BODY = "Calibri"; // sans neutro para cuerpo
const FONT_MONO = "Consolas"; // mono para códigos/datos

// ─── Helpers de iconos ─────────────────────────────────────────────────
function renderIconSvg(IconComponent, color = "#000000", size = 256) {
  return ReactDOMServer.renderToStaticMarkup(
    React.createElement(IconComponent, { color, size: String(size) }),
  );
}
async function iconToBase64Png(IconComponent, color, size = 256) {
  const svg = renderIconSvg(IconComponent, color, size);
  const buf = await sharp(Buffer.from(svg)).png().toBuffer();
  return "image/png;base64," + buf.toString("base64");
}

// ─── Helpers de slide compartidos ──────────────────────────────────────

// Footer / pie de página en todas las slides de contenido.
function addFooter(slide, pageNum, total) {
  slide.addText("timbre.ai", {
    x: 0.5, y: 5.25, w: 2, h: 0.3,
    fontFace: FONT_HEAD, fontSize: 10, color: COLOR.muted,
    italic: true, charSpacing: 4,
  });
  slide.addText(`${pageNum} / ${total}`, {
    x: 8.5, y: 5.25, w: 1, h: 0.3,
    fontFace: FONT_BODY, fontSize: 9, color: COLOR.muted,
    align: "right",
  });
  // Línea sutil arriba del footer.
  slide.addShape("rect", {
    x: 0.5, y: 5.22, w: 9, h: 0.01,
    fill: { color: COLOR.border }, line: { color: COLOR.border, width: 0 },
  });
}

// Brand mark — usado en title slide.
function addBrandMark(slide, x, y) {
  // Círculo coral con punto (simulando la marca).
  slide.addShape("oval", {
    x: x, y: y, w: 0.4, h: 0.4,
    fill: { color: COLOR.coral }, line: { color: COLOR.coral, width: 0 },
  });
  slide.addShape("oval", {
    x: x + 0.14, y: y + 0.14, w: 0.12, h: 0.12,
    fill: { color: COLOR.paper }, line: { color: COLOR.paper, width: 0 },
  });
}

// Eyebrow (texto pequeño en mayúsculas tipo "PORTAL · OPERACIONES").
function addEyebrow(slide, text, x, y, color = COLOR.coral, width = 6) {
  slide.addText(text, {
    x, y, w: width, h: 0.3,
    fontFace: FONT_BODY, fontSize: 10, bold: true,
    color, charSpacing: 4, margin: 0,
  });
}

// Título grande de slide de contenido.
function addTitle(slide, text, x = 0.5, y = 0.75, w = 9, h = 0.8) {
  slide.addText(text, {
    x, y, w, h,
    fontFace: FONT_HEAD, fontSize: 36, bold: false,
    color: COLOR.ink, margin: 0,
  });
}

// Subtítulo / lede debajo del título.
function addLede(slide, text, x = 0.5, y = 1.5, w = 9, h = 0.5) {
  slide.addText(text, {
    x, y, w, h,
    fontFace: FONT_BODY, fontSize: 14,
    color: COLOR.muted, margin: 0,
  });
}

// Tarjeta de feature (icono coral + título + descripción).
async function addFeatureCard(slide, opts) {
  const { x, y, w, h, IconC, title, body } = opts;
  // Tarjeta
  slide.addShape("rect", {
    x, y, w, h,
    fill: { color: COLOR.surface },
    line: { color: COLOR.border, width: 1 },
  });
  // Acento coral lateral
  slide.addShape("rect", {
    x, y, w: 0.06, h,
    fill: { color: COLOR.coral }, line: { color: COLOR.coral, width: 0 },
  });
  // Círculo con icono
  slide.addShape("oval", {
    x: x + 0.3, y: y + 0.3, w: 0.55, h: 0.55,
    fill: { color: COLOR.coralSoft },
    line: { color: COLOR.coralSoft, width: 0 },
  });
  const iconData = await iconToBase64Png(IconC, "#" + COLOR.coralDeep, 256);
  slide.addImage({
    data: iconData,
    x: x + 0.4, y: y + 0.4, w: 0.35, h: 0.35,
  });
  // Título de la tarjeta
  slide.addText(title, {
    x: x + 1.0, y: y + 0.3, w: w - 1.2, h: 0.5,
    fontFace: FONT_HEAD, fontSize: 16, bold: false,
    color: COLOR.ink, margin: 0,
  });
  // Body
  slide.addText(body, {
    x: x + 1.0, y: y + 0.78, w: w - 1.2, h: h - 0.9,
    fontFace: FONT_BODY, fontSize: 11,
    color: COLOR.inkSoft, margin: 0,
  });
}

// Chip estilo app (rounded pill con texto).
function addChip(slide, text, x, y, color = COLOR.ink, bg = COLOR.paper3) {
  slide.addShape("roundRect", {
    x, y, w: 0.06 * text.length + 0.4, h: 0.28,
    fill: { color: bg },
    line: { color: bg, width: 0 },
    rectRadius: 0.14,
  });
  slide.addText(text, {
    x, y, w: 0.06 * text.length + 0.4, h: 0.28,
    fontFace: FONT_BODY, fontSize: 9, bold: true,
    color, align: "center", valign: "middle",
    charSpacing: 3, margin: 0,
  });
}

// ─── BUILD ─────────────────────────────────────────────────────────────
async function build() {
  const pres = new pptxgen();
  pres.layout = "LAYOUT_16x9"; // 10" × 5.625"
  pres.author = "timbre.ai";
  pres.title = "timbre.ai — Plataforma de bots de voz IA";

  const TOTAL_SLIDES = 18;
  // Empezamos en 1 — slide 1 es la title slide y no muestra footer.
  // El primer slide con footer es el slide 2 → muestra "2 / 18".
  let pageNum = 1;

  // ════════════════════════════════════════════════════════════════════
  // Slide 1 — Title (dark)
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.ink };

    // Marca arriba-izquierda
    addBrandMark(s, 0.6, 0.6);
    s.addText([
      { text: "timbre", options: { color: COLOR.paper, bold: false } },
      { text: ".ai", options: { color: COLOR.coral, bold: false } },
    ], {
      x: 1.15, y: 0.55, w: 4, h: 0.5,
      fontFace: FONT_HEAD, fontSize: 22, margin: 0,
    });

    // Título central
    s.addText("Bots de voz IA para inmobiliarias", {
      x: 0.6, y: 1.8, w: 9, h: 1.4,
      fontFace: FONT_HEAD, fontSize: 56,
      color: COLOR.paper, margin: 0,
    });

    // Subtítulo
    s.addText("Llamadas salientes y entrantes automatizadas a leads — calificación, agendado de visitas y seguimiento, 24/7.", {
      x: 0.6, y: 3.3, w: 8.5, h: 0.8,
      fontFace: FONT_BODY, fontSize: 16,
      color: COLOR.paper2, margin: 0,
    });

    // Acento coral diagonal abajo
    s.addShape("rect", {
      x: 0.6, y: 4.3, w: 0.6, h: 0.04,
      fill: { color: COLOR.coral }, line: { color: COLOR.coral, width: 0 },
    });

    // Footer en dark
    s.addText("PRESENTACIÓN DE PRODUCTO  ·  V1.0", {
      x: 0.6, y: 4.5, w: 6, h: 0.3,
      fontFace: FONT_BODY, fontSize: 10, bold: true,
      color: COLOR.muted, charSpacing: 6, margin: 0,
    });

    s.addText("smartgroup.es", {
      x: 7, y: 4.5, w: 2.4, h: 0.3,
      fontFace: FONT_BODY, fontSize: 10, italic: true,
      color: COLOR.quiet, align: "right", margin: 0,
    });
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 2 — El problema
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "EL PROBLEMA", 0.5, 0.5);
    addTitle(s, "Los leads inmobiliarios se enfrían rápido.", 0.5, 0.85, 9, 1.4);

    // Stats grandes a la izquierda
    s.addText("78%", {
      x: 0.5, y: 2.3, w: 3, h: 1.2,
      fontFace: FONT_HEAD, fontSize: 90, bold: true,
      color: COLOR.coral, margin: 0,
    });
    s.addText("de los leads se pierden si no se contactan en menos de 5 minutos.", {
      x: 0.5, y: 3.5, w: 4, h: 0.8,
      fontFace: FONT_BODY, fontSize: 12,
      color: COLOR.inkSoft, margin: 0,
    });

    // Lista de fricciones a la derecha
    s.addShape("rect", {
      x: 5.2, y: 2.3, w: 4.3, h: 2.6,
      fill: { color: COLOR.surface },
      line: { color: COLOR.border, width: 1 },
    });
    s.addText("HOY EN UN COMERCIAL HUMANO", {
      x: 5.4, y: 2.5, w: 4, h: 0.3,
      fontFace: FONT_BODY, fontSize: 9, bold: true,
      color: COLOR.coral, charSpacing: 4, margin: 0,
    });
    s.addText([
      { text: "Horario limitado y vacaciones", options: { bullet: true, breakLine: true } },
      { text: "Lentitud al responder fuera de horas", options: { bullet: true, breakLine: true } },
      { text: "Calidad inconsistente entre operadores", options: { bullet: true, breakLine: true } },
      { text: "Difícil escalar a 1.000+ llamadas/día", options: { bullet: true, breakLine: true } },
      { text: "Sin registro de calidad ni auditoría", options: { bullet: true } },
    ], {
      x: 5.4, y: 2.85, w: 4, h: 2,
      fontFace: FONT_BODY, fontSize: 12, color: COLOR.ink,
      paraSpaceAfter: 6, margin: 0,
    });

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 3 — La solución
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "LA SOLUCIÓN", 0.5, 0.5);
    addTitle(s, "Un agente de voz IA que llama por ti.");
    addLede(s, "Multi-tenant, multi-idioma, integrado con tu CRM y agenda. Conversación natural, en tiempo real.");

    // Tres pilares
    const pillars = [
      { IconC: FaPhoneVolume, t: "Llamadas en tiempo real", d: "Conversaciones bidireccionales con latencia sub-segundo sobre tu propio trunk SIP." },
      { IconC: FaRobot, t: "IA entrenable", d: "Cada bot tiene objetivo, idioma, voz y herramientas a medida. RAG sobre tus propios documentos." },
      { IconC: FaShieldAlt, t: "Operación 24/7", d: "Detección de buzón, reglas de horario por DID, cumplimiento DNC y auditoría completa." },
    ];

    for (let i = 0; i < pillars.length; i++) {
      const x = 0.5 + i * 3.05;
      const p = pillars[i];

      // Card
      s.addShape("rect", {
        x, y: 2.4, w: 2.85, h: 2.55,
        fill: { color: COLOR.surface },
        line: { color: COLOR.border, width: 1 },
      });

      // Icono coral
      s.addShape("oval", {
        x: x + 0.3, y: 2.6, w: 0.7, h: 0.7,
        fill: { color: COLOR.coral }, line: { color: COLOR.coral, width: 0 },
      });
      const iconData = await iconToBase64Png(p.IconC, "#" + COLOR.paper, 256);
      s.addImage({
        data: iconData,
        x: x + 0.46, y: 2.76, w: 0.38, h: 0.38,
      });

      s.addText(p.t, {
        x: x + 0.3, y: 3.4, w: 2.4, h: 0.5,
        fontFace: FONT_HEAD, fontSize: 17,
        color: COLOR.ink, margin: 0,
      });
      s.addText(p.d, {
        x: x + 0.3, y: 3.85, w: 2.4, h: 1.0,
        fontFace: FONT_BODY, fontSize: 11,
        color: COLOR.inkSoft, margin: 0,
      });
    }

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 4 — Capabilities overview
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "VISIÓN GENERAL", 0.5, 0.5);
    addTitle(s, "Todo lo que necesitas, en una sola plataforma.");

    // 2x3 grid de features
    const features = [
      { IconC: FaPhoneAlt,    t: "Salientes",         d: "Campañas masivas con concurrencia y reintentos." },
      { IconC: FaInbox,       t: "Entrantes",         d: "Rutas por DID con horario y prefijo del caller." },
      { IconC: FaCalendarAlt, t: "Google Calendar",   d: "El bot agenda visitas durante la llamada." },
      { IconC: FaBookOpen,    t: "Knowledge Base",    d: "RAG sobre tu catálogo, FAQs y políticas." },
      { IconC: FaPlug,        t: "Webhooks → CRM",    d: "Eventos firmados con HMAC hacia tu sistema." },
      { IconC: FaDollarSign,  t: "Coste real",        d: "Desglose STT · LLM · TTS por llamada." },
    ];

    for (let i = 0; i < features.length; i++) {
      const col = i % 3;
      const row = Math.floor(i / 3);
      await addFeatureCard(s, {
        x: 0.5 + col * 3.05, y: 2.1 + row * 1.5,
        w: 2.85, h: 1.35,
        IconC: features[i].IconC,
        title: features[i].t,
        body: features[i].d,
      });
    }

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 5 — Bots: el corazón
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "BOTS DE VOZ", 0.5, 0.5);
    addTitle(s, "Define el bot. La IA hace el resto.");
    addLede(s, "Cada bot tiene su propia personalidad, voz, idioma y comportamiento.");

    // Mock del editor a la izquierda (chips/secciones)
    s.addShape("rect", {
      x: 0.5, y: 2.2, w: 4.4, h: 2.85,
      fill: { color: COLOR.surface },
      line: { color: COLOR.border, width: 1 },
    });
    s.addText("Editor de bot", {
      x: 0.7, y: 2.35, w: 3, h: 0.3,
      fontFace: FONT_BODY, fontSize: 10, bold: true,
      color: COLOR.coral, charSpacing: 3, margin: 0,
    });
    // chips de secciones del bot
    const sections = ["Identidad", "Voz IA", "Comportamiento", "Detección buzón", "Tools", "Calendar"];
    for (let i = 0; i < sections.length; i++) {
      const col = i % 2;
      const row = Math.floor(i / 2);
      const x = 0.7 + col * 1.85;
      const y = 2.75 + row * 0.4;
      s.addShape("roundRect", {
        x, y, w: 1.7, h: 0.3,
        fill: { color: COLOR.paper3 },
        line: { color: COLOR.border, width: 0.5 },
        rectRadius: 0.05,
      });
      s.addText(sections[i], {
        x, y, w: 1.7, h: 0.3,
        fontFace: FONT_BODY, fontSize: 10,
        color: COLOR.ink, align: "center", valign: "middle", margin: 0,
      });
    }
    s.addText([
      { text: "Objetivo:  ", options: { bold: true, color: COLOR.coral } },
      { text: "calificar leads de alquiler en Madrid y agendar visita.", options: { color: COLOR.inkSoft } },
    ], {
      x: 0.7, y: 4.4, w: 4, h: 0.55,
      fontFace: FONT_BODY, fontSize: 11, italic: true, margin: 0,
    });

    // Bloque derecho — qué controla
    addEyebrow(s, "CONFIGURABLE", 5.3, 2.2);
    s.addText([
      { text: "Idioma y voz", options: { bullet: true, breakLine: true, bold: true } },
      { text: "13 idiomas — ES, EN, FR, DE, IT, PT…", options: { bullet: { indent: 18 }, breakLine: true, color: COLOR.muted } },
      { text: "Provider de IA", options: { bullet: true, breakLine: true, bold: true } },
      { text: "OpenAI Realtime · Deepgram · AssemblyAI", options: { bullet: { indent: 18 }, breakLine: true, color: COLOR.muted } },
      { text: "Guardrails y reglas", options: { bullet: true, breakLine: true, bold: true } },
      { text: "Identificarse como IA · No inventar precios", options: { bullet: { indent: 18 }, color: COLOR.muted } },
    ], {
      x: 5.3, y: 2.5, w: 4.2, h: 2.5,
      fontFace: FONT_BODY, fontSize: 11, color: COLOR.ink,
      paraSpaceAfter: 4, margin: 0,
    });

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 6 — Multi-provider voice IA
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "VOZ IA");
    addTitle(s, "Elige el mejor cerebro para cada bot.");
    addLede(s, "No te casamos con un proveedor. Tres opciones de primer nivel — y siempre puedes cambiar.");

    const providers = [
      {
        name: "OpenAI Realtime",
        tagline: "gpt-realtime (GA)",
        bullets: [
          "Latencia más baja del mercado",
          "Voz natural multi-idioma",
          "20% más barato vs preview",
        ],
        color: COLOR.success,
      },
      {
        name: "Deepgram Voice Agent",
        tagline: "STT + LLM + TTS",
        bullets: [
          "Coste por minuto más ajustado",
          "Modelos nova-3 / aura-2",
          "Pensado para voz, no para chat",
        ],
        color: COLOR.coral,
      },
      {
        name: "AssemblyAI Universal Streaming",
        tagline: "Streaming end-to-end",
        bullets: [
          "Buena transcripción de ruido",
          "Voces premium en ES y EN",
          "Latencia competitiva",
        ],
        color: COLOR.warning,
      },
    ];

    for (let i = 0; i < providers.length; i++) {
      const x = 0.5 + i * 3.05;
      const p = providers[i];

      s.addShape("rect", {
        x, y: 2.4, w: 2.85, h: 2.6,
        fill: { color: COLOR.surface },
        line: { color: COLOR.border, width: 1 },
      });
      // Banda superior coloreada
      s.addShape("rect", {
        x, y: 2.4, w: 2.85, h: 0.18,
        fill: { color: p.color }, line: { color: p.color, width: 0 },
      });

      s.addText(p.name, {
        x: x + 0.25, y: 2.7, w: 2.6, h: 0.4,
        fontFace: FONT_HEAD, fontSize: 15,
        color: COLOR.ink, margin: 0,
      });
      s.addText(p.tagline, {
        x: x + 0.25, y: 3.05, w: 2.6, h: 0.3,
        fontFace: FONT_MONO, fontSize: 10,
        color: COLOR.muted, margin: 0,
      });
      const bulletItems = p.bullets.map((b, idx) => ({
        text: b,
        options: { bullet: true, breakLine: idx < p.bullets.length - 1 },
      }));
      s.addText(bulletItems, {
        x: x + 0.25, y: 3.45, w: 2.55, h: 1.5,
        fontFace: FONT_BODY, fontSize: 11, color: COLOR.inkSoft,
        paraSpaceAfter: 4, margin: 0,
      });
    }

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 7 — Campañas salientes
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "OUTBOUND");
    addTitle(s, "Campañas masivas con control fino.");

    // Card izquierdo: flujo
    s.addShape("rect", {
      x: 0.5, y: 2.05, w: 4.5, h: 3.0,
      fill: { color: COLOR.surface }, line: { color: COLOR.border, width: 1 },
    });
    s.addText("FLUJO DE UNA CAMPAÑA", {
      x: 0.7, y: 2.2, w: 4, h: 0.3,
      fontFace: FONT_BODY, fontSize: 10, bold: true,
      color: COLOR.coral, charSpacing: 3, margin: 0,
    });

    const steps = [
      "Importas leads (CSV o API)",
      "Asignas el bot y la ventana horaria",
      "El dialer marca con concurrencia × cap",
      "Reintentos automáticos con cooldown",
      "Realtime: ves cada llamada en directo",
    ];
    for (let i = 0; i < steps.length; i++) {
      const y = 2.65 + i * 0.45;
      // Número en círculo coral
      s.addShape("oval", {
        x: 0.7, y, w: 0.32, h: 0.32,
        fill: { color: COLOR.coralSoft }, line: { color: COLOR.coral, width: 1 },
      });
      s.addText(`${i + 1}`, {
        x: 0.7, y, w: 0.32, h: 0.32,
        fontFace: FONT_HEAD, fontSize: 13, bold: true,
        color: COLOR.coralDeep, align: "center", valign: "middle", margin: 0,
      });
      s.addText(steps[i], {
        x: 1.15, y, w: 3.5, h: 0.32,
        fontFace: FONT_BODY, fontSize: 12,
        color: COLOR.ink, valign: "middle", margin: 0,
      });
    }

    // Card derecho: capacidades
    addEyebrow(s, "CAPACIDADES", 5.4, 2.2);
    s.addText([
      { text: "Concurrencia configurable", options: { bullet: true, breakLine: true } },
      { text: "Daily call cap por tenant", options: { bullet: true, breakLine: true } },
      { text: "Horario y zona horaria locales", options: { bullet: true, breakLine: true } },
      { text: "Reintentos con backoff exponencial", options: { bullet: true, breakLine: true } },
      { text: "Pausar y reanudar en cualquier momento", options: { bullet: true, breakLine: true } },
      { text: "Métricas de outcome en tiempo real", options: { bullet: true } },
    ], {
      x: 5.4, y: 2.55, w: 4.2, h: 2.5,
      fontFace: FONT_BODY, fontSize: 12, color: COLOR.ink,
      paraSpaceAfter: 6, margin: 0,
    });

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 8 — Inbound calls con DID routing
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "INBOUND");
    addTitle(s, "Llamadas entrantes con reglas inteligentes.");
    addLede(s, "Cada DID puede tener N reglas por horario, día, prefijo del caller o idioma.");

    // Diagrama horizontal: Caller → DID → Reglas → Bot
    const nodes = [
      { x: 0.5, label: "Caller +34…", icon: FaPhoneVolume },
      { x: 3.0, label: "DID +34 911…", icon: FaInbox },
      { x: 5.5, label: "Evaluar reglas", icon: FaListUl },
      { x: 8.0, label: "Bot adecuado", icon: FaRobot },
    ];

    for (let i = 0; i < nodes.length; i++) {
      const n = nodes[i];
      // Caja
      s.addShape("rect", {
        x: n.x, y: 2.3, w: 1.7, h: 1.2,
        fill: { color: COLOR.surface }, line: { color: COLOR.border, width: 1 },
      });
      // Icono coral arriba
      const iconData = await iconToBase64Png(n.icon, "#" + COLOR.coralDeep, 256);
      s.addImage({
        data: iconData,
        x: n.x + 0.65, y: 2.5, w: 0.4, h: 0.4,
      });
      s.addText(n.label, {
        x: n.x, y: 2.95, w: 1.7, h: 0.4,
        fontFace: FONT_BODY, fontSize: 10, bold: true,
        color: COLOR.ink, align: "center", margin: 0,
      });

      // Flecha al siguiente (excepto último). rightArrow es nativa de
      // PptxGenJS y ya tiene la geometría correcta — más claro que
      // rotar un triángulo manualmente.
      if (i < nodes.length - 1) {
        s.addShape("rightArrow", {
          x: n.x + 1.78, y: 2.78, w: 0.65, h: 0.35,
          fill: { color: COLOR.coral },
          line: { color: COLOR.coral, width: 0 },
        });
      }
    }

    // Ejemplos de reglas debajo
    addEyebrow(s, "EJEMPLOS DE REGLA", 0.5, 3.85);
    const examples = [
      "L-V 09:00–18:00 → bot comercial",
      "Sábados 10:00–14:00 → bot de guardia",
      "Prefijo +1 → bot en inglés",
      "Fuera de cualquier regla → bot por defecto del DID",
    ];
    for (let i = 0; i < examples.length; i++) {
      const x = 0.5 + (i % 2) * 4.6;
      const y = 4.15 + Math.floor(i / 2) * 0.42;
      s.addShape("rect", {
        x, y, w: 4.4, h: 0.32,
        fill: { color: COLOR.paper3 }, line: { color: COLOR.border, width: 0 },
      });
      s.addText(examples[i], {
        x: x + 0.15, y, w: 4.2, h: 0.32,
        fontFace: FONT_MONO, fontSize: 10,
        color: COLOR.ink, valign: "middle", margin: 0,
      });
    }

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 9 — AMD (Answering Machine Detection)
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "DETECCIÓN DE BUZÓN");
    addTitle(s, "Si salta el contestador, el bot lo sabe.");
    addLede(s, "Analiza los primeros segundos del audio. Decide en menos de 5s.");

    // Diagrama de decisión
    s.addShape("rect", {
      x: 0.5, y: 2.2, w: 4.4, h: 2.85,
      fill: { color: COLOR.surface }, line: { color: COLOR.border, width: 1 },
    });
    s.addText("HEURÍSTICA EN VOICE-AGENT", {
      x: 0.7, y: 2.35, w: 4, h: 0.3,
      fontFace: FONT_BODY, fontSize: 10, bold: true,
      color: COLOR.coral, charSpacing: 3, margin: 0,
    });
    s.addText([
      { text: "Mide RMS energy + duración de bursts", options: { bullet: true, breakLine: true } },
      { text: "Burst largo sin pausa → buzón", options: { bullet: true, breakLine: true, color: COLOR.danger } },
      { text: "Burst corto + silencio → humano", options: { bullet: true, breakLine: true, color: COLOR.success } },
      { text: "Decide en ≤ 4.5s, ~1 µs por frame", options: { bullet: true } },
    ], {
      x: 0.7, y: 2.7, w: 4.1, h: 2.3,
      fontFace: FONT_BODY, fontSize: 12, color: COLOR.ink,
      paraSpaceAfter: 8, margin: 0,
    });

    // Tres acciones
    addEyebrow(s, "QUÉ HACE EL BOT", 5.3, 2.2);

    const actions = [
      { icon: "✕", title: "Colgar", desc: "No gasta API en una máquina.", color: COLOR.danger },
      { icon: "✎", title: "Dejar mensaje", desc: "Recita un mensaje pregrabado y cuelga.", color: COLOR.coral },
      { icon: "→", title: "Continuar", desc: "Solo registra el evento, sigue.", color: COLOR.muted },
    ];

    for (let i = 0; i < actions.length; i++) {
      const y = 2.45 + i * 0.82;
      const a = actions[i];
      s.addShape("rect", {
        x: 5.3, y, w: 4.2, h: 0.72,
        fill: { color: COLOR.surface }, line: { color: COLOR.border, width: 1 },
      });
      s.addShape("rect", {
        x: 5.3, y, w: 0.06, h: 0.72,
        fill: { color: a.color }, line: { color: a.color, width: 0 },
      });
      s.addText(a.title, {
        x: 5.5, y: y + 0.08, w: 4, h: 0.3,
        fontFace: FONT_HEAD, fontSize: 13,
        color: COLOR.ink, margin: 0,
      });
      s.addText(a.desc, {
        x: 5.5, y: y + 0.38, w: 4, h: 0.3,
        fontFace: FONT_BODY, fontSize: 10,
        color: COLOR.inkSoft, margin: 0,
      });
    }

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 10 — Tools / Function calling
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "TOOLS / FUNCTION CALLING");
    addTitle(s, "El bot ejecuta acciones reales en mitad de la llamada.");
    addLede(s, "Biblioteca de herramientas por tenant. Asignas las que quieras a cada bot.");

    // Lista de tools como tiles
    const tools = [
      { IconC: FaCalendarAlt, name: "schedule_meeting", desc: "Agenda en Google Calendar" },
      { IconC: FaCalendarAlt, name: "reschedule_meeting", desc: "Mueve la cita del lead" },
      { IconC: FaCalendarAlt, name: "cancel_meeting", desc: "Cancela con verificación de propiedad" },
      { IconC: FaSearch,      name: "search_knowledge_base", desc: "Busca en tu RAG" },
      { IconC: FaShareAlt,    name: "send_webhook",    desc: "Notifica a tu CRM" },
      { IconC: FaCog,         name: "custom_action",   desc: "Cualquier endpoint REST tuyo" },
    ];

    for (let i = 0; i < tools.length; i++) {
      const col = i % 2;
      const row = Math.floor(i / 2);
      const x = 0.5 + col * 4.6;
      const y = 2.15 + row * 0.95;
      const t = tools[i];

      s.addShape("rect", {
        x, y, w: 4.4, h: 0.85,
        fill: { color: COLOR.surface }, line: { color: COLOR.border, width: 1 },
      });
      // Círculo
      s.addShape("oval", {
        x: x + 0.2, y: y + 0.2, w: 0.45, h: 0.45,
        fill: { color: COLOR.coralSoft }, line: { color: COLOR.coralSoft, width: 0 },
      });
      const iconData = await iconToBase64Png(t.IconC, "#" + COLOR.coralDeep, 256);
      s.addImage({
        data: iconData,
        x: x + 0.27, y: y + 0.27, w: 0.3, h: 0.3,
      });
      s.addText(t.name, {
        x: x + 0.8, y: y + 0.12, w: 3.5, h: 0.35,
        fontFace: FONT_MONO, fontSize: 12, bold: true,
        color: COLOR.ink, margin: 0,
      });
      s.addText(t.desc, {
        x: x + 0.8, y: y + 0.45, w: 3.5, h: 0.3,
        fontFace: FONT_BODY, fontSize: 10,
        color: COLOR.muted, margin: 0,
      });
    }

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 11 — Knowledge Base (RAG)
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "KNOWLEDGE BASE");
    addTitle(s, "Tu catálogo, FAQs y políticas, dentro del bot.");
    addLede(s, "Sube documentos. El bot los entiende y los usa al hablar con el cliente.");

    // Pipeline horizontal
    const stages = [
      { IconC: FaCloudUploadAlt, label: "Subes\ndocumentos" },
      { IconC: FaLayerGroup,     label: "Chunking +\nembeddings" },
      { IconC: FaServer,         label: "pgvector\ntenant-scoped" },
      { IconC: FaQuoteLeft,      label: "Citas en la\nrespuesta del bot" },
    ];

    for (let i = 0; i < stages.length; i++) {
      const x = 0.5 + i * 2.4;
      const st = stages[i];

      // Círculo grande
      s.addShape("oval", {
        x: x + 0.5, y: 2.3, w: 1.0, h: 1.0,
        fill: { color: COLOR.coralSoft }, line: { color: COLOR.coral, width: 1.5 },
      });
      const iconData = await iconToBase64Png(st.IconC, "#" + COLOR.coralDeep, 256);
      s.addImage({
        data: iconData,
        x: x + 0.75, y: 2.55, w: 0.5, h: 0.5,
      });
      // Etiqueta
      s.addText(st.label, {
        x: x, y: 3.45, w: 2.0, h: 0.6,
        fontFace: FONT_BODY, fontSize: 11, bold: true,
        color: COLOR.ink, align: "center", margin: 0,
      });

      // Flecha al siguiente — rightArrow es la forma nativa.
      if (i < stages.length - 1) {
        s.addShape("rightArrow", {
          x: x + 1.55, y: 2.65, w: 0.85, h: 0.3,
          fill: { color: COLOR.coral },
          line: { color: COLOR.coral, width: 0 },
        });
      }
    }

    // Footer info
    s.addShape("rect", {
      x: 0.5, y: 4.3, w: 9, h: 0.7,
      fill: { color: COLOR.inkSoft }, line: { color: COLOR.inkSoft, width: 0 },
    });
    s.addText([
      { text: "1.536 dimensiones · OpenAI text-embedding-3-small · ", options: { color: COLOR.paper2 } },
      { text: "aislamiento total entre tenants", options: { color: COLOR.coral, bold: true } },
      { text: " · sin entrenar ningún modelo con tus datos.", options: { color: COLOR.paper2 } },
    ], {
      x: 0.7, y: 4.3, w: 8.6, h: 0.7,
      fontFace: FONT_BODY, fontSize: 11, italic: true,
      valign: "middle", margin: 0,
    });

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 12 — Calendar integration
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "AGENDA AUTOMÁTICA");
    addTitle(s, "El bot agenda visitas en tu Google Calendar.");
    addLede(s, "Conexión OAuth segura, refresh tokens cifrados con AES-256-GCM.");

    // Two columns
    // Izquierda: quote del bot
    s.addShape("rect", {
      x: 0.5, y: 2.2, w: 4.5, h: 2.85,
      fill: { color: COLOR.ink }, line: { color: COLOR.ink, width: 0 },
    });
    s.addText("CONVERSACIÓN REAL", {
      x: 0.7, y: 2.35, w: 4, h: 0.3,
      fontFace: FONT_BODY, fontSize: 9, bold: true,
      color: COLOR.coral, charSpacing: 3, margin: 0,
    });

    s.addText([
      { text: "Bot:  ", options: { color: COLOR.coral, bold: true } },
      { text: "¿Le viene bien el martes a las 17:00?", options: { color: COLOR.paper } },
      { text: "\n\nLead:  ", options: { color: COLOR.paper2, bold: true } },
      { text: "Sí, perfecto.", options: { color: COLOR.paper } },
      { text: "\n\nBot:  ", options: { color: COLOR.coral, bold: true } },
      { text: "Confirmado. Le envío invitación a su correo.", options: { color: COLOR.paper } },
    ], {
      x: 0.7, y: 2.7, w: 4.2, h: 2.2,
      fontFace: FONT_HEAD, fontSize: 13, italic: true,
      margin: 0,
    });

    // Derecha: capacidades
    addEyebrow(s, "QUÉ HACE EL BOT", 5.3, 2.2);
    const calItems = [
      "Agendar nueva cita con disponibilidad real",
      "Reagendar o cancelar (verifica que es del mismo lead)",
      "Envía invitación con datos del lead",
      "Sincroniza con tu calendar — no hay duplicados",
    ];
    for (let i = 0; i < calItems.length; i++) {
      const y = 2.6 + i * 0.55;
      // Tick
      const iconData = await iconToBase64Png(FaCheckCircle, "#" + COLOR.coral, 256);
      s.addImage({
        data: iconData,
        x: 5.3, y: y + 0.05, w: 0.25, h: 0.25,
      });
      s.addText(calItems[i], {
        x: 5.65, y, w: 3.85, h: 0.4,
        fontFace: FONT_BODY, fontSize: 12,
        color: COLOR.ink, margin: 0,
      });
    }

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 13 — Recordings + retención
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "GRABACIONES");
    addTitle(s, "Cada llamada queda guardada — sin disparar tu factura.");

    // Stat grande izquierda — fontSize 90 para que quepa en h=1.4
    // (al 110pt original desbordaba ~0.1" hacia abajo).
    s.addText("10×", {
      x: 0.5, y: 2.1, w: 3, h: 1.4,
      fontFace: FONT_HEAD, fontSize: 90, bold: true,
      color: COLOR.coral, margin: 0,
    });
    s.addText("MENOS ESPACIO", {
      x: 0.5, y: 3.4, w: 3, h: 0.3,
      fontFace: FONT_BODY, fontSize: 11, bold: true,
      color: COLOR.coralDeep, charSpacing: 4, margin: 0,
    });
    s.addText("Compresión OGG/Opus 24 kbps. WAV pasa de 1,92 MB/min a ~180 KB/min.", {
      x: 0.5, y: 3.75, w: 4, h: 0.7,
      fontFace: FONT_BODY, fontSize: 12,
      color: COLOR.inkSoft, margin: 0,
    });

    // Lista de features derecha
    addEyebrow(s, "ALMACENAMIENTO INTELIGENTE", 5.2, 2.1);
    const recItems = [
      { t: "MinIO/S3 compatible",          d: "Tu propio bucket — control total." },
      { t: "Presigned URLs on-demand",     d: "1h de validez, sin links eternos." },
      { t: "Retención configurable",       d: "Borrado automático a los N días." },
      { t: "Streaming a disco",            d: "Llamadas largas sin truncar." },
    ];
    for (let i = 0; i < recItems.length; i++) {
      const y = 2.5 + i * 0.62;
      s.addText(recItems[i].t, {
        x: 5.2, y, w: 4.4, h: 0.3,
        fontFace: FONT_BODY, fontSize: 12, bold: true,
        color: COLOR.ink, margin: 0,
      });
      s.addText(recItems[i].d, {
        x: 5.2, y: y + 0.25, w: 4.4, h: 0.3,
        fontFace: FONT_BODY, fontSize: 11,
        color: COLOR.muted, margin: 0,
      });
    }

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 14 — Webhooks salientes hacia CRM
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "INTEGRACIÓN");
    addTitle(s, "Webhooks firmados hacia tu CRM.");
    addLede(s, "Cada evento importante de la llamada cae en tu sistema en tiempo real.");

    // Lista de eventos a la izquierda
    addEyebrow(s, "EVENTOS DISPONIBLES", 0.5, 2.1);
    const events = [
      "lead.qualified",
      "meeting.scheduled",
      "call.finished",
      "call.transferred",
      "do_not_call.requested",
    ];
    for (let i = 0; i < events.length; i++) {
      const y = 2.5 + i * 0.48;
      s.addShape("rect", {
        x: 0.5, y, w: 4.3, h: 0.4,
        fill: { color: COLOR.ink }, line: { color: COLOR.ink, width: 0 },
      });
      s.addText(events[i], {
        x: 0.7, y, w: 4, h: 0.4,
        fontFace: FONT_MONO, fontSize: 12,
        color: COLOR.coral, valign: "middle", margin: 0,
      });
    }

    // Características a la derecha
    addEyebrow(s, "GARANTÍAS", 5.3, 2.1);
    const guarantees = [
      { t: "HMAC-SHA256", d: "Cabecera X-Timbre-Signature. Validas la firma con tu secret." },
      { t: "Reintentos exponenciales", d: "Si tu endpoint cae, reintenta hasta 24h." },
      { t: "Histórico auditable", d: "Las últimas 50 entregas visibles en el portal." },
      { t: "Rotación de secret", d: "Sin downtime: nuevo secret, viejo expira en 24h." },
    ];
    for (let i = 0; i < guarantees.length; i++) {
      const y = 2.5 + i * 0.58;
      s.addText(guarantees[i].t, {
        x: 5.3, y, w: 4.3, h: 0.3,
        fontFace: FONT_BODY, fontSize: 12, bold: true,
        color: COLOR.ink, margin: 0,
      });
      s.addText(guarantees[i].d, {
        x: 5.3, y: y + 0.25, w: 4.3, h: 0.3,
        fontFace: FONT_BODY, fontSize: 10,
        color: COLOR.muted, margin: 0,
      });
    }

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 15 — Dashboard realtime
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "DASHBOARD REALTIME");
    addTitle(s, "Lo que está pasando ahora, sin recargar.");
    addLede(s, "WebSocket push del backend a cada operador. Cmd+K para buscar cualquier cosa.");

    // Mock del hero del dashboard
    s.addShape("rect", {
      x: 0.5, y: 2.05, w: 9, h: 3.0,
      fill: { color: COLOR.surface }, line: { color: COLOR.border, width: 1 },
    });

    // Header del panel
    s.addText("EN ESTE MOMENTO", {
      x: 0.75, y: 2.2, w: 5, h: 0.3,
      fontFace: FONT_BODY, fontSize: 10, bold: true,
      color: COLOR.coral, charSpacing: 3, margin: 0,
    });
    s.addText("3 llamadas en curso", {
      x: 0.75, y: 2.45, w: 6, h: 0.5,
      fontFace: FONT_HEAD, fontSize: 20,
      color: COLOR.ink, margin: 0,
    });
    // live dot pulse mock
    s.addShape("oval", {
      x: 8.7, y: 2.3, w: 0.18, h: 0.18,
      fill: { color: COLOR.success }, line: { color: COLOR.success, width: 0 },
    });
    s.addText("LIVE", {
      x: 8.0, y: 2.3, w: 0.6, h: 0.18,
      fontFace: FONT_BODY, fontSize: 9, bold: true,
      color: COLOR.success, align: "right", valign: "middle", charSpacing: 4, margin: 0,
    });

    // 3 cards de llamadas activas
    const live = [
      { name: "Juan Pérez", phone: "+34 666 111 222", state: "Hablando", camp: "Renting Madrid" },
      { name: "María López", phone: "+34 666 333 444", state: "Marcando", camp: "Renting Madrid" },
      { name: "Carlos Ruiz", phone: "+34 666 555 666", state: "Sonando",   camp: "Owners BCN" },
    ];
    for (let i = 0; i < live.length; i++) {
      const x = 0.75 + i * 2.95;
      s.addShape("rect", {
        x, y: 3.15, w: 2.75, h: 1.7,
        fill: { color: COLOR.paper }, line: { color: COLOR.border, width: 1 },
      });
      s.addText(live[i].name, {
        x: x + 0.15, y: 3.25, w: 2.3, h: 0.35,
        fontFace: FONT_HEAD, fontSize: 14, bold: true,
        color: COLOR.ink, margin: 0,
      });
      // Status chip
      const statusColor = i === 0 ? COLOR.success : (i === 1 ? COLOR.warning : COLOR.coral);
      addChip(s, live[i].state.toUpperCase(), x + 0.15, 3.62, COLOR.paper, statusColor);

      s.addText(live[i].phone, {
        x: x + 0.15, y: 4.0, w: 2.5, h: 0.3,
        fontFace: FONT_MONO, fontSize: 10,
        color: COLOR.muted, margin: 0,
      });
      s.addText(live[i].camp, {
        x: x + 0.15, y: 4.4, w: 2.5, h: 0.3,
        fontFace: FONT_BODY, fontSize: 10, italic: true,
        color: COLOR.muted, margin: 0,
      });
    }

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 16 — Coste real por llamada
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "COSTE Y FACTURACIÓN");
    addTitle(s, "Sabes lo que cuesta cada llamada.");
    addLede(s, "Desglose por STT, LLM, TTS y trunk. Dashboard agregado por día, proveedor o campaña.");

    // Bar chart mock — números ilustrativos para una llamada de ~4 min
    // con OpenAI gpt-realtime (GA, ago 2025: $32/1M audio in, $64/1M
    // audio out, 20% más barato que gpt-4o-realtime-preview).
    const components = [
      { label: "STT",   value: 0.14, color: COLOR.warning },
      { label: "LLM",   value: 0.59, color: COLOR.coral },
      { label: "TTS",   value: 0.25, color: COLOR.coralDeep },
      { label: "Trunk", value: 0.06, color: COLOR.success },
      { label: "Otros", value: 0.04, color: COLOR.muted },
    ];

    // Eje base
    s.addShape("line", {
      x: 0.5, y: 4.4, w: 5, h: 0,
      line: { color: COLOR.border, width: 1 },
    });

    const maxVal = Math.max(...components.map((c) => c.value));
    const barMaxH = 1.6;

    for (let i = 0; i < components.length; i++) {
      const c = components[i];
      const x = 0.7 + i * 1.0;
      const h = (c.value / maxVal) * barMaxH;
      const y = 4.4 - h;

      s.addShape("rect", {
        x, y, w: 0.7, h,
        fill: { color: c.color }, line: { color: c.color, width: 0 },
      });
      s.addText(`$${c.value.toFixed(2)}`, {
        x: x - 0.1, y: y - 0.32, w: 0.9, h: 0.3,
        fontFace: FONT_MONO, fontSize: 10, bold: true,
        color: COLOR.ink, align: "center", margin: 0,
      });
      s.addText(c.label, {
        x: x - 0.1, y: 4.45, w: 0.9, h: 0.3,
        fontFace: FONT_BODY, fontSize: 10, bold: true,
        color: COLOR.muted, align: "center", margin: 0,
      });
    }

    // Total grande a la derecha
    s.addShape("rect", {
      x: 6.5, y: 2.2, w: 3, h: 2.7,
      fill: { color: COLOR.ink }, line: { color: COLOR.ink, width: 0 },
    });
    s.addText("COSTE LLAMADA", {
      x: 6.7, y: 2.4, w: 2.7, h: 0.3,
      fontFace: FONT_BODY, fontSize: 10, bold: true,
      color: COLOR.coral, charSpacing: 3, margin: 0,
    });
    s.addText("$1.08", {
      x: 6.7, y: 2.7, w: 2.7, h: 1.4,
      fontFace: FONT_HEAD, fontSize: 70, bold: true,
      color: COLOR.paper, margin: 0,
    });
    s.addText("4 min 23 s · OpenAI gpt-realtime", {
      x: 6.7, y: 4.1, w: 2.7, h: 0.3,
      fontFace: FONT_BODY, fontSize: 11, italic: true,
      color: COLOR.paper2, margin: 0,
    });
    s.addText("Tarifas configurables por componente.", {
      x: 6.7, y: 4.45, w: 2.7, h: 0.3,
      fontFace: FONT_BODY, fontSize: 9,
      color: COLOR.muted, margin: 0,
    });

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 17 — Compliance y seguridad
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.paper };
    pageNum++;

    addEyebrow(s, "SEGURIDAD Y CUMPLIMIENTO");
    addTitle(s, "Construido pensando en operador serio.");

    const items = [
      { IconC: FaUserShield, t: "Multi-tenant aislado",     d: "Cada cliente con su propio espacio. Imposible cruzar datos." },
      { IconC: FaShieldAlt,  t: "Roles granulares",         d: "platform_admin, tenant_admin, tenant_agent." },
      { IconC: FaListUl,     t: "Audit log completo",       d: "Quién hizo qué, cuándo. Filtrable y exportable." },
      { IconC: FaPhoneAlt,   t: "Do Not Call",              d: "Lista de bloqueo respetada en todas las campañas." },
      { IconC: FaRegSnowflake,t: "Secretos cifrados",       d: "AES-256-GCM para tokens OAuth y API keys." },
      { IconC: FaEye,        t: "Visibilidad total",        d: "Cada llamada con transcripción y grabación." },
    ];

    for (let i = 0; i < items.length; i++) {
      const col = i % 3;
      const row = Math.floor(i / 3);
      const x = 0.5 + col * 3.05;
      const y = 2.1 + row * 1.5;
      await addFeatureCard(s, {
        x, y, w: 2.85, h: 1.35,
        IconC: items[i].IconC, title: items[i].t, body: items[i].d,
      });
    }

    addFooter(s, pageNum, TOTAL_SLIDES);
  }

  // ════════════════════════════════════════════════════════════════════
  // Slide 18 — Cierre
  // ════════════════════════════════════════════════════════════════════
  {
    const s = pres.addSlide();
    s.background = { color: COLOR.ink };

    addBrandMark(s, 0.6, 0.6);
    s.addText([
      { text: "timbre", options: { color: COLOR.paper } },
      { text: ".ai", options: { color: COLOR.coral } },
    ], {
      x: 1.15, y: 0.55, w: 4, h: 0.5,
      fontFace: FONT_HEAD, fontSize: 22, margin: 0,
    });

    s.addText("¿Empezamos?", {
      x: 0.6, y: 1.8, w: 9, h: 1.4,
      fontFace: FONT_HEAD, fontSize: 68,
      color: COLOR.paper, margin: 0,
    });

    s.addText("Una demo de 30 minutos con tus propios leads.", {
      x: 0.6, y: 3.1, w: 8.5, h: 0.6,
      fontFace: FONT_BODY, fontSize: 18, italic: true,
      color: COLOR.paper2, margin: 0,
    });

    // CTA chip coral
    s.addShape("rect", {
      x: 0.6, y: 4.0, w: 3.2, h: 0.6,
      fill: { color: COLOR.coral }, line: { color: COLOR.coral, width: 0 },
    });
    s.addText("RESERVAR DEMO  →", {
      x: 0.6, y: 4.0, w: 3.2, h: 0.6,
      fontFace: FONT_BODY, fontSize: 14, bold: true,
      color: COLOR.paper, align: "center", valign: "middle",
      charSpacing: 4, margin: 0,
    });

    s.addText("soporte@smartgroup.es  ·  smartgroup.es/timbre", {
      x: 0.6, y: 4.8, w: 8.5, h: 0.3,
      fontFace: FONT_BODY, fontSize: 11,
      color: COLOR.muted, margin: 0,
    });
  }

  // Guardar
  await pres.writeFile({ fileName: "timbre-ai-presentacion.pptx" });
  console.log("✓ Generated timbre-ai-presentacion.pptx");
}

build().catch((err) => {
  console.error(err);
  process.exit(1);
});
