# SIGO-Dental

Sistema Integral de Gestión Odontológica y Expediente Clínico Digital. Proyecto académico en desarrollo, basado en el SRS recibido el 29 de octubre de 2026. No usar con información clínica real.

## Ejecutar en desarrollo

Requiere Docker Desktop. Desde la raíz:

```bash
docker compose up --build -d
```

- Aplicación web: <http://localhost:5174>
- API: <http://localhost:8080/api/health>
- PostgreSQL: `localhost:5433`

Para detener contenedores sin borrar datos: `docker compose down`. **No uses `docker compose down -v` salvo que quieras borrar la base local.** PostgreSQL solo aplica `POSTGRES_USER`, `POSTGRES_PASSWORD` y `POSTGRES_DB` al inicializar un volumen vacío. Cambiar esas variables después no cambia las credenciales dentro de una base ya creada; conserva la contraseña original o cámbiala explícitamente en PostgreSQL antes de actualizar la configuración.

El modo local (`docker compose ...`) y el de producción (`docker compose -f docker-compose.prod.yml --env-file .env.prod ...`) usan configuraciones y contraseñas distintas. En este repositorio comparten por defecto nombres de contenedor y volumen: utiliza un solo modo a la vez y mantén el mismo comando/archivo de entorno al reiniciar. Si la base se creó con `.env.prod`, no la arranques con el comando local; la API recibirá otra contraseña y quedará unhealthy.

La interfaz local con Vite puede iniciarse con `cd frontend; npm install; npm run dev` en el puerto 5174. No ejecutes Vite local y el servicio web Docker al mismo tiempo.

## Cuentas de demostración

Las cuentas locales usan `Demo123!`: `admin@sigo.local`, `recepcion@sigo.local`, `valeria.ruiz@sigo.local`, `asistente@sigo.local` y `auditor@sigo.local`. Son credenciales de demostración; no reutilizarlas en producción.

## Estado frente al SRS

| Requisito | Estado en esta entrega |
| --- | --- |
| RF-01 | Registro, consulta y edición de ficha, contacto de emergencia, tutor, seguro y antecedentes. |
| RF-02 | Alertas de alergias/condiciones destacadas en expediente; validación clínica y reglas configurables aún requieren revisión. |
| RF-03 | Odontograma permanente/temporal, superficies, FDI/Universal y hallazgos de caries, restauración, endodoncia, implante, prótesis, exodoncia, sellante y ausencia. La persistencia usa el identificador dental canónico FDI. |
| RF-04 | Sondaje en seis sitios, margen, NIC calculado, sangrado, movilidad y furca. Requiere validación clínica antes de uso asistencial. |
| RF-05 | Notas SOAP append-only y endpoint/UI para adendas. El bloqueo configurable de 24 horas y una firma digital legal no están implementados; la marca actual identifica la sesión del usuario. |
| RF-06 | Consentimientos registrables; firma biométrica y OTP real pendientes. |
| RF-07 / RF-08 | Presupuestos por fases/sesiones, costos, descuentos y aprobación total/parcial; partidas aprobadas se pueden vincular a citas. |
| RF-09 / RF-10 / RN-03 | Agenda por sillón/profesional, estados de citas y protección contra doble reserva del sillón. |
| RF-11 | No implementado: WhatsApp/SMS/correo requiere cuentas y proveedores externos. |
| RF-12 | Registro básico de cobros; facturación electrónica fiscal y pagos reales no implementados. |
| RF-13 / RF-14 | Reglas de honorarios estimados y descargo de inventario al completar procedimientos. No es liquidación contable/fiscal. |
| RN-01 | Notas inmutables y creación de adendas vinculadas; firma legal pendiente. |
| RN-02 | Cierre de caja ciego: recepción registra el efectivo contado antes de ver el efectivo esperado y diferencia. |
| RNF-05 / RNF-06 | Roles, MFA TOTP y auditoría disponibles en el prototipo; faltan revisión de seguridad, recuperación y limitación de intentos. |
| RNF-07 | Diseño responsive implementado; falta validación formal en la matriz de dispositivos y navegadores solicitada. |

### Trabajo futuro / requisitos operativos pendientes

- Conseguir VPS/servidor con IP pública, configurar firewall y dominio, HTTPS/TLS 1.3 y certificados. El repositorio incluye `docker-compose.prod.yml` y `.env.prod.example`, pero **no se ha desplegado** porque aún no hay servidor/IP pública.
- Cifrado de datos en reposo, copias de seguridad cifradas y prueba de restauración.
- Integraciones reales de mensajería y facturación fiscal, firma digital/OTP y pasarela de pagos.
- Pruebas de rendimiento/disponibilidad/concurrencia, cobertura unitaria e integración del 80% en endpoints críticos, escaneos SAST/DAST y completar/revisar el contrato OpenAPI.
- UAT, auditoría de seguridad y validación clínica/legal antes de cualquier uso real.

## Despliegue de producción

Usa `.env.prod.example` como plantilla para crear `.env.prod` localmente; no compartas ni subas `.env.prod`. Configura secretos nuevos y una contraseña PostgreSQL fuerte. Cuando exista el VPS, abre solo los puertos web necesarios (80/443); la base y API no deben exponerse directamente a Internet. Para iniciar en el servidor, desde la raíz del repositorio:

```bash
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
```

Verifica el estado con `docker compose -f docker-compose.prod.yml --env-file .env.prod ps` y los logs con `docker compose -f docker-compose.prod.yml --env-file .env.prod logs api`. No publiques logs o archivos de entorno que incluyan secretos.

## Demo pública en Render

El archivo [`render.yaml`](render.yaml) declara una API Go, la web estática Vite y PostgreSQL administrado. Para una demo académica, los servicios están configurados en el plan Free. Desde Render Dashboard selecciona **New → Blueprint**, conecta este repositorio de GitHub y confirma la creación de los tres recursos. Render genera `JWT_SECRET`, `MFA_ENCRYPTION_KEY` y la contraseña de PostgreSQL; no copies los secretos locales de `.env.prod` a Render. La API inicializa el esquema si la base está vacía.

Cuando los servicios terminen de desplegarse, abre la URL pública del servicio `sigo-dental-web`; la API se comunica con la URL del servicio `sigo-dental-api`. El API tiene health check en `/api/health`. Las bases de datos Free de Render expiran a los 30 días, tienen 1 GB y no incluyen backups; esta configuración sirve para una demostración temporal, no para conservar datos. Usa únicamente datos ficticios y revisa el consumo/límites en el Dashboard.

## Documentación técnica

Existe un borrador OpenAPI 3.0 en `backend/openapi.yaml` y notas UAT/seguridad en [`docs/FASE4-UAT-Y-SEGURIDAD.md`](docs/FASE4-UAT-Y-SEGURIDAD.md). El contrato OpenAPI todavía no cubre todos los endpoints.
