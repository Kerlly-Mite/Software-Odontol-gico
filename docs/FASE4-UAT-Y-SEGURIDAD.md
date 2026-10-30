# Fase 4: UAT y revisión de seguridad

Estado de esta revisión: **en curso; no autorizado para producción**.

## Evidencia UAT recibida

| Caso | Evidencia | Estado |
| --- | --- | --- |
| Registrar insumo y stock inicial | Captura de inventario con existencia inicial y movimiento | Aprobado |
| Registrar consumo manual | Captura con saldo actualizado y alerta de stock bajo | Aprobado |
| Configurar regla de descargo | Regla guardada para el procedimiento `control` | Parcial |
| Completar una cita | Cita de `Control y profilaxis` aparece como `Atendido`; el reporte cuenta una atendida | Aprobado |
| Descargar insumo automáticamente | La regla `control` no es idéntica a `Control y profilaxis`; confirmar un nuevo caso con el nombre exacto y revisar el movimiento | Pendiente |
| Crear regla de honorarios | Captura indica que se guardó una regla fija de USD 30 | Aprobado |
| Ver el honorario calculado | La captura previa a guardar la regla aún muestra USD 0; recargar el reporte y confirmar el cálculo por un procedimiento atendido | Pendiente |

## Ejecución manual pendiente

1. En **Inventario**, registra una regla para `Control y profilaxis` con una cantidad pequeña de un insumo de prueba.
2. En **Agenda**, crea una cita de demostración con ese mismo motivo y márcala como `Atendido`.
3. Confirma que **Movimientos recientes** muestre el consumo asociado a la cita y que repetir la solicitud no duplique el consumo.
4. En **Reportes**, registra una regla fija de honorarios para el profesional de esa cita, actualiza el rango de fechas y confirma que aparezca el monto esperado.
5. Inicia sesión con los roles de recepción, asistente, odontología y auditoría; confirma que cada rol solo pueda realizar las acciones permitidas.
6. Configura TOTP en una cuenta, confirma inscripción con código válido, cierra sesión y verifica que un código válido permita el acceso y un código inválido lo rechace.
7. Reinicia la API conservando `MFA_ENCRYPTION_KEY` y confirma que la cuenta siga autenticándose; luego comprueba que cambiar esa clave invalida el descifrado como se espera.
8. En la pantalla clínica, confirma el cierre tras 15 minutos sin actividad.
9. Verifica en PostgreSQL que el `audit_log` registra consultas y cambios autorizados, y que no permite `UPDATE` ni `DELETE`.
10. Crea una nota SOAP nueva y confirma que la API la muestra con firma HMAC verificada y `lockedAt` según `CLINICAL_NOTE_LOCK_HOURS` (24 por defecto); confirma que cualquier edición o borrado directo se rechaza y que la corrección se registra como adenda.
11. Crea un consentimiento de un paciente adulto, captura la firma en pantalla y confirma que la lista indica `Firmado` y `signatureVerified: true`; repite con un menor y confirma que solo acepte el tutor legal registrado.
12. Confirma que un consentimiento firmado rechaza cambios y borrado en PostgreSQL. La captura es una firma electrónica de demostración y no sustituye certificado legal ni OTP.

## Revisión de seguridad y bloqueos de producción

- MFA TOTP: implementada para todas las rutas protegidas; **validación runtime, limitación de intentos, recuperación de cuenta y gestión de clave productiva pendientes**.
- Firma HMAC de notas/adendas y captura manuscrita HMAC de consentimientos: implementadas en código; **casos UAT 10–12 pendientes**. No equivalen a firma digital legal certificada.
- TLS 1.3: **pendiente**. Compose es solo para desarrollo local y no configura TLS.
- Audit log: el middleware registra solicitudes autorizadas a endpoints protegidos y la base impide modificar o borrar eventos. El registro de denegaciones y exportaciones requiere completar cobertura y validación en PostgreSQL.
- Cierre por inactividad: el frontend cierra la sesión tras 15 minutos; la expiración y revocación del token en servidor requieren completar validación.
- Contraseñas y secretos de demostración: solo para entorno local; configurar secretos persistentes fuera del repositorio y reemplazar cuentas antes de cualquier despliegue.
- SAST/DAST, revisión OWASP, pruebas de integración con PostgreSQL y cobertura mínima del 80%: **pendientes**.
- Especificación OpenAPI 3.0: hay un borrador en `backend/openapi.yaml`; completar la cobertura de rutas y validarlo antes de aprobar la entrega.
- UAT integral del descargo automático y de comisiones: **pendiente**, según los casos anteriores.

La pantalla de demostración no debe recibir datos reales de pacientes hasta cerrar estos bloqueos y completar la aprobación formal.
