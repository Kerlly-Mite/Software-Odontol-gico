# SIGO-Dental

Sistema Integral de Gestión Odontológica y Expediente Clínico Digital.

## Alcance inicial

La interfaz cubre el prototipo de las Fases 1 a 3 del documento de requerimientos:

- **Fase 1:** ficha de pacientes y alertas médicas, agenda por sillón/profesional, planes de tratamiento y caja/cobros de demostración.
- **Fase 2:** expediente clínico, odontograma permanente/temporal, periodontograma, notas SOAP, recetas y registro de consentimientos.
- **Fase 3:** inventario con descargo automático configurado por procedimiento, honorarios configurables por profesional y reportes básicos de caja, agenda, comisiones y stock mínimo.

## Ejecutar la interfaz

Requiere Node.js 20.19+ o 22.12+.

```bash
cd frontend
npm install
npm run dev
```

La interfaz queda en <http://localhost:5174>. El servidor de Vite usa el puerto 5174 tanto fuera como dentro de Docker. Si quieres usar `npm run dev` en tu máquina mientras la API y la base siguen en Docker, ejecuta `docker compose stop web`, luego `docker compose up -d db api` y arranca Vite desde `frontend`. No ejecutes Vite local y el contenedor `web` al mismo tiempo porque ambos necesitan reservar ese puerto. Para volver al modo web en Docker, detén Vite con Ctrl+C y ejecuta `docker compose start web`.

## Ejecutar con Docker

Con Docker Desktop abierto, desde la carpeta raíz del repositorio:

```bash
docker compose up --build
```

- Web odontológica: <http://localhost:5174>
- API: <http://localhost:8080/api/health>
- PostgreSQL: `localhost:5433` (base `sigodental`, usuario `sigo`)

La primera ejecución crea el volumen de PostgreSQL y aplica el esquema inicial automáticamente. La API guarda pacientes, alertas médicas, citas, estados de cita, facturas y pagos en PostgreSQL. Al conectar una base vacía, la interfaz importa los registros ficticios de demostración una sola vez. Para dejar los servicios en segundo plano, usa `docker compose up --build -d`; para detenerlos, `docker compose down`. Los datos de PostgreSQL se conservan en el volumen.

## Cuentas locales de demostración

La API exige iniciar sesión y revisa el rol en cada endpoint protegido. Usa cualquiera de estas cuentas con la contraseña `Demo123!`:

- Administradora: `admin@sigo.local`
- Recepción: `recepcion@sigo.local`
- Odontología: `valeria.ruiz@sigo.local`
- Asistente: `asistente@sigo.local`
- Auditoría: `auditor@sigo.local`

## Estado de esta primera entrega

La API implementa login con JWT y permisos por rol, además de persistencia de pacientes, agenda, cobros, registros clínicos de Fase 2 (odontograma, periodontograma, notas SOAP append-only, recetas y consentimientos pendientes de firma) e inventario de insumos con stock mínimo e historial de entradas/consumos. Las reglas de descargo se configuran por nombre exacto del procedimiento y se aplican al marcar la cita como atendida. Los honorarios son estimaciones sobre reglas configuradas y procedimientos completados. Las migraciones idempotentes `backend/database/002_fase3.sql` y `003_fase4.sql` se ejecutan al iniciar la API, incluso con volúmenes de base existentes. La firma de notas/recetas del prototipo identifica la sesión de usuario y no constituye una firma electrónica legal. **No usar con información clínica real.** Las cuentas compartidas y contraseñas de demostración son solo para desarrollo. Antes de un piloto real faltan administración segura de cuentas, trazabilidad completa, respaldos cifrados, lotes/caducidad de insumos, firma legal e integraciones de facturación electrónica y mensajería.

La Fase 4 está en revisión: se añadió MFA TOTP obligatorio para endpoints protegidos, con secreto cifrado mediante `MFA_ENCRYPTION_KEY`, cierre de sesión del navegador tras 15 minutos sin actividad y registro de auditoría para endpoints autenticados, con protección de la tabla frente a actualización/borrado. Hay un borrador OpenAPI 3.0 en `backend/openapi.yaml`. La evidencia UAT, los bloqueos de seguridad y las verificaciones pendientes se documentan en [`docs/FASE4-UAT-Y-SEGURIDAD.md`](docs/FASE4-UAT-Y-SEGURIDAD.md). Falta validar el flujo MFA con PostgreSQL, TLS de despliegue, recuperación/rate limit de MFA, pruebas de integración/DAST y cobertura/documentación; esta versión no está lista para producción.

## Correspondencia con requisitos

| Área | Requisitos cubiertos en interfaz |
| --- | --- |
| Pacientes | Alta, búsqueda, edición y alertas médicas/antecedentes |
| Agenda | Citas por día, profesional y sillón; control de doble reserva |
| Caja | Registro de cobros y saldo de presupuestos de demostración |
| Expediente | Odontograma, superficies, notas de evolución y recetas |
| Periodoncia | Captura inicial de seis sitios por diente; NIC, sangrado y mediciones complementarias requieren completar validaciones clínicas |
| Consentimientos | Registro por procedimiento; la firma/OTP es solo un estado visual pendiente de proveedor |
| Inventario | Stock mínimo, historial, consumo manual y descargo automático al completar procedimientos con reglas configuradas |
| Honorarios y reportes | Reglas por profesional/procedimiento y reportes del rango elegido; valores estimados, no liquidación fiscal |
