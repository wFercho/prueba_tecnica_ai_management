# 02: Iniciar sesión con una cuenta de demo persistida

**What to build:** El evaluador entra como `admin@email.com` con contraseña `admin` y accede a las vistas y consultas protegidas mediante una sesión real. Los usuarios y sesiones tienen una única fuente de verdad en PostgreSQL; la experiencia es una demo **solo local**, no autenticación lista para Internet.

**Blocked by:** 01 — Ficha de medidor enlazable y base visual en español.

**Status:** resolved

- [ ] El primer arranque provisiona la cuenta demo solo si no existe, guarda un hash de la contraseña y permite provisionar otras cuentas administrativamente sin CRUD de usuarios en la interfaz.
- [ ] Login válido crea una sesión revocable y una cookie opaca HttpOnly; credenciales inválidas no crean sesión y muestran un error español sin filtrar datos sensibles.
- [ ] La API rechaza sin sesión válida lecturas, medidores, anomalías, resumen y ejecución de análisis. El guard de TanStack Router redirige al acceso y devuelve al destino solicitado tras entrar, sin considerarse la barrera de seguridad.
- [ ] Recursos públicos y un health check independiente de rutas protegidas siguen accesibles; el smoke y las utilidades existentes que llaman a la API se autentican o se adaptan para no fallar por un `401` inesperado.
- [ ] API y puerto publicado de PostgreSQL se vinculan por defecto a `127.0.0.1`. Queda visible y documentado que ambas contraseñas por defecto son solo de demo local.
- [ ] Tests HTTP de login, cookie, autorización y reingreso, más integración PostgreSQL de usuarios/sesiones y migraciones, prueban el recorrido sin exponer credenciales en logs ni frontend.
