package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	_ "embed"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

//go:embed database/001_fases_1_2.sql
var baseSchemaMigration string

//go:embed database/002_fase3.sql
var fase3Migration string

//go:embed database/003_fase4.sql
var fase4Migration string

//go:embed database/004_fase4_correcciones.sql
var fase4CorrectionsMigration string

var (
	db       *sql.DB
	clinicID int64
	userID   int64
	jwtKey   []byte
	mfaKey   []byte
	chairIDs = map[string]int64{}
)

type principal struct {
	UserID   int64
	ClinicID int64
	Role     string
	Name     string
	Email    string
}

type claims struct {
	UserID   int64  `json:"uid"`
	ClinicID int64  `json:"cid"`
	Role     string `json:"role"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

type loginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

type Patient struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Doc        string `json:"doc"`
	Age        int    `json:"age"`
	Birth      string `json:"birth"`
	Phone      string `json:"phone"`
	Emergency  string `json:"emergency"`
	Email      string `json:"email"`
	Blood      string `json:"blood"`
	Guardian   string `json:"guardian"`
	Allergies  string `json:"allergies"`
	Conditions string `json:"conditions"`
	Insurance  string `json:"insurance"`
	Last       string `json:"last"`
}

type Appointment struct {
	ID        int64  `json:"id"`
	Patient   string `json:"patient"`
	Doctor    string `json:"doctor"`
	Procedure string `json:"procedure"`
	Date      string `json:"date"`
	Time      string `json:"time"`
	Chair     string `json:"chair"`
	Status    string `json:"status"`
}

type Invoice struct {
	ID      string  `json:"id"`
	Patient string  `json:"patient"`
	Concept string  `json:"concept"`
	Total   float64 `json:"total"`
	Paid    float64 `json:"paid"`
	Date    string  `json:"date"`
	State   string  `json:"state"`
}

type PatientInput struct {
	Name       string `json:"name"`
	Doc        string `json:"doc"`
	Age        int    `json:"age"`
	Birth      string `json:"birth"`
	Phone      string `json:"phone"`
	Emergency  string `json:"emergency"`
	Email      string `json:"email"`
	Blood      string `json:"blood"`
	Insurance  string `json:"insurance"`
	Guardian   string `json:"guardian"`
	Allergies  string `json:"allergies"`
	Conditions string `json:"conditions"`
}

type AppointmentInput struct {
	Patient    string `json:"patient"`
	Doctor     string `json:"doctor"`
	Procedure  string `json:"procedure"`
	PlanItemID int64  `json:"planItemId"`
	Date       string `json:"date"`
	Time       string `json:"time"`
	Chair      string `json:"chair"`
	Status     string `json:"status"`
}

type InvoiceInput struct {
	Patient string  `json:"patient"`
	Concept string  `json:"concept"`
	Total   float64 `json:"total"`
	Paid    float64 `json:"paid"`
	Method  string  `json:"method"`
}

func main() {
	ctx := context.Background()
	secret := env("JWT_SECRET", "sigo-local-dev-jwt-secret-change-before-production-2026")
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET debe tener al menos 32 caracteres")
	}
	jwtKey = []byte(secret)
	mfaKey = deriveMFAKey(env("MFA_ENCRYPTION_KEY", "sigo-local-dev-mfa-encryption-key-change-before-production"))
	var err error
	url := env("DATABASE_URL", "postgres://sigo:sigo_local_dev_only@db:5432/sigodental?sslmode=disable")
	db, err = sql.Open("pgx", url)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)

	for attempt := 1; attempt <= 30; attempt++ {
		if err = db.PingContext(ctx); err == nil {
			break
		}
		log.Printf("PostgreSQL no disponible (intento %d/30): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal("no se pudo conectar a PostgreSQL: ", err)
	}
	if err := bootstrap(ctx); err != nil {
		log.Fatal("inicialización de SIGO-Dental: ", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", health)
	mux.HandleFunc("POST /api/auth/login", login)
	mux.Handle("POST /api/auth/mfa/setup", secureSetup([]string{"admin_ti", "recepcionista", "odontologo", "asistente", "auditor"}, http.HandlerFunc(startMFASetup)))
	mux.Handle("POST /api/auth/mfa/verify-setup", secureSetup([]string{"admin_ti", "recepcionista", "odontologo", "asistente", "auditor"}, http.HandlerFunc(verifyMFASetup)))
	mux.Handle("GET /api/patients", secure([]string{"admin_ti", "recepcionista", "odontologo", "asistente"}, http.HandlerFunc(listPatients)))
	mux.Handle("POST /api/patients", secure([]string{"admin_ti", "recepcionista"}, http.HandlerFunc(createPatient)))
	mux.Handle("PUT /api/patients/{patientID}", secure([]string{"admin_ti", "recepcionista"}, http.HandlerFunc(updatePatient)))
	mux.Handle("GET /api/treatment-plans", secure([]string{"admin_ti", "recepcionista", "odontologo", "asistente", "auditor"}, http.HandlerFunc(listTreatmentPlans)))
	mux.Handle("POST /api/treatment-plans", secure([]string{"admin_ti", "odontologo"}, http.HandlerFunc(createTreatmentPlan)))
	mux.Handle("POST /api/treatment-plans/{id}/approval", secure([]string{"admin_ti", "recepcionista", "odontologo"}, http.HandlerFunc(approveTreatmentPlan)))
	mux.Handle("POST /api/cash/close", secure([]string{"admin_ti", "recepcionista"}, http.HandlerFunc(closeCash)))
	mux.Handle("GET /api/appointments", secure([]string{"admin_ti", "recepcionista", "odontologo", "asistente"}, http.HandlerFunc(listAppointments)))
	mux.Handle("POST /api/appointments", secure([]string{"admin_ti", "recepcionista", "odontologo"}, http.HandlerFunc(createAppointment)))
	mux.Handle("PATCH /api/appointments/{id}/status", secure([]string{"admin_ti", "recepcionista", "odontologo"}, http.HandlerFunc(updateAppointmentStatus)))
	mux.Handle("GET /api/invoices", secure([]string{"admin_ti", "recepcionista", "auditor"}, http.HandlerFunc(listInvoices)))
	mux.Handle("POST /api/invoices", secure([]string{"admin_ti", "recepcionista"}, http.HandlerFunc(createInvoice)))
	mux.Handle("GET /api/inventory/products", secure([]string{"admin_ti", "recepcionista", "asistente", "auditor"}, http.HandlerFunc(listInventoryProducts)))
	mux.Handle("POST /api/inventory/products", secure([]string{"admin_ti", "recepcionista"}, http.HandlerFunc(createInventoryProduct)))
	mux.Handle("GET /api/inventory/movements", secure([]string{"admin_ti", "recepcionista", "asistente", "auditor"}, http.HandlerFunc(listInventoryMovements)))
	mux.Handle("POST /api/inventory/movements", secure([]string{"admin_ti", "recepcionista", "asistente"}, http.HandlerFunc(createInventoryMovement)))
	mux.Handle("GET /api/inventory/consumption-rules", secure([]string{"admin_ti"}, http.HandlerFunc(listInventoryRules)))
	mux.Handle("POST /api/inventory/consumption-rules", secure([]string{"admin_ti"}, http.HandlerFunc(createInventoryRule)))
	mux.Handle("DELETE /api/inventory/consumption-rules/{id}", secure([]string{"admin_ti"}, http.HandlerFunc(deleteInventoryRule)))
	mux.Handle("GET /api/commissions/rules", secure([]string{"admin_ti", "auditor"}, http.HandlerFunc(listCommissionRules)))
	mux.Handle("POST /api/commissions/rules", secure([]string{"admin_ti"}, http.HandlerFunc(createCommissionRule)))
	mux.Handle("DELETE /api/commissions/rules/{id}", secure([]string{"admin_ti"}, http.HandlerFunc(deleteCommissionRule)))
	mux.Handle("GET /api/reports/summary", secure([]string{"admin_ti", "auditor"}, http.HandlerFunc(reportSummary)))
	mux.Handle("GET /api/patients/{patientID}/clinical-notes", secure([]string{"admin_ti", "odontologo", "asistente"}, http.HandlerFunc(listClinicalNotes)))
	mux.Handle("POST /api/patients/{patientID}/clinical-notes", secure([]string{"admin_ti", "odontologo"}, http.HandlerFunc(createClinicalNote)))
	mux.Handle("POST /api/patients/{patientID}/clinical-notes/{noteID}/amendments", secure([]string{"admin_ti", "odontologo"}, http.HandlerFunc(createClinicalAmendment)))
	mux.Handle("GET /api/patients/{patientID}/odontograms", secure([]string{"admin_ti", "odontologo", "asistente"}, http.HandlerFunc(getOdontogram)))
	mux.Handle("POST /api/patients/{patientID}/odontograms", secure([]string{"admin_ti", "odontologo"}, http.HandlerFunc(saveOdontogram)))
	mux.Handle("GET /api/patients/{patientID}/periodontogram", secure([]string{"admin_ti", "odontologo", "asistente"}, http.HandlerFunc(getPeriodontogram)))
	mux.Handle("PUT /api/patients/{patientID}/periodontogram", secure([]string{"admin_ti", "odontologo"}, http.HandlerFunc(savePeriodontogram)))
	mux.Handle("GET /api/patients/{patientID}/prescriptions", secure([]string{"admin_ti", "odontologo", "asistente"}, http.HandlerFunc(listPrescriptions)))
	mux.Handle("POST /api/patients/{patientID}/prescriptions", secure([]string{"admin_ti", "odontologo"}, http.HandlerFunc(createPrescription)))
	mux.Handle("GET /api/patients/{patientID}/consents", secure([]string{"admin_ti", "odontologo", "asistente"}, http.HandlerFunc(listConsents)))
	mux.Handle("POST /api/patients/{patientID}/consents", secure([]string{"admin_ti", "odontologo"}, http.HandlerFunc(createConsent)))
	mux.HandleFunc("OPTIONS /", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	addr := ":" + env("PORT", "8080")
	log.Printf("SIGO-Dental API escuchando en %s", addr)
	log.Fatal(http.ListenAndServe(addr, cors(mux)))
}

// bootstrap adds compatibility columns for databases created by the first prototype
// and creates the local clinic/user/operatory records needed by these dev endpoints.
func bootstrap(ctx context.Context) error {
	var schemaExists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('public.clinics') IS NOT NULL`).Scan(&schemaExists); err != nil {
		return fmt.Errorf("no se pudo verificar el esquema inicial: %w", err)
	}
	if !schemaExists {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("no se pudo iniciar el esquema inicial: %w", err)
		}
		for _, statement := range splitSQLStatements(baseSchemaMigration) {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				tx.Rollback()
				return fmt.Errorf("esquema inicial: %w", err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("no se pudo confirmar el esquema inicial: %w", err)
		}
	}
	migrations := []struct {
		name string
		sql  string
	}{{"Fase 3", fase3Migration}, {"Fase 4", fase4Migration}, {"Correcciones Fase 4", fase4CorrectionsMigration}}
	for _, migration := range migrations {
		for _, statement := range splitSQLStatements(migration.sql) {
			if _, err := db.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migración %s: %w", migration.name, err)
			}
		}
	}
	statements := []string{
		`ALTER TABLE patients ADD COLUMN IF NOT EXISTS age_years SMALLINT CHECK (age_years IS NULL OR age_years BETWEEN 0 AND 125)`,
		`ALTER TABLE invoices ADD COLUMN IF NOT EXISTS concept TEXT NOT NULL DEFAULT 'Atención odontológica'`,
	}
	for _, q := range statements {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	if _, err := db.ExecContext(ctx, `CREATE OR REPLACE FUNCTION reject_audit_log_mutation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'El registro de auditoría es inmutable.'; END; $$`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `DROP TRIGGER IF EXISTS audit_log_immutable ON audit_log`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER audit_log_immutable BEFORE UPDATE OR DELETE ON audit_log FOR EACH ROW EXECUTE FUNCTION reject_audit_log_mutation()`); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, `SELECT id FROM clinics ORDER BY id LIMIT 1`).Scan(&clinicID); errors.Is(err, sql.ErrNoRows) {
		if err := db.QueryRowContext(ctx, `INSERT INTO clinics(name, timezone) VALUES ('Clínica Dental Sonrisa', 'America/Guayaquil') RETURNING id`).Scan(&clinicID); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	adminID, err := ensureDemoUser(ctx, "admin@sigo.local", "Administradora SIGO", "admin_ti")
	if err != nil {
		return err
	}
	userID = adminID
	for _, name := range []string{"Sillón 1", "Sillón 2"} {
		var id int64
		err := db.QueryRowContext(ctx, `SELECT id FROM chairs WHERE clinic_id=$1 AND name=$2`, clinicID, name).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			err = db.QueryRowContext(ctx, `INSERT INTO chairs(clinic_id,name) VALUES ($1,$2) RETURNING id`, clinicID, name).Scan(&id)
		}
		if err != nil {
			return err
		}
		chairIDs[name] = id
	}
	for _, account := range []struct{ email, name, role string }{
		{"recepcion@sigo.local", "Camila Torres", "recepcionista"},
		{"valeria.ruiz@sigo.local", "Dra. Valeria Ruiz", "odontologo"},
		{"daniel.paredes@sigo.local", "Dr. Daniel Paredes", "odontologo"},
		{"asistente@sigo.local", "Andrea Mite", "asistente"},
		{"auditor@sigo.local", "Auditor SIGO", "auditor"},
	} {
		if _, err := ensureDemoUser(ctx, account.email, account.name, account.role); err != nil {
			return err
		}
	}
	return nil
}

// splitSQLStatements splits at semicolons outside quoted strings, comments, and
// PostgreSQL dollar-quoted procedure bodies.
func splitSQLStatements(script string) []string {
	statements := make([]string, 0)
	start := 0
	var dollarTag string
	var singleQuoted, doubleQuoted, lineComment, blockComment bool
	for i := 0; i < len(script); {
		if dollarTag != "" {
			if strings.HasPrefix(script[i:], dollarTag) {
				i += len(dollarTag)
				dollarTag = ""
			} else {
				i++
			}
			continue
		}
		if lineComment {
			if script[i] == '\n' {
				lineComment = false
			}
			i++
			continue
		}
		if blockComment {
			if i+1 < len(script) && script[i] == '*' && script[i+1] == '/' {
				blockComment = false
				i += 2
			} else {
				i++
			}
			continue
		}
		if singleQuoted {
			if script[i] == '\'' {
				if i+1 < len(script) && script[i+1] == '\'' {
					i += 2
				} else {
					singleQuoted = false
					i++
				}
			} else {
				i++
			}
			continue
		}
		if doubleQuoted {
			if script[i] == '"' {
				if i+1 < len(script) && script[i+1] == '"' {
					i += 2
				} else {
					doubleQuoted = false
					i++
				}
			} else {
				i++
			}
			continue
		}
		if i+1 < len(script) && script[i] == '-' && script[i+1] == '-' {
			lineComment = true
			i += 2
			continue
		}
		if i+1 < len(script) && script[i] == '/' && script[i+1] == '*' {
			blockComment = true
			i += 2
			continue
		}
		switch script[i] {
		case '\'':
			singleQuoted = true
			i++
		case '"':
			doubleQuoted = true
			i++
		case '$':
			if tagEnd := dollarQuoteTagEnd(script, i); tagEnd > i {
				dollarTag = script[i:tagEnd]
				i = tagEnd
			} else {
				i++
			}
		case ';':
			if statement := strings.TrimSpace(script[start : i+1]); statement != "" {
				statements = append(statements, statement)
			}
			i++
			start = i
		default:
			i++
		}
	}
	if statement := strings.TrimSpace(script[start:]); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}

func dollarQuoteTagEnd(script string, start int) int {
	if start+1 >= len(script) {
		return 0
	}
	if script[start+1] == '$' {
		return start + 2
	}
	if !((script[start+1] >= 'a' && script[start+1] <= 'z') || (script[start+1] >= 'A' && script[start+1] <= 'Z') || script[start+1] == '_') {
		return 0
	}
	for i := start + 2; i < len(script); i++ {
		c := script[i]
		if c == '$' {
			return i + 1
		}
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return 0
		}
	}
	return 0
}

func ensureDemoUser(ctx context.Context, email, name, role string) (int64, error) {
	var id int64
	var hash string
	err := db.QueryRowContext(ctx, `SELECT id,password_hash FROM users WHERE email=$1`, email).Scan(&id, &hash)
	if err == nil {
		if hash == "development-only-no-login" {
			newHash, hashErr := bcrypt.GenerateFromPassword([]byte("Demo123!"), bcrypt.DefaultCost)
			if hashErr != nil {
				return 0, hashErr
			}
			if _, err = db.ExecContext(ctx, `UPDATE users SET password_hash=$1,full_name=$2 WHERE id=$3`, string(newHash), name, id); err != nil {
				return 0, err
			}
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		newHash, hashErr := bcrypt.GenerateFromPassword([]byte("Demo123!"), bcrypt.DefaultCost)
		if hashErr != nil {
			return 0, hashErr
		}
		err = db.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,full_name) VALUES ($1,$2,$3) RETURNING id`, email, string(newHash), name).Scan(&id)
	} else {
		return 0, err
	}
	if err != nil {
		return 0, err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO clinic_members(clinic_id,user_id,role,specialty) VALUES ($1,$2,$3,$4) ON CONFLICT (clinic_id,user_id) DO UPDATE SET role=EXCLUDED.role,specialty=EXCLUDED.specialty`, clinicID, id, role, specialtyForRole(role))
	return id, err
}

func specialtyForRole(role string) string {
	if role == "odontologo" {
		return "Odontología general"
	}
	return ""
}

func health(w http.ResponseWriter, r *http.Request) {
	if err := db.PingContext(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "PostgreSQL no disponible")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "sigo-dental-api"})
}

func login(w http.ResponseWriter, r *http.Request) {
	var in loginInput
	if !decode(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	var p principal
	var passwordHash string
	var mfaEnabled bool
	var encryptedSecret string
	err := db.QueryRowContext(r.Context(), `SELECT u.id,cm.clinic_id,cm.role,u.full_name,u.email,u.password_hash,u.mfa_enabled,COALESCE(u.mfa_secret,'') FROM users u JOIN clinic_members cm ON cm.user_id=u.id WHERE lower(u.email)=lower($1) AND cm.clinic_id=$2 AND u.active AND cm.active`, in.Email, clinicID).Scan(&p.UserID, &p.ClinicID, &p.Role, &p.Name, &p.Email, &passwordHash, &mfaEnabled, &encryptedSecret)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(in.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "Correo o contraseña incorrectos")
		return
	}
	if mfaEnabled {
		secret, decryptErr := decryptMFASecret(encryptedSecret)
		if decryptErr != nil || !validTOTP(secret, in.Code, time.Now()) {
			writeJSON(w, http.StatusForbidden, map[string]any{"status": http.StatusForbidden, "message": "Ingresa el código de seis dígitos de tu aplicación autenticadora", "mfa_required": true})
			return
		}
	}
	issueSession(w, p, mfaEnabled)
}

func issueSession(w http.ResponseWriter, p principal, mfaEnabled bool) {
	now := time.Now()
	tokenClaims := claims{UserID: p.UserID, ClinicID: p.ClinicID, Role: p.Role, Name: p.Name, Email: p.Email, RegisteredClaims: jwt.RegisteredClaims{Issuer: "sigo-dental-api", IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(8 * time.Hour))}}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims)
	signed, err := token.SignedString(jwtKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudo iniciar sesión")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": signed, "user": map[string]any{"id": p.UserID, "name": p.Name, "email": p.Email, "role": p.Role, "mfaEnabled": mfaEnabled}})
}

func deriveMFAKey(secret string) []byte { sum := sha256.Sum256([]byte(secret)); return sum[:] }

func encryptMFASecret(secret string) (string, error) {
	block, err := aes.NewCipher(mfaKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(secret), []byte("sigo-dental-totp"))
	return base64.StdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func decryptMFASecret(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(mfaKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("secreto MFA inválido")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], []byte("sigo-dental-totp"))
	return string(plain), err
}

func totpCode(secret string, unixSeconds int64) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", err
	}
	counter := uint64(unixSeconds / 30)
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", value%1000000), nil
}

func validTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	for delta := int64(-1); delta <= 1; delta++ {
		expected, err := totpCode(secret, now.Unix()+delta*30)
		if err == nil && subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

func mfaSetupPayload(p principal, secret string) map[string]string {
	label := url.QueryEscape("SIGO-Dental:" + p.Email)
	issuer := url.QueryEscape("SIGO-Dental")
	return map[string]string{"secret": secret, "account": p.Email, "otpauthUrl": fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30", label, secret, issuer)}
}

func startMFASetup(w http.ResponseWriter, r *http.Request) {
	p, _ := r.Context().Value(principalContextKey{}).(principal)
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar la configuración de MFA")
		return
	}
	defer tx.Rollback()
	var enabled bool
	var encrypted string
	if err = tx.QueryRowContext(r.Context(), `SELECT mfa_enabled,COALESCE(mfa_secret,'') FROM users WHERE id=$1 FOR UPDATE`, p.UserID).Scan(&enabled, &encrypted); err != nil {
		writeError(w, 500, "No se pudo consultar la cuenta")
		return
	}
	if enabled {
		writeError(w, 409, "MFA ya está activo para esta cuenta")
		return
	}
	var secret string
	if encrypted != "" {
		secret, err = decryptMFASecret(encrypted)
		if err != nil {
			writeError(w, 500, "No se pudo leer el secreto de MFA")
			return
		}
	} else {
		randomSecret := make([]byte, 20)
		if _, err = rand.Read(randomSecret); err != nil {
			writeError(w, 500, "No se pudo generar el secreto MFA")
			return
		}
		secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomSecret)
		encrypted, err = encryptMFASecret(secret)
		if err != nil {
			writeError(w, 500, "No se pudo proteger el secreto MFA")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE users SET mfa_secret=$1 WHERE id=$2`, encrypted, p.UserID); err != nil {
			writeError(w, 500, "No se pudo guardar el secreto MFA")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeError(w, 500, "No se pudo guardar la configuración MFA")
		return
	}
	writeJSON(w, 200, mfaSetupPayload(p, secret))
}

