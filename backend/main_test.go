package main

import (
	"testing"
	"time"
)

func TestAppointmentStatusMappings(t *testing.T) {
	cases := []struct{ input, code, label string }{
		{"Programada", "scheduled", "Programada"},
		{"Confirmada", "confirmed", "Confirmada"},
		{"En sala de espera", "waiting", "En sala de espera"},
		{"En atención", "in_progress", "En atención"},
		{"Atendido", "attended", "Atendido"},
		{"Cancelada", "cancelled", "Cancelada"},
		{"No asistió", "no_show", "No asistió"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			if got := statusCode(tc.input); got != tc.code {
				t.Fatalf("statusCode(%q)=%q, want %q", tc.input, got, tc.code)
			}
			if got := statusLabel(tc.code); got != tc.label {
				t.Fatalf("statusLabel(%q)=%q, want %q", tc.code, got, tc.label)
			}
		})
	}
}

func TestPaymentMethodMapping(t *testing.T) {
	cases := map[string]string{"Efectivo": "cash", "Tarjeta": "card", "Transferencia": "transfer", "Seguro / convenio": "insurance", "Otro": "other"}
	for input, want := range cases {
		if got := paymentCode(input); got != want {
			t.Errorf("paymentCode(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestInvoiceStatusLabels(t *testing.T) {
	cases := map[string]string{"paid": "Pagada", "partially_paid": "Abono", "void": "Anulada", "pending": "Pendiente"}
	for input, want := range cases {
		if got := invoiceLabel(input); got != want {
			t.Errorf("invoiceLabel(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestTOTPEnrollmentAndValidation(t *testing.T) {
	mfaKey = deriveMFAKey("test-only-encryption-key")
	secret := "JBSWY3DPEHPK3PXP"
	instant := time.Unix(1_700_000_000, 0)
	code, err := totpCode(secret, instant.Unix())
	if err != nil {
		t.Fatal(err)
	}
	if !validTOTP(secret, code, instant) {
		t.Fatalf("generated TOTP %s should verify", code)
	}
	if validTOTP(secret, "not-a-code", instant) {
		t.Fatal("non-numeric TOTP must be rejected")
	}
	wrongCode := "000000"
	if code == wrongCode {
		wrongCode = "000001"
	}
	if validTOTP(secret, wrongCode, instant) {
		t.Fatal("incorrect TOTP should be rejected")
	}
	sealed, err := encryptMFASecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := decryptMFASecret(sealed)
	if err != nil || opened != secret {
		t.Fatalf("MFA secret encryption round-trip failed: %v", err)
	}
}
