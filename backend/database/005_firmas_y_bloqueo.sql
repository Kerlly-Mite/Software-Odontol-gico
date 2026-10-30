-- Firma electrónica capturada para consentimientos y protección de registros.
-- Las notas clínicas permanecen append-only; el plazo de bloqueo se expone como
-- política configurable en la API, con 24 horas por defecto.
CREATE OR REPLACE FUNCTION reject_signed_consent_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status = 'signed' THEN
        RAISE EXCEPTION 'El consentimiento firmado es inmutable.';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS informed_consents_signed_immutable ON informed_consents;
CREATE TRIGGER informed_consents_signed_immutable
    BEFORE UPDATE OR DELETE ON informed_consents
    FOR EACH ROW EXECUTE FUNCTION reject_signed_consent_mutation();