func verifyMFASetup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	p, _ := r.Context().Value(principalContextKey{}).(principal)
	var enabled bool
	var encrypted string
	if err := db.QueryRowContext(r.Context(), `SELECT mfa_enabled,COALESCE(mfa_secret,'') FROM users WHERE id=$1`, p.UserID).Scan(&enabled, &encrypted); err != nil {
		writeError(w, 500, "No se pudo consultar la cuenta")
		return
	}
	if enabled {
		writeJSON(w, 200, map[string]bool{"mfaEnabled": true})
		return
	}
	secret, err := decryptMFASecret(encrypted)
	if err != nil || !validTOTP(secret, in.Code, time.Now()) {
		writeError(w, 400, "El código no es válido o ya venció; revisa tu aplicación e inténtalo otra vez")
		return
	}
	if _, err = db.ExecContext(r.Context(), `UPDATE users SET mfa_enabled=TRUE WHERE id=$1 AND mfa_secret IS NOT NULL`, p.UserID); err != nil {
		writeError(w, 500, "No se pudo activar MFA")
		return
	}
	writeJSON(w, 200, map[string]bool{"mfaEnabled": true})
}

type principalContextKey struct{}

func secure(roles []string, next http.Handler) http.Handler {
	return requireAuth(requireRole(roles, requireMFA(auditRequests(next))))
}

func secureSetup(roles []string, next http.Handler) http.Handler {
	return requireAuth(requireRole(roles, auditRequests(next)))
}

