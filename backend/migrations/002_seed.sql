-- Antes contenía datos demo (Atrium Leasing, Maria Lopez, Sunset Villas, etc.)
-- Ya no — el tenant principal lo crea el bootstrap del backend (ver main.go,
-- EnsureTenant) y los datos los crea el operador desde la UI.
--
-- Este archivo se queda como no-op para no romper el orden de migraciones de
-- los despliegues existentes. Ver 011_strip_demo.sql para la limpieza.
SELECT 1;
