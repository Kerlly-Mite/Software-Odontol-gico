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
| RF-05 | Notas SOAP append-only y adendas. Cada nota nueva recibe HMAC-SHA256 del servidor y expone el vencimiento de la política configurable (`CLINICAL_NOTE_LOCK_HOURS`, 24 por defecto). El esquema bloquea cambios desde la creación, una regla más estricta que 24 horas. El HMAC es una firma técnica de demo, no un certificado de firma digital legal. |
| RF-06 | Consentimiento con captura manuscrita en mouse/pantalla táctil, control de tutor legal para menores e integridad HMAC. OTP y certificado legal requieren proveedor/certificado externo. |
| RF-07 / RF-08 | Presupuestos por fases/sesiones, costos, descuentos y aprobación total/parcial; partidas aprobadas se pueden vincular a citas. |
| RF-09 / RF-10 / RN-03 | Agenda por sillón/profesional, estados de citas y protección contra doble reserva del sillón. |
| RF-11 | No implementado: WhatsApp/SMS/correo requiere cuentas y proveedores externos. |
| RF-12 | Registro básico de cobros; facturación electrónica fiscal y pagos reales no implementados. |
| RF-13 / RF-14 | Reglas de honorarios estimados y descargo de inventario al completar procedimientos. No es liquidación contable/fiscal. |
| RN-01 | Notas firmadas e inmutables, adendas vinculadas y consentimientos firmados protegidos contra cambios. HMAC técnico; certificado legal pendiente. |
| RN-02 | Cierre de caja ciego: recepción registra el efectivo contado antes de ver el efectivo esperado y diferencia. |
| RNF-05 / RNF-06 | Roles, MFA TOTP y auditoría disponibles en el prototipo; faltan revisión de seguridad, recuperación y limitación de intentos. |
| RNF-07 | Diseño responsive implementado; falta validación formal en la matriz de dispositivos y navegadores solicitada. |

### Trabajo futuro / requisitos operativos pendientes

- Conseguir VPS/servidor con IP pública, configurar firewall y dominio, HTTPS/TLS 1.3 y certificados. El repositorio incluye `docker-compose.prod.yml` y `.env.prod.example`, pero **no se ha desplegado** porque aún no hay servidor/IP pública.
- Cifrado de datos en reposo, copias de seguridad cifradas y prueba de restauración.
- Integraciones reales de mensajería, facturación fiscal, OTP y firma digital legal, y pasarela de pagos.
- Pruebas de rendimiento/disponibilidad/concurrencia, cobertura unitaria e integración del 80% en endpoints críticos, escaneos SAST/DAST y completar/revisar el contrato OpenAPI.
- UAT, auditoría de seguridad y validación clínica/legal antes de cualquier uso real.

## Preparación de Docker para servidor

No ejecutar este despliegue hasta que el ingeniero lo autorice y se asigne el servidor. La receta queda documentada para esa etapa.

Usa `.env.prod.example` como plantilla para crear `.env.prod` localmente; no compartas ni subas `.env.prod`. Configura secretos nuevos y una contraseña PostgreSQL fuerte. Cuando exista el VPS, abre solo los puertos web necesarios (80/443); la base y API no deben exponerse directamente a Internet. Para iniciar en el servidor, desde la raíz del repositorio:

```bash
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
```

Verifica el estado con `docker compose -f docker-compose.prod.yml --env-file .env.prod ps` y los logs con `docker compose -f docker-compose.prod.yml --env-file .env.prod logs api`. No publiques logs o archivos de entorno que incluyan secretos.

## Despliegue público

El despliegue público está **en espera de nueva orden del ingeniero**. No crees recursos ni elimines bases de datos de Render para esta entrega: el espacio ya tiene una base gratuita activa que pertenece a otro proyecto y Render solo permite una base PostgreSQL Free activa por espacio. El archivo `render.yaml` queda como referencia técnica; la publicación requerirá una base autorizada y una nueva instrucción.

## Documentación técnica

Existe un borrador OpenAPI 3.0 en `backend/openapi.yaml` y notas UAT/seguridad en [`docs/FASE4-UAT-Y-SEGURIDAD.md`](docs/FASE4-UAT-Y-SEGURIDAD.md). El contrato OpenAPI todavía no cubre todos los endpoints. Las claves nuevas son `CLINICAL_SIGNING_KEY` (secreto estable de al menos 32 caracteres) y `CLINICAL_NOTE_LOCK_HOURS` (entero de 1 a 720; por defecto 24). Debe conservarse la clave de firma para verificar firmas previas.