func requireMFA(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := r.Context().Value(principalContextKey{}).(principal)
		if !ok {
			writeError(w, http.StatusUnauthorized, "Inicia sesión para continuar")
			return
		}
		var enabled bool
		if err := db.QueryRowContext(r.Context(), `SELECT mfa_enabled FROM users WHERE id=$1 AND active`, p.UserID).Scan(&enabled); err != nil {
			writeError(w, 500, "No se pudo comprobar el segundo factor")
			return
		}
		if !enabled {
			writeJSON(w, http.StatusForbidden, map[string]any{"status": http.StatusForbidden, "message": "Configura MFA para acceder a los módulos del sistema", "mfa_setup_required": true})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func auditRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		p, ok := r.Context().Value(principalContextKey{}).(principal)
		if !ok {
			return
		}
		action := map[string]string{http.MethodGet: "READ", http.MethodPost: "CREATE", http.MethodPut: "UPDATE", http.MethodPatch: "UPDATE", http.MethodDelete: "DELETE"}[r.Method]
		if action == "" {
			action = r.Method
		}
		route := r.Pattern
		if route == "" {
			route = r.URL.Path
		}
		if i := strings.Index(route, "/api/"); i >= 0 {
			route = route[i+5:]
		}
		segments := strings.Split(strings.Trim(route, "/"), "/")
		entity := strings.Join(segments, ".")
		entityID := r.PathValue("patientID")
		if entityID == "" {
			entityID = r.PathValue("id")
		}
		if entityID == "" {
			entityID = "collection"
		}
		var sourceIP any
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			sourceIP = host
		}
		details, _ := json.Marshal(map[string]string{"method": r.Method, "path": r.URL.Path})
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if _, err := db.ExecContext(ctx, `INSERT INTO audit_log(clinic_id,actor_id,action,entity_type,entity_id,source_ip,details) VALUES($1,$2,$3,$4,$5,$6,$7::jsonb)`, p.ClinicID, p.UserID, action, entity, entityID, sourceIP, string(details)); err != nil {
			log.Printf("no se pudo registrar evento de auditoría: %v", err)
		}
	})
}

func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "Inicia sesión para continuar")
			return
		}
		parsed, err := jwt.ParseWithClaims(parts[1], &claims{}, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("algoritmo de firma inválido")
			}
			return jwtKey, nil
		}, jwt.WithIssuer("sigo-dental-api"))
		if err != nil || !parsed.Valid {
			writeError(w, http.StatusUnauthorized, "Sesión inválida o vencida")
			return
		}
		c, ok := parsed.Claims.(*claims)
		if !ok {
			writeError(w, http.StatusUnauthorized, "Sesión inválida")
			return
		}
		p := principal{UserID: c.UserID, ClinicID: c.ClinicID, Role: c.Role, Name: c.Name, Email: c.Email}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, p)))
	})
}

func requireRole(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := r.Context().Value(principalContextKey{}).(principal)
		if !ok {
			writeError(w, http.StatusUnauthorized, "Inicia sesión para continuar")
			return
		}
		for _, role := range allowed {
			if p.Role == role {
				next.ServeHTTP(w, r)
				return
			}
		}
		writeError(w, http.StatusForbidden, "Tu rol no tiene permiso para esta operación")
	})
}

func actorID(r *http.Request) int64 {
	if p, ok := r.Context().Value(principalContextKey{}).(principal); ok {
		return p.UserID
	}
	return userID
}

// Phase 2 clinical record handlers. Every route first verifies that the patient
// belongs to the active clinic; clinical notes are append-only by schema.
func clinicalPatientID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("patientID"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "Paciente inválido")
		return 0, false
	}
	var found int64
	if err = db.QueryRowContext(r.Context(), `SELECT id FROM patients WHERE id=$1 AND clinic_id=$2 AND active`, id, clinicID).Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "Paciente no encontrado")
			return 0, false
		}
		writeError(w, http.StatusInternalServerError, "No se pudo consultar el paciente")
		return 0, false
	}
	return found, true
}

func encounterFor(ctx context.Context, tx *sql.Tx, patientID, providerID int64) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM encounters WHERE patient_id=$1 AND provider_id=$2 AND status='open' ORDER BY started_at DESC LIMIT 1`, patientID, providerID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO encounters(patient_id,provider_id,status) VALUES($1,$2,'open') RETURNING id`, patientID, providerID).Scan(&id)
	return id, err
}

type soapInput struct {
	Subjective string `json:"subjective"`
	Objective  string `json:"objective"`
	Assessment string `json:"assessment"`
	Plan       string `json:"plan"`
}

func createClinicalNote(w http.ResponseWriter, r *http.Request) {
	patientID, ok := clinicalPatientID(w, r)
	if !ok {
		return
	}
	var in soapInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeError(w, 400, "Datos de nota inválidos")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar el registro")
		return
	}
	defer tx.Rollback()
	enc, err := encounterFor(r.Context(), tx, patientID, actorID(r))
	if err != nil {
		writeError(w, 500, "No se pudo crear la atención")
		return
	}
	var id int64
	var created time.Time
	err = tx.QueryRowContext(r.Context(), `INSERT INTO clinical_notes(encounter_id,author_id,subjective,objective,assessment,plan,signature_ref) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,created_at`, enc, actorID(r), in.Subjective, in.Objective, in.Assessment, in.Plan, fmt.Sprintf("session:%d", actorID(r))).Scan(&id, &created)
	if err != nil {
		writeError(w, 500, "No se pudo guardar la nota")
		return
	}
	if tx.Commit() != nil {
		writeError(w, 500, "No se pudo confirmar la nota")
		return
	}
	var author string
	_ = db.QueryRowContext(r.Context(), `SELECT full_name FROM users WHERE id=$1`, actorID(r)).Scan(&author)
	writeJSON(w, 201, map[string]any{"id": id, "patient": patientID, "text": soapText(in), "date": created.Format("02/01/2006 15:04"), "author": author})
}

func createClinicalAmendment(w http.ResponseWriter, r *http.Request) {
	patientID, ok := clinicalPatientID(w, r)
	if !ok {
		return
	}
	noteID, err := strconv.ParseInt(r.PathValue("noteID"), 10, 64)
	if err != nil || noteID < 1 {
		writeError(w, 400, "Nota original inválida")
		return
	}
	var in soapInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(soapText(in)) == "" {
		writeError(w, 400, "La adenda debe incluir al menos un cambio o aclaración")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar la adenda")
		return
	}
	defer tx.Rollback()
	var encounterID int64
	if err = tx.QueryRowContext(r.Context(), `SELECT n.encounter_id FROM clinical_notes n JOIN encounters e ON e.id=n.encounter_id WHERE n.id=$1 AND e.patient_id=$2 FOR SHARE`, noteID, patientID).Scan(&encounterID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, 404, "La nota original no pertenece a este paciente")
		} else {
			writeError(w, 500, "No se pudo consultar la nota original")
		}
		return
	}
	var id int64
	var created time.Time
	err = tx.QueryRowContext(r.Context(), `INSERT INTO clinical_notes(encounter_id,author_id,subjective,objective,assessment,plan,signature_ref,amendment_of) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,created_at`, encounterID, actorID(r), in.Subjective, in.Objective, in.Assessment, in.Plan, fmt.Sprintf("session:%d", actorID(r)), noteID).Scan(&id, &created)
	if err != nil {
		writeError(w, 500, "No se pudo guardar la adenda")
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, 500, "No se pudo confirmar la adenda")
		return
	}
	var author string
	_ = db.QueryRowContext(r.Context(), `SELECT full_name FROM users WHERE id=$1`, actorID(r)).Scan(&author)
	writeJSON(w, 201, map[string]any{"id": id, "patient": patientID, "amendmentOf": noteID, "text": "ADENDA A NOTA #" + strconv.FormatInt(noteID, 10) + "\n" + soapText(in), "date": created.Format("02/01/2006 15:04"), "author": author})
}

func soapText(n soapInput) string {
	parts := []string{}
	for _, p := range []struct{ k, v string }{{"SUBJETIVO", n.Subjective}, {"OBJETIVO", n.Objective}, {"EVALUACIÓN", n.Assessment}, {"PLAN", n.Plan}} {
		if strings.TrimSpace(p.v) != "" {
			parts = append(parts, p.k+": "+strings.TrimSpace(p.v))
		}
	}
	return strings.Join(parts, "\n")
}
func listClinicalNotes(w http.ResponseWriter, r *http.Request) {
	id, ok := clinicalPatientID(w, r)
	if !ok {
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT n.id,n.subjective,n.objective,n.assessment,n.plan,n.created_at,u.full_name FROM clinical_notes n JOIN encounters e ON e.id=n.encounter_id JOIN users u ON u.id=n.author_id WHERE e.patient_id=$1 ORDER BY n.created_at DESC`, id)
	if err != nil {
		writeError(w, 500, "No se pudieron cargar las notas")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var n soapInput
		var nid int64
		var at time.Time
		var author string
		if rows.Scan(&nid, &n.Subjective, &n.Objective, &n.Assessment, &n.Plan, &at, &author) != nil {
			writeError(w, 500, "No se pudieron leer las notas")
			return
		}
		out = append(out, map[string]any{"id": nid, "patient": id, "text": soapText(n), "date": at.Format("02/01/2006 15:04"), "author": author})
	}
	writeJSON(w, 200, out)
}

type odontogramInput struct {
	Dentition     string            `json:"dentition"`
	Notation      string            `json:"notation"`
	ToothStatus   map[string]string `json:"toothStatus"`
	SurfaceStatus map[string]bool   `json:"surfaceStatus"`
}

func getOdontogram(w http.ResponseWriter, r *http.Request) {
	id, ok := clinicalPatientID(w, r)
	if !ok {
		return
	}
	var oid int64
	var dent, notation string
	err := db.QueryRowContext(r.Context(), `SELECT o.id,o.dentition,o.notation FROM odontograms o JOIN encounters e ON e.id=o.encounter_id WHERE e.patient_id=$1 ORDER BY o.created_at DESC LIMIT 1`, id).Scan(&oid, &dent, &notation)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 200, map[string]any{"dentition": "Permanente", "notation": "FDI", "toothStatus": map[string]string{}, "surfaceStatus": map[string]bool{}})
		return
	}
	if err != nil {
		writeError(w, 500, "No se pudo cargar el odontograma")
		return
	}
	tooth := map[string]string{}
	surface := map[string]bool{}
	rows, err := db.QueryContext(r.Context(), `SELECT tooth_number,surface,finding,note FROM tooth_findings WHERE odontogram_id=$1`, oid)
	if err != nil {
		writeError(w, 500, "No se pudieron cargar los hallazgos")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var n int
		var s, f string
		var note sql.NullString
		if rows.Scan(&n, &s, &f, &note) != nil {
			continue
		}
		switch f {
		case "caries":
			tooth[strconv.Itoa(n)] = "caries"
		case "restoration":
			tooth[strconv.Itoa(n)] = "restaurado"
		case "endodontics":
			tooth[strconv.Itoa(n)] = "endodoncia"
		case "missing":
			tooth[strconv.Itoa(n)] = "ausente"
		case "implant":
			tooth[strconv.Itoa(n)] = "implante"
		case "fixed_prosthesis":
			tooth[strconv.Itoa(n)] = "protesis-fija"
		case "removable_prosthesis":
			tooth[strconv.Itoa(n)] = "protesis-removible"
		case "extraction":
			tooth[strconv.Itoa(n)] = "exodoncia"
		case "sealant":
			tooth[strconv.Itoa(n)] = "sellante"
		case "other":
			if strings.HasPrefix(note.String, "surface:") {
				surface[strconv.Itoa(n)+"-"+strings.TrimPrefix(note.String, "surface:")] = true
			}
		}
	}
	if dent == "primary" {
		dent = "Temporal"
	} else {
		dent = "Permanente"
	}
	if notation == "universal" {
		notation = "Universal"
	} else {
		notation = "FDI"
	}
	writeJSON(w, 200, map[string]any{"dentition": dent, "notation": notation, "toothStatus": tooth, "surfaceStatus": surface})
}
func saveOdontogram(w http.ResponseWriter, r *http.Request) {
	patientID, ok := clinicalPatientID(w, r)
	if !ok {
		return
	}
	var in odontogramInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeError(w, 400, "Odontograma inválido")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar el registro")
		return
	}
	defer tx.Rollback()
	enc, err := encounterFor(r.Context(), tx, patientID, actorID(r))
	if err != nil {
		writeError(w, 500, "No se pudo crear la atención")
		return
	}
	dent := "permanent"
	if in.Dentition == "Temporal" {
		dent = "primary"
	}
	notation := "FDI"
	if strings.EqualFold(in.Notation, "universal") {
		notation = "universal"
	} else if in.Notation != "" && !strings.EqualFold(in.Notation, "FDI") {
		writeError(w, 400, "Notación odontológica inválida")
		return
	}
	var oid int64
	if err = tx.QueryRowContext(r.Context(), `INSERT INTO odontograms(encounter_id,provider_id,dentition,notation) VALUES($1,$2,$3,$4) RETURNING id`, enc, actorID(r), dent, notation).Scan(&oid); err != nil {
		writeError(w, 500, "No se pudo guardar el odontograma")
		return
	}
	for k, v := range in.ToothStatus {
		n, e := strconv.Atoi(k)
		f := map[string]string{"caries": "caries", "restaurado": "restoration", "endodoncia": "endodontics", "implante": "implant", "protesis-fija": "fixed_prosthesis", "protesis-removible": "removable_prosthesis", "exodoncia": "extraction", "sellante": "sealant", "ausente": "missing"}[v]
		valid := validFDITooth(n, dent)
		if e != nil || !valid || f == "" {
			writeError(w, 400, "Diente o hallazgo odontológico inválido")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO tooth_findings(odontogram_id,tooth_number,surface,finding) VALUES($1,$2,'whole_tooth',$3)`, oid, n, f); err != nil {
			writeError(w, 500, "Hallazgo dental inválido")
			return
		}
	}
	surfaceMap := map[string]string{"V": "vestibular", "P": "palatal", "M": "mesial", "D": "distal", "O": "occlusal"}
	for k, v := range in.SurfaceStatus {
		if !v {
			continue
		}
		parts := strings.Split(k, "-")
		if len(parts) != 2 {
			continue
		}
		n, e := strconv.Atoi(parts[0])
		s := surfaceMap[parts[1]]
		if e == nil && validFDITooth(n, dent) && s != "" {
			if _, err = tx.ExecContext(r.Context(), `INSERT INTO tooth_findings(odontogram_id,tooth_number,surface,finding,note) VALUES($1,$2,$3,'other',$4)`, oid, n, s, "surface:"+parts[1]); err != nil {
				writeError(w, 500, "Superficie dental inválida")
				return
			}
		}
	}
	if tx.Commit() != nil {
		writeError(w, 500, "No se pudo confirmar el odontograma")
		return
	}
	writeJSON(w, 201, map[string]any{"saved": true})
}

