# 03: Salir revocando la sesión

**What to build:** Desde cualquier vista privada la persona puede salir y dejar de estar autorizada, incluso si alguien intenta reutilizar su cookie anterior. La UI vuelve al acceso sin mostrar información privada en caché.

**Blocked by:** 02 — Iniciar sesión con una cuenta de demo persistida.

**Status:** resolved

- [ ] El menú de usuario presenta «Cerrar sesión» en español y la acción revoca la sesión almacenada, no solo oculta la vista.
- [ ] La cookie anterior deja de autorizar cualquier consulta protegida; recargar o usar Atrás no restablece una sesión revocada.
- [ ] Se limpian o invalidan las consultas privadas del navegador al salir; otra cuenta puede entrar después sin ver datos de sesión de la primera.
- [ ] Los tests HTTP cubren revocación, doble salida y expiración; un recorrido del navegador valida la navegación de retorno al acceso.
