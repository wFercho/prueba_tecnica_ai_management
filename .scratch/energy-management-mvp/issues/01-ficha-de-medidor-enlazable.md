# 01: Ficha de medidor enlazable y base visual en español

**What to build:** Una persona puede abrir directamente la ficha de M-109, recargarla y volver al panel sin perder la navegación. Es el primer recorrido completo que introduce TanStack Router, TanStack Query y Tailwind CSS 4 sobre la app existente, manteniendo la API Go y el contenido visible de ese recorrido en español.

**Blocked by:** None (can start immediately).

**Status:** resolved

- [x] La ficha se abre desde un enlace del panel y mediante URL directa o recarga; el botón Atrás del navegador se comporta como se espera.
- [x] Las rutas de la interfaz no colisionan con rutas de la API: una URL de ficha entrega la app y la consulta de lecturas sigue entregando JSON en el mismo origen.
- [x] La ficha obtiene los datos remotos con TanStack Query; no conserva un segundo mecanismo manual de carga para ese recorrido.
- [x] Tailwind CSS 4 se integra con Vite para el marco y la ficha sin eliminar de golpe los estilos de vistas todavía no migradas.
- [x] Navegación, estados de carga/error y textos que se tocan en este recorrido están en español; IDs y valores técnicos se preservan como datos.
- [x] Una comprobación HTTP de las rutas y un recorrido del navegador verifican enlace directo, recarga y consulta de la ficha.

## Comments

- Verificado con `go test ./internal/httpapi`, `make test`, `pnpm build`, `pnpm lint` y un recorrido en Chromium: panel → M-109 → recarga → Atrás/Adelante → URL directa; `/meters/M-109/readings` devuelve JSON. El texto de narración devuelto por la API sigue en inglés hasta los tickets de narración en español.