func validFDITooth(number int, dentition string) bool {
	if dentition == "permanent" {
		return number >= 11 && number <= 18 || number >= 21 && number <= 28 || number >= 31 && number <= 38 || number >= 41 && number <= 48
	}
	return number >= 51 && number <= 55 || number >= 61 && number <= 65 || number >= 71 && number <= 75 || number >= 81 && number <= 85
}

type periodInput struct {
	Readings map[string]periodReadingInput `json:"readings"`
}

type periodReadingInput struct {
	Depth     float64  `json:"depth"`
	Margin    *float64 `json:"margin"`
	Bleeding  bool     `json:"bleeding"`
	Mobility  *int     `json:"mobility"`
	Furcation *int     `json:"furcation"`
}

var periodSites = map[string]string{"v-m": "mesiobuccal", "v-c": "midbuccal", "v-d": "distobuccal", "l-m": "mesiolingual", "l-c": "midlingual", "l-d": "distolingual"}

func getPeriodontogram(w http.ResponseWriter, r *http.Request) {
	id, ok := clinicalPatientID(w, r)
	if !ok {
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT p.tooth_number,p.site,p.probing_depth_mm,p.gingival_margin_mm,p.bleeding,p.mobility,p.furcation FROM periodontal_readings p JOIN encounters e ON e.id=p.encounter_id WHERE e.patient_id=$1 ORDER BY p.recorded_at DESC`, id)
	if err != nil {
		writeError(w, 500, "No se pudo cargar el periodontograma")
		return
	}
	defer rows.Close()
	readings := map[string]map[string]any{}
	for rows.Next() {
		var n int
		var site string
		var depth float64
		var margin sql.NullFloat64
		var bleeding bool
		var mobility, furcation sql.NullInt64
		if rows.Scan(&n, &site, &depth, &margin, &bleeding, &mobility, &furcation) != nil {
			continue
		}
		side, part := "", ""
		switch site {
		case "mesiobuccal":
			side, part = "v", "m"
		case "midbuccal":
			side, part = "v", "c"
		case "distobuccal":
			side, part = "v", "d"
		case "mesiolingual":
			side, part = "l", "m"
		case "midlingual":
			side, part = "l", "c"
		case "distolingual":
			side, part = "l", "d"
		}
		k := strconv.Itoa(n) + "-" + side + "-" + part
		if _, ok := readings[k]; !ok {
			attachment := depth
			var marginValue, mobilityValue, furcationValue any
			if margin.Valid {
				marginValue = margin.Float64
				attachment += margin.Float64
			}
			if mobility.Valid {
				mobilityValue = mobility.Int64
			}
			if furcation.Valid {
				furcationValue = furcation.Int64
			}
			readings[k] = map[string]any{"depth": depth, "margin": marginValue, "bleeding": bleeding, "mobility": mobilityValue, "furcation": furcationValue, "nic": attachment}
		}
	}
	writeJSON(w, 200, map[string]any{"readings": readings})
}
func savePeriodontogram(w http.ResponseWriter, r *http.Request) {
	patientID, ok := clinicalPatientID(w, r)
	if !ok {
		return
	}
	var in periodInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeError(w, 400, "Mediciones inválidas")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar el registro")
		return
	}
	defer tx.Rollback()
	enc, err := encounterFor(r.Context(), tx, patientID, actorID(r))
	if err != nil {
		writeError(w, 500, "No se pudo crear la atención")
		return
	}
	for k, reading := range in.Readings {
		p := strings.Split(k, "-")
		if len(p) != 3 {
			continue
		}
		n, e := strconv.Atoi(p[0])
		depth := reading.Depth
		site := periodSites[p[1]+"-"+p[2]]
		if e != nil || n < 1 || site == "" || depth < 0 || depth > 15 || (reading.Margin != nil && (*reading.Margin < -15 || *reading.Margin > 15)) || (reading.Mobility != nil && (*reading.Mobility < 0 || *reading.Mobility > 3)) || (reading.Furcation != nil && (*reading.Furcation < 0 || *reading.Furcation > 3)) {
			writeError(w, 400, "Una medición periodontal está fuera de rango")
			return
		}
		var margin, mobility, furcation any
		if reading.Margin != nil {
			margin = *reading.Margin
		}
		if reading.Mobility != nil {
			mobility = *reading.Mobility
		}
		if reading.Furcation != nil {
			furcation = *reading.Furcation
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO periodontal_readings(encounter_id,tooth_number,site,probing_depth_mm,gingival_margin_mm,bleeding,mobility,furcation,recorded_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(encounter_id,tooth_number,site) DO UPDATE SET probing_depth_mm=EXCLUDED.probing_depth_mm,gingival_margin_mm=EXCLUDED.gingival_margin_mm,bleeding=EXCLUDED.bleeding,mobility=EXCLUDED.mobility,furcation=EXCLUDED.furcation,recorded_by=EXCLUDED.recorded_by,recorded_at=now()`, enc, n, site, depth, margin, reading.Bleeding, mobility, furcation, actorID(r))
		if err != nil {
			writeError(w, 500, "No se pudo guardar una medición")
			return
		}
	}
	if tx.Commit() != nil {
		writeError(w, 500, "No se pudo confirmar el periodontograma")
		return
	}
	writeJSON(w, 200, map[string]any{"saved": true})
}

type prescriptionInput struct {
	Med      string `json:"med"`
	Dose     string `json:"dose"`
	Duration string `json:"duration"`
}

func listPrescriptions(w http.ResponseWriter, r *http.Request) {
	id, ok := clinicalPatientID(w, r)
	if !ok {
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT p.id,p.medication,p.instructions,p.duration,p.signed_at,u.full_name FROM prescriptions p JOIN encounters e ON e.id=p.encounter_id JOIN users u ON u.id=p.provider_id WHERE e.patient_id=$1 ORDER BY p.signed_at DESC`, id)
	if err != nil {
		writeError(w, 500, "No se pudieron cargar las recetas")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var i int64
		var med, dose, dur, name string
		var at time.Time
		if rows.Scan(&i, &med, &dose, &dur, &at, &name) != nil {
			continue
		}
		out = append(out, map[string]any{"id": i, "patient": id, "med": med, "dose": dose, "duration": dur, "date": at.Format("02/01/2006"), "author": name})
	}
	writeJSON(w, 200, out)
}
func createPrescription(w http.ResponseWriter, r *http.Request) {
	patientID, ok := clinicalPatientID(w, r)
	if !ok {
		return
	}
	var in prescriptionInput
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Med) == "" || strings.TrimSpace(in.Dose) == "" || strings.TrimSpace(in.Duration) == "" {
		writeError(w, 400, "Completa medicamento, indicaciones y duración")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar el registro")
		return
	}
	defer tx.Rollback()
	enc, err := encounterFor(r.Context(), tx, patientID, actorID(r))
	if err != nil {
		writeError(w, 500, "No se pudo crear la atención")
		return
	}
	var id int64
	var at time.Time
	err = tx.QueryRowContext(r.Context(), `INSERT INTO prescriptions(encounter_id,provider_id,medication,instructions,duration,signature_ref) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,signed_at`, enc, actorID(r), in.Med, in.Dose, in.Duration, fmt.Sprintf("session:%d", actorID(r))).Scan(&id, &at)
	if err != nil {
		writeError(w, 500, "No se pudo guardar la receta")
		return
	}
	if tx.Commit() != nil {
		writeError(w, 500, "No se pudo confirmar la receta")
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "patient": patientID, "med": in.Med, "dose": in.Dose, "duration": in.Duration, "date": at.Format("02/01/2006")})
}

type consentInput struct {
	Procedure string `json:"procedure"`
}

func listConsents(w http.ResponseWriter, r *http.Request) {
	id, ok := clinicalPatientID(w, r)
	if !ok {
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT id,procedure_name,status,created_at FROM informed_consents WHERE patient_id=$1 ORDER BY created_at DESC`, id)
	if err != nil {
		writeError(w, 500, "No se pudieron cargar los consentimientos")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var i int64
		var proc, status string
		var at time.Time
		if rows.Scan(&i, &proc, &status, &at) != nil {
			continue
		}
		label := "Pendiente de firma"
		if status == "signed" {
			label = "Firmado"
		}
		if status == "declined" {
			label = "Rechazado"
		}
		out = append(out, map[string]any{"id": i, "patient": id, "procedure": proc, "state": label, "date": at.Format("02/01/2006")})
	}
	writeJSON(w, 200, out)
}
func createConsent(w http.ResponseWriter, r *http.Request) {
	patientID, ok := clinicalPatientID(w, r)
	if !ok {
		return
	}
	var in consentInput
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Procedure) == "" {
		writeError(w, 400, "Indica el procedimiento del consentimiento")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar el registro")
		return
	}
	defer tx.Rollback()
	enc, err := encounterFor(r.Context(), tx, patientID, actorID(r))
	if err != nil {
		writeError(w, 500, "No se pudo crear la atención")
		return
	}
	var id int64
	var at time.Time
	err = tx.QueryRowContext(r.Context(), `INSERT INTO informed_consents(patient_id,encounter_id,procedure_name,document_ref,created_by) VALUES($1,$2,$3,$4,$5) RETURNING id,created_at`, patientID, enc, in.Procedure, fmt.Sprintf("draft:%d", time.Now().UnixNano()), actorID(r)).Scan(&id, &at)
	if err != nil {
		writeError(w, 500, "No se pudo preparar el consentimiento")
		return
	}
	if tx.Commit() != nil {
		writeError(w, 500, "No se pudo confirmar el consentimiento")
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "patient": patientID, "procedure": in.Procedure, "state": "Pendiente de firma", "date": at.Format("02/01/2006")})
}

