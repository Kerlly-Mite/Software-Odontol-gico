const API_ROOT = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

async function request(path, options = {}) {
  let session = null;
  try { session = JSON.parse(localStorage.getItem('sigo-session') || 'null'); } catch { /* sesión vacía */ }
  const authHeaders = session?.token ? { Authorization: `Bearer ${session.token}` } : {};
  const response = await fetch(`${API_ROOT}${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...authHeaders, ...options.headers },
  });
  const result = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(result.message || result.error || `API ${response.status}`);
    error.status = response.status;
    error.mfaRequired = Boolean(result.mfa_required);
    throw error;
  }
  return result;
}

export const api = {
  health: () => request('/api/health'),
  login: (email, password, code = '') => request('/api/auth/login', { method: 'POST', body: JSON.stringify({ email, password, code }) }),
  startMFASetup: () => request('/api/auth/mfa/setup', { method: 'POST', body: JSON.stringify({}) }),
  verifyMFASetup: (code) => request('/api/auth/mfa/verify-setup', { method: 'POST', body: JSON.stringify({ code }) }),
  patients: () => request('/api/patients'),
  createPatient: (data) => request('/api/patients', { method: 'POST', body: JSON.stringify(data) }),
  updatePatient: (id, data) => request(`/api/patients/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  treatmentPlans: (patientId) => request(`/api/treatment-plans${patientId ? `?patientId=${patientId}` : ''}`),
  createTreatmentPlan: (data) => request('/api/treatment-plans', { method: 'POST', body: JSON.stringify(data) }),
  approveTreatmentPlan: (id, itemIds) => request(`/api/treatment-plans/${id}/approval`, { method: 'POST', body: JSON.stringify({ itemIds }) }),
  closeCash: (declaredCash) => request('/api/cash/close', { method: 'POST', body: JSON.stringify({ declaredCash }) }),
  appointments: () => request('/api/appointments'),
  createAppointment: (data) => request('/api/appointments', { method: 'POST', body: JSON.stringify(data) }),
  updateAppointmentStatus: (id, status) => request(`/api/appointments/${id}/status`, { method: 'PATCH', body: JSON.stringify({ status }) }),
  invoices: () => request('/api/invoices'),
  createInvoice: (data) => request('/api/invoices', { method: 'POST', body: JSON.stringify(data) }),
  clinicalNotes: (patientId) => request(`/api/patients/${patientId}/clinical-notes`),
  createClinicalNote: (patientId, data) => request(`/api/patients/${patientId}/clinical-notes`, { method: 'POST', body: JSON.stringify(data) }),
  createClinicalAmendment: (patientId, noteId, data) => request(`/api/patients/${patientId}/clinical-notes/${noteId}/amendments`, { method: 'POST', body: JSON.stringify(data) }),
  odontogram: (patientId) => request(`/api/patients/${patientId}/odontograms`),
  saveOdontogram: (patientId, data) => request(`/api/patients/${patientId}/odontograms`, { method: 'POST', body: JSON.stringify(data) }),
  periodontogram: (patientId) => request(`/api/patients/${patientId}/periodontogram`),
  savePeriodontogram: (patientId, data) => request(`/api/patients/${patientId}/periodontogram`, { method: 'PUT', body: JSON.stringify(data) }),
  prescriptions: (patientId) => request(`/api/patients/${patientId}/prescriptions`),
  createPrescription: (patientId, data) => request(`/api/patients/${patientId}/prescriptions`, { method: 'POST', body: JSON.stringify(data) }),
  consents: (patientId) => request(`/api/patients/${patientId}/consents`),
  createConsent: (patientId, data) => request(`/api/patients/${patientId}/consents`, { method: 'POST', body: JSON.stringify(data) }),
  signConsent: (patientId, consentId, data) => request(`/api/patients/${patientId}/consents/${consentId}/signature`, { method: 'POST', body: JSON.stringify(data) }),
  inventoryProducts: () => request('/api/inventory/products'),
  createInventoryProduct: (data) => request('/api/inventory/products', { method: 'POST', body: JSON.stringify(data) }),
  inventoryMovements: (productId) => request(`/api/inventory/movements${productId ? `?productId=${productId}` : ''}`),
  createInventoryMovement: (data) => request('/api/inventory/movements', { method: 'POST', body: JSON.stringify(data) }),
  inventoryRules: () => request('/api/inventory/consumption-rules'),
  createInventoryRule: (data) => request('/api/inventory/consumption-rules', { method: 'POST', body: JSON.stringify(data) }),
  deleteInventoryRule: (id) => request(`/api/inventory/consumption-rules/${id}`, { method: 'DELETE' }),
  commissionRules: () => request('/api/commissions/rules'),
  createCommissionRule: (data) => request('/api/commissions/rules', { method: 'POST', body: JSON.stringify(data) }),
  deleteCommissionRule: (id) => request(`/api/commissions/rules/${id}`, { method: 'DELETE' }),
  reportSummary: (from, to) => request(`/api/reports/summary?from=${from}&to=${to}`),
};
