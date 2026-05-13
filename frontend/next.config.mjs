/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  async rewrites() {
    // Reverse proxy server-side: el navegador solo habla con el dominio del
    // portal; Next reenvía /api/* al backend y /storage/* a minio por la red
    // Docker interna. Resultado: backend y minio sin dominio público.
    // Dentro de Docker, los nombres de servicio del compose resuelven por DNS
    // interno (`backend`, `minio`). Solo cambian si haces dev fuera de Docker.
    const backend = process.env.BACKEND_URL || "http://backend:8080";
    const storage = process.env.MINIO_URL || "http://minio:9000";
    return [
      { source: "/api/:path*", destination: `${backend}/api/:path*` },
      { source: "/healthz", destination: `${backend}/healthz` },
      { source: "/storage/:path*", destination: `${storage}/:path*` },
    ];
  },
};

export default nextConfig;