func listPatients(w http.ResponseWriter, r *http.Request) {
	rows, err := db.QueryContext(r.Context(), `
		SELECT p.id, p.given_names || ' ' || p.family_names, p.national_id,
		       COALESCE(EXTRACT(YEAR FROM age(CURRENT_DATE, p.birth_date))::int, p.age_years::int, 0), COALESCE(to_char(p.birth_date,'YYYY-MM-DD'),''),
		       COALESCE(p.phone,''),COALESCE(p.emergency_contact,''), COALESCE(p.email,''), COALESCE(p.blood_type,''),
		       COALESCE((SELECT full_name FROM patient_guardians g WHERE g.patient_id=p.id AND g.legal_guardian ORDER BY g.id DESC LIMIT 1),''),
		       COALESCE((SELECT string_agg(description, ', ') FROM medical_alerts WHERE patient_id=p.id AND active AND category='allergy'), 'Ninguna conocida'),
		       COALESCE((SELECT string_agg(description, ', ') FROM medical_alerts WHERE patient_id=p.id AND active AND category IN ('condition','medication','surgery','coagulation','other')), 'Ninguna'),
		       COALESCE(p.insurance_name,'Particular'),
		       COALESCE((SELECT to_char(max(started_at) AT TIME ZONE 'America/Guayaquil', 'DD Mon YYYY') FROM encounters WHERE patient_id=p.id), 'Sin visitas')
		FROM patients p WHERE p.clinic_id=$1 AND p.active ORDER BY p.family_names,p.given_names`, clinicID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	result := make([]Patient, 0)
	for rows.Next() {
		var p Patient
		if err := rows.Scan(&p.ID, &p.Name, &p.Doc, &p.Age, &p.Birth, &p.Phone, &p.Emergency, &p.Email, &p.Blood, &p.Guardian, &p.Allergies, &p.Conditions, &p.Insurance, &p.Last); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result = append(result, p)
	}
	writeJSON(w, http.StatusOK, result)
}

func createPatient(w http.ResponseWriter, r *http.Request) {
	var in PatientInput
	if !decode(w, r, &in) {
		return
	}
	in.Name, in.Doc = strings.TrimSpace(in.Name), strings.TrimSpace(in.Doc)
	if in.Name == "" || in.Doc == "" || in.Age < 0 || in.Age > 125 {
		writeError(w, http.StatusBadRequest, "Nombre, identificación y edad válida son obligatorios")
		return
	}
	parts := strings.Fields(in.Name)
	given, family := strings.Join(parts, " "), "Sin apellido"
	if len(parts) > 1 {
		given, family = strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
	}
	var birth any
	if in.Birth != "" {
		parsed, err := time.Parse("2006-01-02", in.Birth)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Fecha de nacimiento inválida")
			return
		}
		birth = parsed
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO patients(clinic_id,national_id,given_names,family_names,birth_date,age_years,phone,email,emergency_contact,blood_type,insurance_name,created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,''),$11,$12) RETURNING id`, clinicID, in.Doc, given, family, birth, in.Age, in.Phone, in.Email, in.Emergency, in.Blood, in.Insurance, actorID(r)).Scan(&id)
	if err != nil {
		writeError(w, http.StatusConflict, "No se pudo registrar: verifica que la identificación no esté duplicada")
		return
	}
	for _, alert := range []struct{ category, value string }{{"allergy", in.Allergies}, {"condition", in.Conditions}} {
		v := strings.TrimSpace(alert.value)
		if v == "" || strings.EqualFold(v, "Ninguna") || strings.EqualFold(v, "Ninguna conocida") {
			continue
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO medical_alerts(patient_id,category,description,critical,recorded_by) VALUES ($1,$2,$3,$4,$5)`, id, alert.category, v, alert.category == "allergy", actorID(r)); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if guardian := strings.TrimSpace(in.Guardian); guardian != "" {
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO patient_guardians(patient_id,full_name,relationship,legal_guardian) VALUES ($1,$2,'Tutor legal',TRUE)`, id, guardian); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, Patient{ID: id, Name: in.Name, Doc: in.Doc, Age: in.Age, Birth: in.Birth, Phone: in.Phone, Emergency: in.Emergency, Email: in.Email, Blood: in.Blood, Guardian: in.Guardian, Allergies: fallback(in.Allergies, "Ninguna conocida"), Conditions: fallback(in.Conditions, "Ninguna"), Insurance: fallback(in.Insurance, "Particular"), Last: "Sin visitas"})
}

func updatePatient(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("patientID"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "Identificador de paciente inválido")
		return
	}
	var in PatientInput
	if !decode(w, r, &in) {
		return
	}
	in.Name, in.Doc = strings.TrimSpace(in.Name), strings.TrimSpace(in.Doc)
	if in.Name == "" || in.Doc == "" || in.Age < 0 || in.Age > 125 {
		writeError(w, http.StatusBadRequest, "Nombre, identificación y edad válida son obligatorios")
		return
	}
	parts := strings.Fields(in.Name)
	given, family := strings.Join(parts, " "), "Sin apellido"
	if len(parts) > 1 {
		given, family = strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
	}
	var birth any
	if in.Birth != "" {
		parsed, parseErr := time.Parse("2006-01-02", in.Birth)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "Fecha de nacimiento inválida")
			return
		}
		birth = parsed
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudo iniciar la actualización")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `UPDATE patients SET national_id=$1,given_names=$2,family_names=$3,birth_date=$4,age_years=$5,phone=$6,email=$7,emergency_contact=$8,blood_type=NULLIF($9,''),insurance_name=$10,updated_at=now() WHERE id=$11 AND clinic_id=$12 AND active`, in.Doc, given, family, birth, in.Age, in.Phone, in.Email, in.Emergency, in.Blood, in.Insurance, id, clinicID)
	if err != nil {
		writeError(w, http.StatusConflict, "No se pudo actualizar: verifica que la identificación no esté duplicada")
		return
	}
	changed, err := result.RowsAffected()
	if err != nil || changed == 0 {
		writeError(w, http.StatusNotFound, "Paciente no encontrado")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE medical_alerts SET active=FALSE WHERE patient_id=$1 AND active`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudieron actualizar las alertas")
		return
	}
	for _, alert := range []struct{ category, value string }{{"allergy", in.Allergies}, {"condition", in.Conditions}} {
		value := strings.TrimSpace(alert.value)
		if value == "" || strings.EqualFold(value, "Ninguna") || strings.EqualFold(value, "Ninguna conocida") {
			continue
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO medical_alerts(patient_id,category,description,critical,recorded_by) VALUES($1,$2,$3,$4,$5)`, id, alert.category, value, alert.category == "allergy", actorID(r)); err != nil {
			writeError(w, http.StatusInternalServerError, "No se pudieron guardar las alertas")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM patient_guardians WHERE patient_id=$1 AND legal_guardian`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudo actualizar el tutor legal")
		return
	}
	if guardian := strings.TrimSpace(in.Guardian); guardian != "" {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO patient_guardians(patient_id,full_name,relationship,legal_guardian) VALUES($1,$2,'Tutor legal',TRUE)`, id, guardian); err != nil {
			writeError(w, http.StatusInternalServerError, "No se pudo guardar el tutor legal")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudo confirmar la actualización")
		return
	}
	writeJSON(w, http.StatusOK, Patient{ID: id, Name: in.Name, Doc: in.Doc, Age: in.Age, Birth: in.Birth, Phone: in.Phone, Emergency: in.Emergency, Email: in.Email, Blood: in.Blood, Guardian: in.Guardian, Allergies: fallback(in.Allergies, "Ninguna conocida"), Conditions: fallback(in.Conditions, "Ninguna"), Insurance: fallback(in.Insurance, "Particular")})
}

type treatmentPlanItemInput struct {
	Procedure     string  `json:"procedure"`
	Phase         int     `json:"phase"`
	Sessions      int     `json:"sessions"`
	Fee           float64 `json:"fee"`
	LabCost       float64 `json:"labCost"`
	MaterialsCost float64 `json:"materialsCost"`
}

type treatmentPlanInput struct {
	PatientID int64                    `json:"patientId"`
	Title     string                   `json:"title"`
	Discount  float64                  `json:"discount"`
	Items     []treatmentPlanItemInput `json:"items"`
}

func listTreatmentPlans(w http.ResponseWriter, r *http.Request) {
	query := `SELECT t.id,t.patient_id,p.given_names || ' ' || p.family_names,t.title,t.status,t.subtotal,t.discount,t.total,t.created_at FROM treatment_plans t JOIN patients p ON p.id=t.patient_id WHERE p.clinic_id=$1`
	args := []any{clinicID}
	if raw := r.URL.Query().Get("patientId"); raw != "" {
		patientID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || patientID < 1 {
			writeError(w, http.StatusBadRequest, "Paciente inválido")
			return
		}
		query += ` AND t.patient_id=$2`
		args = append(args, patientID)
	}
	query += ` ORDER BY t.created_at DESC`
	rows, err := db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "No se pudieron cargar los planes")
		return
	}
	type planRow struct {
		ID, PatientID             int64
		Patient, Title, Status    string
		Subtotal, Discount, Total float64
		Created                   time.Time
	}
	plans := []planRow{}
	for rows.Next() {
		var plan planRow
		if err = rows.Scan(&plan.ID, &plan.PatientID, &plan.Patient, &plan.Title, &plan.Status, &plan.Subtotal, &plan.Discount, &plan.Total, &plan.Created); err != nil {
			rows.Close()
			writeError(w, 500, "No se pudo leer un plan")
			return
		}
		plans = append(plans, plan)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		writeError(w, 500, "No se pudieron leer los planes")
		return
	}
	rows.Close()
	out := make([]map[string]any, 0, len(plans))
	for _, plan := range plans {
		itemRows, queryErr := db.QueryContext(r.Context(), `SELECT i.id,i.procedure_name,i.phase_number,i.sessions,i.fee,i.lab_cost,i.materials_cost,i.approved_at,i.completed_at,(SELECT count(*) FROM appointments a WHERE a.plan_item_id=i.id AND a.status NOT IN ('cancelled','no_show')) FROM treatment_plan_items i WHERE i.plan_id=$1 ORDER BY i.phase_number,i.id`, plan.ID)
		if queryErr != nil {
			writeError(w, 500, "No se pudieron cargar las partidas")
			return
		}
		items := []map[string]any{}
		for itemRows.Next() {
			var id int64
			var procedure string
			var phase, sessions, scheduled int
			var fee, lab, materials float64
			var approved, completed sql.NullTime
			if itemRows.Scan(&id, &procedure, &phase, &sessions, &fee, &lab, &materials, &approved, &completed, &scheduled) != nil {
				itemRows.Close()
				writeError(w, 500, "No se pudo leer una partida")
				return
			}
			items = append(items, map[string]any{"id": id, "procedure": procedure, "phase": phase, "sessions": sessions, "scheduledSessions": scheduled, "availableSessions": max(0, sessions-scheduled), "fee": fee, "labCost": lab, "materialsCost": materials, "approved": approved.Valid, "completed": completed.Valid})
		}
		if err = itemRows.Err(); err != nil {
			itemRows.Close()
			writeError(w, 500, "No se pudieron leer las partidas")
			return
		}
		itemRows.Close()
		out = append(out, map[string]any{"id": plan.ID, "patientId": plan.PatientID, "patient": plan.Patient, "title": plan.Title, "status": plan.Status, "subtotal": plan.Subtotal, "discount": plan.Discount, "total": plan.Total, "createdAt": plan.Created, "items": items})
	}
	writeJSON(w, http.StatusOK, out)
}

func createTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	var in treatmentPlanInput
	if !decode(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.PatientID < 1 || in.Title == "" || len(in.Items) == 0 || len(in.Items) > 30 || in.Discount < 0 {
		writeError(w, 400, "Paciente, título y al menos una partida válida son obligatorios")
		return
	}
	subtotal := 0.0
	for i := range in.Items {
		item := &in.Items[i]
		item.Procedure = strings.TrimSpace(item.Procedure)
		if item.Procedure == "" || item.Phase < 1 || item.Sessions < 1 || item.Sessions > 100 || item.Fee < 0 || item.LabCost < 0 || item.MaterialsCost < 0 {
			writeError(w, 400, "Revisa el procedimiento, fase, sesiones y costos de cada partida")
			return
		}
		subtotal += item.Fee + item.LabCost + item.MaterialsCost
	}
	if in.Discount > subtotal {
		writeError(w, 400, "El descuento no puede superar el subtotal")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar el plan")
		return
	}
	defer tx.Rollback()
	var patientExists bool
	if err = tx.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM patients WHERE id=$1 AND clinic_id=$2 AND active)`, in.PatientID, clinicID).Scan(&patientExists); err != nil || !patientExists {
		writeError(w, 404, "Paciente no encontrado")
		return
	}
	var planID int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO treatment_plans(patient_id,provider_id,title,status,subtotal,discount,total) VALUES($1,$2,$3,'presented',$4,$5,$6) RETURNING id`, in.PatientID, actorID(r), in.Title, subtotal, in.Discount, subtotal-in.Discount).Scan(&planID)
	if err != nil {
		writeError(w, 500, "No se pudo guardar el plan")
		return
	}
	for _, item := range in.Items {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO treatment_plan_items(plan_id,procedure_name,phase_number,sessions,fee,lab_cost,materials_cost) VALUES($1,$2,$3,$4,$5,$6,$7)`, planID, item.Procedure, item.Phase, item.Sessions, item.Fee, item.LabCost, item.MaterialsCost); err != nil {
			writeError(w, 500, "No se pudo guardar una partida")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeError(w, 500, "No se pudo confirmar el plan")
		return
	}
	writeJSON(w, 201, map[string]any{"id": planID, "patientId": in.PatientID, "title": in.Title, "status": "presented", "subtotal": subtotal, "discount": in.Discount, "total": subtotal - in.Discount})
}

func approveTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	planID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || planID < 1 {
		writeError(w, 400, "Plan inválido")
		return
	}
	var in struct {
		ItemIDs []int64 `json:"itemIds"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.ItemIDs) == 0 {
		writeError(w, 400, "Selecciona al menos una partida aprobada")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar la aprobación")
		return
	}
	defer tx.Rollback()
	var clinic int64
	if err = tx.QueryRowContext(r.Context(), `SELECT p.clinic_id FROM treatment_plans t JOIN patients p ON p.id=t.patient_id WHERE t.id=$1 FOR UPDATE`, planID).Scan(&clinic); err != nil || clinic != clinicID {
		writeError(w, 404, "Plan no encontrado")
		return
	}
	seen := map[int64]bool{}
	for _, itemID := range in.ItemIDs {
		if itemID < 1 || seen[itemID] {
			writeError(w, 400, "Partidas inválidas o repetidas")
			return
		}
		seen[itemID] = true
		res, e := tx.ExecContext(r.Context(), `UPDATE treatment_plan_items SET approved_at=COALESCE(approved_at,now()) WHERE id=$1 AND plan_id=$2 AND completed_at IS NULL`, itemID, planID)
		if e != nil {
			writeError(w, 500, "No se pudo aprobar una partida")
			return
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			writeError(w, 404, "Una partida ya no está disponible")
			return
		}
	}
	var total, approved int
	if err = tx.QueryRowContext(r.Context(), `SELECT count(*),count(*) FILTER(WHERE approved_at IS NOT NULL) FROM treatment_plan_items WHERE plan_id=$1`, planID).Scan(&total, &approved); err != nil {
		writeError(w, 500, "No se pudo verificar la aprobación")
		return
	}
	status := "partially_approved"
	if approved == total {
		status = "approved"
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE treatment_plans SET status=$1 WHERE id=$2`, status, planID); err != nil {
		writeError(w, 500, "No se pudo actualizar el plan")
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, 500, "No se pudo confirmar la aprobación")
		return
	}
	writeJSON(w, 200, map[string]any{"planId": planID, "status": status, "approvedItems": approved, "totalItems": total, "readyToSchedule": true})
}

func closeCash(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DeclaredCash float64 `json:"declaredCash"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.DeclaredCash < 0 {
		writeError(w, 400, "El efectivo contado no puede ser negativo")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar el cierre")
		return
	}
	defer tx.Rollback()
	var lockResult any
	if err = tx.QueryRowContext(r.Context(), `SELECT pg_advisory_xact_lock($1)`, clinicID).Scan(&lockResult); err != nil {
		writeError(w, 500, "No se pudo bloquear la caja para el cierre")
		return
	}
	var periodStart time.Time
	err = tx.QueryRowContext(r.Context(), `SELECT closed_at FROM cash_closures WHERE clinic_id=$1 ORDER BY closed_at DESC LIMIT 1`, clinicID).Scan(&periodStart)
	if errors.Is(err, sql.ErrNoRows) {
		loc, _ := time.LoadLocation("America/Guayaquil")
		now := time.Now().In(loc)
		periodStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	} else if err != nil {
		writeError(w, 500, "No se pudo consultar el cierre anterior")
		return
	}
	var expected, total float64
	var count int
	if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(sum(p.amount) FILTER(WHERE p.method='cash'),0),COALESCE(sum(p.amount),0),count(*) FROM payments p JOIN invoices i ON i.id=p.invoice_id WHERE i.clinic_id=$1 AND p.received_at >= $2`, clinicID, periodStart).Scan(&expected, &total, &count); err != nil {
		writeError(w, 500, "No se pudieron calcular los cobros del turno")
		return
	}
	difference := in.DeclaredCash - expected
	var closureID int64
	var closedAt time.Time
	err = tx.QueryRowContext(r.Context(), `INSERT INTO cash_closures(clinic_id,closed_by,period_start,declared_cash,expected_cash,total_collected,payment_count,difference) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,closed_at`, clinicID, actorID(r), periodStart, in.DeclaredCash, expected, total, count, difference).Scan(&closureID, &closedAt)
	if err != nil {
		writeError(w, 500, "No se pudo registrar el cierre")
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, 500, "No se pudo confirmar el cierre")
		return
	}
	writeJSON(w, 201, map[string]any{"id": closureID, "periodStart": periodStart, "closedAt": closedAt, "declaredCash": in.DeclaredCash, "expectedCash": expected, "difference": difference, "totalCollected": total, "paymentCount": count})
}

func listAppointments(w http.ResponseWriter, r *http.Request) {
	rows, err := db.QueryContext(r.Context(), `SELECT a.id, p.given_names || ' ' || p.family_names, u.full_name, a.reason, to_char(a.starts_at AT TIME ZONE 'America/Guayaquil','YYYY-MM-DD'), to_char(a.starts_at AT TIME ZONE 'America/Guayaquil','HH24:MI'), c.name, a.status FROM appointments a JOIN patients p ON p.id=a.patient_id JOIN users u ON u.id=a.provider_id JOIN chairs c ON c.id=a.chair_id WHERE a.clinic_id=$1 ORDER BY a.starts_at`, clinicID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	result := make([]Appointment, 0)
	for rows.Next() {
		var a Appointment
		if err := rows.Scan(&a.ID, &a.Patient, &a.Doctor, &a.Procedure, &a.Date, &a.Time, &a.Chair, &a.Status); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.Status = statusLabel(a.Status)
		result = append(result, a)
	}
	writeJSON(w, http.StatusOK, result)
}

func createAppointment(w http.ResponseWriter, r *http.Request) {
	var in AppointmentInput
	if !decode(w, r, &in) {
		return
	}
	chairID, ok := chairIDs[in.Chair]
	if !ok {
		writeError(w, http.StatusBadRequest, "Sillón inválido")
		return
	}
	loc, _ := time.LoadLocation("America/Guayaquil")
	start, err := time.ParseInLocation("2006-01-02 15:04", in.Date+" "+in.Time, loc)
	if err != nil || strings.TrimSpace(in.Procedure) == "" {
		writeError(w, http.StatusBadRequest, "Fecha, hora y procedimiento son obligatorios")
		return
	}
	var patientID int64
	err = db.QueryRowContext(r.Context(), `SELECT id FROM patients WHERE clinic_id=$1 AND (given_names || ' ' || family_names)=$2 AND active`, clinicID, in.Patient).Scan(&patientID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "El paciente seleccionado no existe en la base de datos")
		return
	}
	var providerID int64
	err = db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE full_name=$1 AND active`, in.Doctor).Scan(&providerID)
	if errors.Is(err, sql.ErrNoRows) {
		providerID = actorID(r)
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	state := statusCode(in.Status)
	var id int64
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar el agendamiento")
		return
	}
	defer tx.Rollback()
	if in.PlanItemID > 0 {
		var approvedProcedure string
		var sessions int
		err = tx.QueryRowContext(r.Context(), `SELECT i.procedure_name,i.sessions FROM treatment_plan_items i JOIN treatment_plans t ON t.id=i.plan_id JOIN patients p ON p.id=t.patient_id WHERE i.id=$1 AND t.patient_id=$2 AND p.clinic_id=$3 AND i.approved_at IS NOT NULL AND i.completed_at IS NULL FOR UPDATE OF i`, in.PlanItemID, patientID, clinicID).Scan(&approvedProcedure, &sessions)
		if err != nil || !strings.EqualFold(strings.TrimSpace(approvedProcedure), strings.TrimSpace(in.Procedure)) {
			writeError(w, http.StatusBadRequest, "La cita debe corresponder a una partida aprobada del plan del paciente")
			return
		}
		var scheduled int
		if err = tx.QueryRowContext(r.Context(), `SELECT count(*) FROM appointments WHERE plan_item_id=$1 AND status NOT IN ('cancelled','no_show')`, in.PlanItemID).Scan(&scheduled); err != nil {
			writeError(w, 500, "No se pudo verificar las sesiones disponibles")
			return
		}
		if scheduled >= sessions {
			writeError(w, 409, "Ya se agendaron todas las sesiones aprobadas para este procedimiento")
			return
		}
	}
	err = tx.QueryRowContext(r.Context(), `INSERT INTO appointments(clinic_id,patient_id,chair_id,provider_id,starts_at,ends_at,reason,status,created_by,plan_item_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,0)) RETURNING id`, clinicID, patientID, chairID, providerID, start, start.Add(30*time.Minute), in.Procedure, state, actorID(r), in.PlanItemID).Scan(&id)
	if err != nil {
		writeError(w, http.StatusConflict, "El sillón ya tiene una cita en ese horario")
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, http.StatusConflict, "No se pudo confirmar la cita; revisa si el sillón ya está ocupado")
		return
	}
	writeJSON(w, http.StatusCreated, Appointment{ID: id, Patient: in.Patient, Doctor: fallback(in.Doctor, "Dra. Valeria Ruiz"), Procedure: in.Procedure, Date: in.Date, Time: in.Time, Chair: in.Chair, Status: statusLabel(state)})
}

func updateAppointmentStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Identificador de cita inválido")
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &body) {
		return
	}
	state := statusCode(body.Status)
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar el cambio de estado")
		return
	}
	defer tx.Rollback()
	var previous, procedure string
	var planItemID sql.NullInt64
	if err = tx.QueryRowContext(r.Context(), `SELECT status,reason,plan_item_id FROM appointments WHERE id=$1 AND clinic_id=$2 FOR UPDATE`, id, clinicID).Scan(&previous, &procedure, &planItemID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, 404, "Cita no encontrada")
		} else {
			writeError(w, 500, "No se pudo consultar la cita")
		}
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE appointments SET status=$1 WHERE id=$2 AND clinic_id=$3`, state, id, clinicID); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	warnings := []string{}
	if state == "attended" && previous != "attended" {
		warnings, err = consumeAppointmentInventory(r.Context(), tx, id, procedure, actorID(r))
		if err != nil {
			writeError(w, 500, "No se pudo descargar el inventario: "+err.Error())
			return
		}
		if planItemID.Valid {
			if _, err = tx.ExecContext(r.Context(), `UPDATE treatment_plan_items i SET completed_at=COALESCE(i.completed_at,now()) WHERE i.id=$1 AND i.sessions <= (SELECT count(*) FROM appointments a WHERE a.plan_item_id=i.id AND a.status='attended')`, planItemID.Int64); err != nil {
				writeError(w, 500, "No se pudo actualizar el avance del plan")
				return
			}
		}
	}
	if err = tx.Commit(); err != nil {
		writeError(w, 500, "No se pudo confirmar el estado de la cita")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": statusLabel(state), "inventoryWarnings": warnings})
}

func consumeAppointmentInventory(ctx context.Context, tx *sql.Tx, appointmentID int64, procedure string, actor int64) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT r.id,r.product_id,p.name,r.quantity FROM inventory_consumption_rules r JOIN inventory_products p ON p.id=r.product_id WHERE r.clinic_id=$1 AND r.active AND lower(trim(r.procedure_match))=lower(trim($2)) AND p.active ORDER BY r.id`, clinicID, procedure)
	if err != nil {
		return nil, err
	}
	type rule struct {
		id, product int64
		name        string
		qty         float64
	}
	rules := []rule{}
	for rows.Next() {
		var v rule
		if err := rows.Scan(&v.id, &v.product, &v.name, &v.qty); err != nil {
			rows.Close()
			return nil, err
		}
		rules = append(rules, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	warnings := []string{}
	for _, v := range rules {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM appointment_inventory_consumptions WHERE appointment_id=$1 AND rule_id=$2)`, appointmentID, v.id).Scan(&exists); err != nil {
			return nil, err
		}
		if exists {
			continue
		}
		var lockedID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM inventory_products WHERE id=$1 AND clinic_id=$2 AND active FOR UPDATE`, v.product, clinicID).Scan(&lockedID); err != nil {
			return nil, err
		}
		var stock float64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(CASE WHEN movement_type='consumption' THEN -quantity ELSE quantity END),0) FROM inventory_movements WHERE product_id=$1`, v.product).Scan(&stock); err != nil {
			return nil, err
		}
		var movementID int64
		if err := tx.QueryRowContext(ctx, `INSERT INTO inventory_movements(product_id,movement_type,quantity,reference,notes,created_by) VALUES($1,'consumption',$2,$3,$4,$5) RETURNING id`, v.product, v.qty, fmt.Sprintf("Cita #%d", appointmentID), "Descargo automático por procedimiento: "+procedure, actor).Scan(&movementID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO appointment_inventory_consumptions(appointment_id,rule_id,movement_id) VALUES($1,$2,$3)`, appointmentID, v.id, movementID); err != nil {
			return nil, err
		}
		if stock < v.qty {
			warnings = append(warnings, fmt.Sprintf("Stock insuficiente de %s: el descargo automático dejó existencias negativas", v.name))
		}
	}
	return warnings, nil
}

func listInvoices(w http.ResponseWriter, r *http.Request) {
	rows, err := db.QueryContext(r.Context(), `SELECT i.number, p.given_names || ' ' || p.family_names, i.concept, i.total, COALESCE(sum(pay.amount),0), to_char(i.issued_at AT TIME ZONE 'America/Guayaquil','YYYY-MM-DD'), i.status FROM invoices i JOIN patients p ON p.id=i.patient_id LEFT JOIN payments pay ON pay.invoice_id=i.id WHERE i.clinic_id=$1 GROUP BY i.id,p.id ORDER BY i.issued_at DESC`, clinicID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	result := make([]Invoice, 0)
	for rows.Next() {
		var i Invoice
		if err := rows.Scan(&i.ID, &i.Patient, &i.Concept, &i.Total, &i.Paid, &i.Date, &i.State); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		i.State = invoiceLabel(i.State)
		result = append(result, i)
	}
	writeJSON(w, http.StatusOK, result)
}

func createInvoice(w http.ResponseWriter, r *http.Request) {
	var in InvoiceInput
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Concept) == "" || in.Total <= 0 || in.Paid < 0 || in.Paid > in.Total {
		writeError(w, http.StatusBadRequest, "Concepto y total válidos requeridos; el abono no puede superar el total")
		return
	}
	var patientID int64
	if err := db.QueryRowContext(r.Context(), `SELECT id FROM patients WHERE clinic_id=$1 AND (given_names || ' ' || family_names)=$2 AND active`, clinicID, in.Patient).Scan(&patientID); err != nil {
		writeError(w, http.StatusBadRequest, "El paciente seleccionado no existe en la base de datos")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()
	number := fmt.Sprintf("FAC-%d", time.Now().UnixNano())
	state := "pending"
	if in.Paid == in.Total {
		state = "paid"
	} else if in.Paid > 0 {
		state = "partially_paid"
	}
	var id int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO invoices(clinic_id,patient_id,number,concept,total,status,issued_by) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`, clinicID, patientID, number, in.Concept, in.Total, state, actorID(r)).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if in.Paid > 0 {
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO payments(invoice_id,amount,method,received_by) VALUES ($1,$2,$3,$4)`, id, in.Paid, paymentCode(in.Method), actorID(r)); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, Invoice{ID: number, Patient: in.Patient, Concept: in.Concept, Total: in.Total, Paid: in.Paid, Date: time.Now().In(time.FixedZone("ECT", -5*60*60)).Format("2006-01-02"), State: invoiceLabel(state)})
}

type inventoryProductInput struct {
	SKU          string  `json:"sku"`
	Name         string  `json:"name"`
	Category     string  `json:"category"`
	Unit         string  `json:"unit"`
	MinimumStock float64 `json:"minimumStock"`
	InitialStock float64 `json:"initialStock"`
}
type inventoryMovementInput struct {
	ProductID int64   `json:"productId"`
	Type      string  `json:"type"`
	Quantity  float64 `json:"quantity"`
	Reference string  `json:"reference"`
	Notes     string  `json:"notes"`
}

func listInventoryProducts(w http.ResponseWriter, r *http.Request) {
	rows, err := db.QueryContext(r.Context(), `SELECT p.id,p.sku,p.name,p.category,p.unit,p.minimum_stock,COALESCE(sum(CASE WHEN m.movement_type='consumption' THEN -m.quantity ELSE m.quantity END),0),p.active FROM inventory_products p LEFT JOIN inventory_movements m ON m.product_id=p.id WHERE p.clinic_id=$1 GROUP BY p.id ORDER BY p.name`, clinicID)
	if err != nil {
		writeError(w, 500, "No se pudo cargar el inventario")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var sku, name, category, unit string
		var minimum, stock float64
		var active bool
		if rows.Scan(&id, &sku, &name, &category, &unit, &minimum, &stock, &active) != nil {
			writeError(w, 500, "No se pudo leer el inventario")
			return
		}
		out = append(out, map[string]any{"id": id, "sku": sku, "name": name, "category": category, "unit": unit, "minimumStock": minimum, "stock": stock, "lowStock": stock <= minimum, "active": active})
	}
	writeJSON(w, 200, out)
}
func createInventoryProduct(w http.ResponseWriter, r *http.Request) {
	var in inventoryProductInput
	if !decode(w, r, &in) {
		return
	}
	in.SKU = strings.TrimSpace(in.SKU)
	in.Name = strings.TrimSpace(in.Name)
	in.Category = fallback(strings.TrimSpace(in.Category), "Insumos")
	in.Unit = fallback(strings.TrimSpace(in.Unit), "unidad")
	if in.SKU == "" || in.Name == "" || in.MinimumStock < 0 || in.InitialStock < 0 {
		writeError(w, 400, "Código, nombre y cantidades válidas son obligatorios")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar el registro")
		return
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(r.Context(), `INSERT INTO inventory_products(clinic_id,sku,name,category,unit,minimum_stock,created_by) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, clinicID, in.SKU, in.Name, in.Category, in.Unit, in.MinimumStock, actorID(r)).Scan(&id)
	if err != nil {
		writeError(w, 409, "No se pudo crear el insumo; verifica que el código no esté repetido")
		return
	}
	if in.InitialStock > 0 {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO inventory_movements(product_id,movement_type,quantity,reference,notes,created_by) VALUES($1,'adjustment',$2,'Stock inicial','Registro inicial del producto',$3)`, id, in.InitialStock, actorID(r))
		if err != nil {
			writeError(w, 500, "No se pudo guardar el stock inicial")
			return
		}
	}
	if tx.Commit() != nil {
		writeError(w, 500, "No se pudo confirmar el insumo")
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "sku": in.SKU, "name": in.Name, "category": in.Category, "unit": in.Unit, "minimumStock": in.MinimumStock, "stock": in.InitialStock, "lowStock": in.InitialStock <= in.MinimumStock, "active": true})
}
func listInventoryMovements(w http.ResponseWriter, r *http.Request) {
	query := `SELECT m.id,m.product_id,p.name,m.movement_type,m.quantity,m.reference,m.notes,m.created_at,u.full_name FROM inventory_movements m JOIN inventory_products p ON p.id=m.product_id JOIN users u ON u.id=m.created_by WHERE p.clinic_id=$1`
	args := []any{clinicID}
	if raw := r.URL.Query().Get("productId"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, 400, "Producto inválido")
			return
		}
		query += ` AND p.id=$2`
		args = append(args, id)
	}
	query += ` ORDER BY m.created_at DESC LIMIT 100`
	rows, err := db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, 500, "No se pudo cargar el historial")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, pid int64
		var name, kind, reference, notes, author string
		var qty float64
		var at time.Time
		if rows.Scan(&id, &pid, &name, &kind, &qty, &reference, &notes, &at, &author) != nil {
			writeError(w, 500, "No se pudo leer el historial")
			return
		}
		out = append(out, map[string]any{"id": id, "productId": pid, "product": name, "type": kind, "quantity": qty, "reference": reference, "notes": notes, "date": at.Format("02/01/2006 15:04"), "author": author})
	}
	writeJSON(w, 200, out)
}
func createInventoryMovement(w http.ResponseWriter, r *http.Request) {
	var in inventoryMovementInput
	if !decode(w, r, &in) {
		return
	}
	if in.ProductID < 1 || in.Quantity <= 0 || (in.Type != "receipt" && in.Type != "consumption") {
		writeError(w, 400, "Tipo de movimiento, producto y cantidad válidos son obligatorios")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "No se pudo iniciar el movimiento")
		return
	}
	defer tx.Rollback()
	var id int64
	var stock float64
	err = tx.QueryRowContext(r.Context(), `SELECT id FROM inventory_products WHERE id=$1 AND clinic_id=$2 AND active FOR UPDATE`, in.ProductID, clinicID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "Insumo no encontrado")
		return
	}
	if err != nil {
		writeError(w, 500, "No se pudo verificar el stock")
		return
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(sum(CASE WHEN movement_type='consumption' THEN -quantity ELSE quantity END),0) FROM inventory_movements WHERE product_id=$1`, id).Scan(&stock); err != nil {
		writeError(w, 500, "No se pudo calcular el stock")
		return
	}
	if in.Type == "consumption" && in.Quantity > stock {
		writeError(w, 409, "No hay existencias suficientes para registrar ese consumo")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO inventory_movements(product_id,movement_type,quantity,reference,notes,created_by) VALUES($1,$2,$3,$4,$5,$6)`, in.ProductID, in.Type, in.Quantity, strings.TrimSpace(in.Reference), strings.TrimSpace(in.Notes), actorID(r))
	if err != nil {
		writeError(w, 500, "No se pudo registrar el movimiento")
		return
	}
	if tx.Commit() != nil {
		writeError(w, 500, "No se pudo confirmar el movimiento")
		return
	}
	writeJSON(w, 201, map[string]any{"saved": true})
}

type inventoryRuleInput struct {
	Procedure string  `json:"procedure"`
	ProductID int64   `json:"productId"`
	Quantity  float64 `json:"quantity"`
}

func listInventoryRules(w http.ResponseWriter, r *http.Request) {
	rows, err := db.QueryContext(r.Context(), `SELECT r.id,r.procedure_match,r.product_id,p.name,p.unit,r.quantity FROM inventory_consumption_rules r JOIN inventory_products p ON p.id=r.product_id WHERE r.clinic_id=$1 AND r.active ORDER BY r.procedure_match,p.name`, clinicID)
	if err != nil {
		writeError(w, 500, "No se pudieron cargar las recetas de consumo")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, pid int64
		var proc, name, unit string
		var qty float64
		if rows.Scan(&id, &proc, &pid, &name, &unit, &qty) != nil {
			writeError(w, 500, "No se pudieron leer las reglas")
			return
		}
		out = append(out, map[string]any{"id": id, "procedure": proc, "productId": pid, "product": name, "unit": unit, "quantity": qty})
	}
	writeJSON(w, 200, out)
}
func createInventoryRule(w http.ResponseWriter, r *http.Request) {
	var in inventoryRuleInput
	if !decode(w, r, &in) {
		return
	}
	in.Procedure = strings.TrimSpace(in.Procedure)
	if in.Procedure == "" || in.ProductID < 1 || in.Quantity <= 0 {
		writeError(w, 400, "Procedimiento, insumo y cantidad válida son obligatorios")
		return
	}
	var id int64
	err := db.QueryRowContext(r.Context(), `INSERT INTO inventory_consumption_rules(clinic_id,procedure_match,product_id,quantity,created_by) SELECT $1,$2,p.id,$4,$5 FROM inventory_products p WHERE p.id=$3 AND p.clinic_id=$1 AND p.active RETURNING id`, clinicID, in.Procedure, in.ProductID, in.Quantity, actorID(r)).Scan(&id)
	if err != nil {
		writeError(w, 409, "No se pudo guardar la regla; verifica el insumo y que no exista una regla igual")
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "procedure": in.Procedure, "productId": in.ProductID, "quantity": in.Quantity})
}
func deleteInventoryRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "Regla inválida")
		return
	}
	result, err := db.ExecContext(r.Context(), `UPDATE inventory_consumption_rules SET active=FALSE WHERE id=$1 AND clinic_id=$2 AND active`, id, clinicID)
	if err != nil {
		writeError(w, 500, "No se pudo desactivar la regla")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, 404, "Regla no encontrada")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type commissionRuleInput struct {
	ProviderID    int64   `json:"providerId"`
	Procedure     string  `json:"procedure"`
	Type          string  `json:"type"`
	Value         float64 `json:"value"`
	Fee           float64 `json:"fee"`
	LabCost       float64 `json:"labCost"`
	MaterialsCost float64 `json:"materialsCost"`
}

func listCommissionRules(w http.ResponseWriter, r *http.Request) {
	providers, err := db.QueryContext(r.Context(), `SELECT u.id,u.full_name FROM users u JOIN clinic_members m ON m.user_id=u.id WHERE m.clinic_id=$1 AND m.role='odontologo' AND m.active AND u.active ORDER BY u.full_name`, clinicID)
	if err != nil {
		writeError(w, 500, "No se pudieron cargar los profesionales")
		return
	}
	providerList := []map[string]any{}
	for providers.Next() {
		var id int64
		var name string
		if providers.Scan(&id, &name) != nil {
			providers.Close()
			writeError(w, 500, "No se pudieron leer los profesionales")
			return
		}
		providerList = append(providerList, map[string]any{"id": id, "name": name})
	}
	providers.Close()
	rows, err := db.QueryContext(r.Context(), `SELECT r.id,r.provider_id,u.full_name,r.procedure_match,r.calculation_type,r.calculation_value,r.procedure_fee,r.lab_cost,r.materials_cost FROM provider_commission_rules r JOIN users u ON u.id=r.provider_id WHERE r.clinic_id=$1 AND r.active ORDER BY u.full_name,r.procedure_match`, clinicID)
	if err != nil {
		writeError(w, 500, "No se pudieron cargar las comisiones")
		return
	}
	defer rows.Close()
	rules := []map[string]any{}
	for rows.Next() {
		var id, pid int64
		var provider, procedure, typ string
		var value, fee, lab, materials float64
		if rows.Scan(&id, &pid, &provider, &procedure, &typ, &value, &fee, &lab, &materials) != nil {
			writeError(w, 500, "No se pudieron leer las comisiones")
			return
		}
		rules = append(rules, map[string]any{"id": id, "providerId": pid, "provider": provider, "procedure": procedure, "type": typ, "value": value, "fee": fee, "labCost": lab, "materialsCost": materials})
	}
	writeJSON(w, 200, map[string]any{"providers": providerList, "rules": rules})
}
func createCommissionRule(w http.ResponseWriter, r *http.Request) {
	var in commissionRuleInput
	if !decode(w, r, &in) {
		return
	}
	in.Procedure = strings.TrimSpace(in.Procedure)
	if in.Procedure == "" {
		in.Procedure = "*"
	}
	if in.ProviderID < 1 || (in.Type != "percentage" && in.Type != "fixed" && in.Type != "net_percentage") || in.Value < 0 || in.Fee < 0 || in.LabCost < 0 || in.MaterialsCost < 0 || (in.Type == "percentage" || in.Type == "net_percentage") && in.Value > 100 {
		writeError(w, 400, "Regla de comisión o montos inválidos")
		return
	}
	var id int64
	err := db.QueryRowContext(r.Context(), `INSERT INTO provider_commission_rules(clinic_id,provider_id,procedure_match,calculation_type,calculation_value,procedure_fee,lab_cost,materials_cost,created_by) SELECT $1,m.user_id,$3,$4,$5,$6,$7,$8,$9 FROM clinic_members m WHERE m.clinic_id=$1 AND m.user_id=$2 AND m.role='odontologo' AND m.active RETURNING id`, clinicID, in.ProviderID, in.Procedure, in.Type, in.Value, in.Fee, in.LabCost, in.MaterialsCost, actorID(r)).Scan(&id)
	if err != nil {
		writeError(w, 409, "No se pudo registrar la comisión; revisa profesional y regla repetida")
		return
	}
	writeJSON(w, 201, map[string]any{"id": id})
}
func deleteCommissionRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "Regla inválida")
		return
	}
	result, err := db.ExecContext(r.Context(), `UPDATE provider_commission_rules SET active=FALSE WHERE id=$1 AND clinic_id=$2 AND active`, id, clinicID)
	if err != nil {
		writeError(w, 500, "No se pudo desactivar la comisión")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeError(w, 404, "Regla no encontrada")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func reportSummary(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	now := time.Now().In(time.FixedZone("ECT", -5*60*60))
	if _, err := time.Parse("2006-01-02", from); err != nil {
		from = now.AddDate(0, 0, -29).Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		to = now.Format("2006-01-02")
	}
	if from > to {
		writeError(w, 400, "El rango de fechas es inválido")
		return
	}
	var appointments, attended, noShow, cancelled int
	err := db.QueryRowContext(r.Context(), `SELECT count(*),count(*) FILTER(WHERE status='attended'),count(*) FILTER(WHERE status='no_show'),count(*) FILTER(WHERE status='cancelled') FROM appointments WHERE clinic_id=$1 AND (starts_at AT TIME ZONE 'America/Guayaquil')::date BETWEEN $2::date AND $3::date`, clinicID, from, to).Scan(&appointments, &attended, &noShow, &cancelled)
	if err != nil {
		writeError(w, 500, "No se pudieron calcular las citas")
		return
	}
	var collected, receivable float64
	if err = db.QueryRowContext(r.Context(), `SELECT COALESCE((SELECT sum(p.amount) FROM payments p JOIN invoices i ON i.id=p.invoice_id WHERE i.clinic_id=$1 AND (p.received_at AT TIME ZONE 'America/Guayaquil')::date BETWEEN $2::date AND $3::date),0),COALESCE((SELECT sum(i.total-COALESCE(x.paid,0)) FROM invoices i LEFT JOIN (SELECT invoice_id,sum(amount) AS paid FROM payments GROUP BY invoice_id) x ON x.invoice_id=i.id WHERE i.clinic_id=$1 AND i.status<>'void' AND (i.issued_at AT TIME ZONE 'America/Guayaquil')::date BETWEEN $2::date AND $3::date),0)`, clinicID, from, to).Scan(&collected, &receivable); err != nil {
		writeError(w, 500, "No se pudieron calcular los cobros")
		return
	}
	var lowStock int
	if err = db.QueryRowContext(r.Context(), `SELECT count(*) FROM (SELECT p.id,p.minimum_stock,COALESCE(sum(CASE WHEN m.movement_type='consumption' THEN -m.quantity ELSE m.quantity END),0) stock FROM inventory_products p LEFT JOIN inventory_movements m ON m.product_id=p.id WHERE p.clinic_id=$1 AND p.active GROUP BY p.id) s WHERE s.stock<=s.minimum_stock`, clinicID).Scan(&lowStock); err != nil {
		writeError(w, 500, "No se pudo calcular el inventario bajo")
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT u.full_name,count(*),sum(CASE r.calculation_type WHEN 'fixed' THEN r.calculation_value WHEN 'percentage' THEN r.procedure_fee*r.calculation_value/100 ELSE GREATEST(r.procedure_fee-r.lab_cost-r.materials_cost,0)*r.calculation_value/100 END) FROM appointments a JOIN users u ON u.id=a.provider_id JOIN LATERAL(SELECT x.calculation_type,x.calculation_value,x.procedure_fee,x.lab_cost,x.materials_cost FROM provider_commission_rules x WHERE x.clinic_id=a.clinic_id AND x.provider_id=a.provider_id AND x.active AND (x.procedure_match='*' OR lower(trim(x.procedure_match))=lower(trim(a.reason))) ORDER BY CASE WHEN x.procedure_match='*' THEN 1 ELSE 0 END LIMIT 1) r ON TRUE WHERE a.clinic_id=$1 AND a.status='attended' AND (a.starts_at AT TIME ZONE 'America/Guayaquil')::date BETWEEN $2::date AND $3::date GROUP BY u.id,u.full_name ORDER BY u.full_name`, clinicID, from, to)
	if err != nil {
		writeError(w, 500, "No se pudieron calcular honorarios")
		return
	}
	defer rows.Close()
	commissions := []map[string]any{}
	var commissionTotal float64
	for rows.Next() {
		var name string
		var count int
		var amount float64
		if rows.Scan(&name, &count, &amount) != nil {
			writeError(w, 500, "No se pudo leer la liquidación")
			return
		}
		commissionTotal += amount
		commissions = append(commissions, map[string]any{"provider": name, "procedures": count, "amount": amount})
	}
	writeJSON(w, 200, map[string]any{"from": from, "to": to, "appointments": map[string]int{"total": appointments, "attended": attended, "noShow": noShow, "cancelled": cancelled}, "collected": collected, "receivable": receivable, "lowStock": lowStock, "commissionTotal": commissionTotal, "commissions": commissions})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := origin == "http://localhost:5174" || origin == "http://127.0.0.1:5174"
		for _, o := range strings.Split(env("CORS_ORIGINS", ""), ",") {
			if originMatches(strings.TrimSpace(o), origin) {
				allowed = true
			}
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func originMatches(rule, origin string) bool {
	if rule == "" || origin == "" {
		return false
	}
	if rule == origin {
		return true
	}
	const wildcardScheme = "https://*"
	if !strings.HasPrefix(rule, wildcardScheme) || !strings.HasPrefix(origin, "https://") {
		return false
	}
	suffix := strings.TrimPrefix(rule, wildcardScheme)
	host := strings.TrimPrefix(origin, "https://")
	return suffix != "" && strings.HasSuffix(host, suffix) && len(host) > len(suffix)
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(value); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"status": status, "message": message, "error": message})
}

func statusCode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "confirmada", "confirmed":
		return "confirmed"
	case "en sala de espera", "waiting":
		return "waiting"
	case "en atención", "en atencion", "in_progress":
		return "in_progress"
	case "atendido", "attended":
		return "attended"
	case "cancelada", "cancelado", "cancelled":
		return "cancelled"
	case "no asistió", "no asistio", "no_show":
		return "no_show"
	default:
		return "scheduled"
	}
}

func statusLabel(s string) string {
	switch s {
	case "confirmed":
		return "Confirmada"
	case "waiting":
		return "En sala de espera"
	case "in_progress":
		return "En atención"
	case "attended":
		return "Atendido"
	case "cancelled":
		return "Cancelada"
	case "no_show":
		return "No asistió"
	default:
		return "Programada"
	}
}

func invoiceLabel(s string) string {
	switch s {
	case "paid":
		return "Pagada"
	case "partially_paid":
		return "Abono"
	case "void":
		return "Anulada"
	default:
		return "Pendiente"
	}
}

func paymentCode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "tarjeta", "card":
		return "card"
	case "transferencia", "transfer":
		return "transfer"
	case "seguro / convenio", "insurance":
		return "insurance"
	case "otro", "other":
		return "other"
	default:
		return "cash"
	}
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return strings.TrimSpace(value)
}

func env(name, defaultValue string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return defaultValue
}
