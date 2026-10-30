import { useEffect, useMemo, useState } from 'react';
import { Activity, AlertTriangle, CalendarDays, ChevronDown, CircleDollarSign, ClipboardList, FileHeart, HeartPulse, LayoutDashboard, Menu, Pencil, Plus, Search, ShieldCheck, Stethoscope, Users, X } from 'lucide-react';
import { api } from './services/api.js';

const seedPatients = [
  { id: 1, name: 'María Fernanda López', doc: '0923456781', age: 34, phone: '099 482 1730', email: 'maria.lopez@email.com', blood: 'O+', allergies: 'Penicilina', conditions: 'Hipertensión controlada', insurance: 'Particular', last: '24 oct 2026' },
  { id: 2, name: 'Carlos Andrés Vera', doc: '0912345678', age: 42, phone: '098 310 4462', email: 'carlos.vera@email.com', blood: 'A+', allergies: 'Ninguna conocida', conditions: 'Ninguna', insurance: 'IESS', last: '27 oct 2026' },
  { id: 3, name: 'Sofía Mite González', doc: '0929876543', age: 12, phone: '099 730 2051', email: 'contacto@familia.ec', blood: 'B+', allergies: 'Látex', conditions: 'Asma leve', insurance: 'Particular', last: '20 oct 2026' },
  { id: 4, name: 'José Miguel Cedeño', doc: '0908765432', age: 56, phone: '096 557 0182', email: 'jose.cedeno@email.com', blood: 'O-', allergies: 'Ninguna conocida', conditions: 'Diabetes tipo 2', insurance: 'Saludsa', last: '18 oct 2026' },
];
const seedVisits = [
  { id: 1, patient: 'María Fernanda López', doctor: 'Dra. Valeria Ruiz', procedure: 'Control y profilaxis', time: '09:00', date: '2026-10-29', chair: 'Sillón 1', status: 'Confirmada' },
  { id: 2, patient: 'Carlos Andrés Vera', doctor: 'Dr. Daniel Paredes', procedure: 'Endodoncia', time: '09:30', date: '2026-10-29', chair: 'Sillón 2', status: 'En atención' },
  { id: 3, patient: 'Sofía Mite González', doctor: 'Dra. Valeria Ruiz', procedure: 'Control de ortodoncia', time: '10:00', date: '2026-10-29', chair: 'Sillón 1', status: 'Confirmada' },
  { id: 4, patient: 'José Miguel Cedeño', doctor: 'Dr. Daniel Paredes', procedure: 'Valoración', time: '11:00', date: '2026-10-29', chair: 'Sillón 2', status: 'Programada' },
];
const getSaved = (key, initial) => { try { return JSON.parse(localStorage.getItem(key)) ?? initial; } catch { return initial; } };
const money = (n) => new Intl.NumberFormat('es-EC', { style: 'currency', currency: 'USD' }).format(n);
const universalToothLabels = { 18:'1',17:'2',16:'3',15:'4',14:'5',13:'6',12:'7',11:'8',21:'9',22:'10',23:'11',24:'12',25:'13',26:'14',27:'15',28:'16',38:'17',37:'18',36:'19',35:'20',34:'21',33:'22',32:'23',31:'24',41:'25',42:'26',43:'27',44:'28',45:'29',46:'30',47:'31',48:'32',55:'A',54:'B',53:'C',52:'D',51:'E',61:'F',62:'G',63:'H',64:'I',65:'J',75:'K',74:'L',73:'M',72:'N',71:'O',81:'P',82:'Q',83:'R',84:'S',85:'T' };
const toothLabel = (fdi, dentition, notation) => notation === 'Universal' ? (universalToothLabels[fdi] || String(fdi)) : String(fdi);
const today = (() => { const d = new Date(); return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`; })();
const initialInvoice = [{ id: 'FAC-00031', patient: 'María Fernanda López', concept: 'Profilaxis y control', total: 45, paid: 45, date: today, state: 'Pagada' }, { id: 'FAC-00030', patient: 'Carlos Andrés Vera', concept: 'Tratamiento de endodoncia', total: 280, paid: 120, date: today, state: 'Abono' }];

export default function App() {
  const [section, setSection] = useState('Inicio');
  const [patients, setPatients] = useState(() => getSaved('sigo-patients', seedPatients));
  const [visits, setVisits] = useState(() => getSaved('sigo-visits', seedVisits));
  const [invoices, setInvoices] = useState(() => getSaved('sigo-invoices', initialInvoice));
  const [selected, setSelected] = useState(null);
  const [query, setQuery] = useState('');
  const [modal, setModal] = useState('');
  const [toast, setToast] = useState('');
  const [apiStatus, setApiStatus] = useState('Conectando a PostgreSQL…');
  const [backendReady, setBackendReady] = useState(false);
  const [session, setSession] = useState(() => getSaved('sigo-session', null));
  const [sessionNotice, setSessionNotice] = useState('');
  const [tab, setTab] = useState('Odontograma');
  const [toothStatus, setToothStatus] = useState(() => getSaved('sigo-teeth', {}));
  const [surfaceStatus, setSurfaceStatus] = useState(() => getSaved('sigo-surfaces', {}));
  const [notes, setNotes] = useState(() => getSaved('sigo-notes', []));
  const [prescriptions, setPrescriptions] = useState(() => getSaved('sigo-rx', []));
  const [consents, setConsents] = useState(() => getSaved('sigo-consents', []));
  const [period, setPeriod] = useState(() => getSaved('sigo-period', {}));

  const persist = (key, value, setter) => { setter(value); localStorage.setItem(key, JSON.stringify(value)); };
  const notify = (message) => { setToast(message); window.setTimeout(() => setToast(''), 3000); };
  useEffect(() => {
    let active = true;
    if (!session?.token) {
      setBackendReady(false);
      setApiStatus('Inicia sesión');
      return () => { active = false; };
    }
    if (!session.user?.mfaEnabled) {
      setBackendReady(false);
      setApiStatus('Configura MFA para continuar');
      return () => { active = false; };
    }
    const boot = async () => {
      try {
        await api.health();
        let remotePatients = await api.patients();
        if (!remotePatients.length) {
          const seed = getSaved('sigo-patients', seedPatients);
          for (const patient of seed) await api.createPatient({ ...patient, age: Number(patient.age || 0) });
          remotePatients = await api.patients();
        }
        let remoteVisits = await api.appointments();
        if (!remoteVisits.length && remotePatients.length) {
          const seed = getSaved('sigo-visits', seedVisits);
          for (const visit of seed) {
            try { await api.createAppointment(visit); } catch (error) { console.warn('No se pudo importar una cita de demostración:', error.message); }
          }
          remoteVisits = await api.appointments();
        }
        let remoteInvoices = await api.invoices();
        if (!remoteInvoices.length && remotePatients.length) {
          const seed = getSaved('sigo-invoices', initialInvoice);
          for (const invoice of seed) {
            try { await api.createInvoice({ patient: invoice.patient, concept: invoice.concept, total: invoice.total, paid: invoice.paid, method: 'Efectivo' }); } catch (error) { console.warn('No se pudo importar un cobro de demostración:', error.message); }
          }
          remoteInvoices = await api.invoices();
        }
        if (!active) return;
        persist('sigo-patients', remotePatients, setPatients);
        persist('sigo-visits', remoteVisits, setVisits);
        persist('sigo-invoices', remoteInvoices, setInvoices);
        setBackendReady(true);
        setApiStatus('PostgreSQL conectado');
      } catch (error) {
        if (!active) return;
        if (error.status === 401) {
          localStorage.removeItem('sigo-session');
          setSession(null);
          setApiStatus('Sesión vencida');
          return;
        }
        setBackendReady(false);
        setApiStatus('Modo demostración local');
        console.warn('API SIGO-Dental no disponible; se conserva el modo local:', error.message);
      }
    };
    boot();
    return () => { active = false; };
  }, [session?.token, session?.user?.mfaEnabled]);

  useEffect(() => {
    if (!session?.token) return undefined;
    let timer;
    const expire = () => {
      localStorage.removeItem('sigo-session');
      setSession(null);
      setBackendReady(false);
      setSelected(null);
      setSection('Inicio');
      setSessionNotice('La sesión se cerró por 15 minutos de inactividad. Inicia sesión nuevamente.');
    };
    const resetTimer = () => { window.clearTimeout(timer); timer = window.setTimeout(expire, 15 * 60 * 1000); };
    const events = ['mousemove','mousedown','keydown','touchstart','scroll'];
    events.forEach(event => window.addEventListener(event, resetTimer, { passive: true }));
    resetTimer();
    return () => { window.clearTimeout(timer); events.forEach(event => window.removeEventListener(event, resetTimer)); };
  }, [session?.token]);

  const handleLogin = async (email, password, code) => {
    const result = await api.login(email, password, code);
    const saved = { token: result.token, user: result.user };
    persist('sigo-session', saved, setSession);
    setSessionNotice('');
  };
  const handleLogout = () => {
    localStorage.removeItem('sigo-session');
    setSession(null);
    setBackendReady(false);
    setSelected(null);
    setSection('Inicio');
  };

  const savePatient = async (patient) => {
    if (backendReady) {
      const saved = await api.createPatient(patient);
      persist('sigo-patients', [saved, ...patients], setPatients);
      return;
    }
    persist('sigo-patients', [{ ...patient, id: Date.now(), last: 'Sin visitas' }, ...patients], setPatients);
  };
  const editPatient = async (patient) => {
    const saved = backendReady ? await api.updatePatient(patient.id, patient) : { ...patient };
    persist('sigo-patients', patients.map(p => p.id === saved.id ? { ...p, ...saved } : p), setPatients);
    if (selected?.id === saved.id) setSelected({ ...selected, ...saved });
  };
  const saveVisit = async (visit) => {
    if (backendReady) {
      const saved = await api.createAppointment(visit);
      persist('sigo-visits', [...visits, saved].sort((a, b) => a.time.localeCompare(b.time)), setVisits);
      return;
    }
    persist('sigo-visits', [...visits, { ...visit, id: Date.now() }].sort((a, b) => a.time.localeCompare(b.time)), setVisits);
  };
  const changeVisits = (next) => {
    const changed = next.filter(v => { const old = visits.find(x => x.id === v.id); return old && old.status !== v.status; });
    persist('sigo-visits', next, setVisits);
    if (backendReady) changed.forEach(v => api.updateAppointmentStatus(v.id, v.status).then(result => { if (result.inventoryWarnings?.length) notify(result.inventoryWarnings.join('. ')); }).catch(error => notify(error.message)));
  };
  const saveInvoice = async (invoice) => {
    if (backendReady) {
      const saved = await api.createInvoice(invoice);
      persist('sigo-invoices', [saved, ...invoices], setInvoices);
      return;
    }
    persist('sigo-invoices', [{ ...invoice, id: `FAC-${String(32 + invoices.length).padStart(5, '0')}`, date: today, state: invoice.paid >= invoice.total ? 'Pagada' : 'Abono' }, ...invoices], setInvoices);
  };
  const matchingPatients = useMemo(() => patients.filter(p => `${p.name} ${p.doc} ${p.phone}`.toLowerCase().includes(query.toLowerCase())), [patients, query]);
  const openPatient = (p) => { setSelected(p); setSection('Expediente'); setTab('Odontograma'); };
  const patientInitials = (name) => name.split(' ').slice(0, 2).map(x => x[0]).join('');

  const access = {
    admin_ti: ['Inicio','Pacientes','Agenda','Expediente','Planes de tratamiento','Caja y facturación','Inventario','Reportes'],
    recepcionista: ['Inicio','Pacientes','Agenda','Planes de tratamiento','Caja y facturación','Inventario'],
    odontologo: ['Inicio','Pacientes','Agenda','Expediente','Planes de tratamiento'],
    asistente: ['Inicio','Pacientes','Agenda','Expediente','Planes de tratamiento','Inventario'],
    auditor: ['Inicio','Planes de tratamiento','Caja y facturación','Inventario','Reportes'],
  };
  const allowed = new Set(access[session?.user?.role] || []);
  const nav = [
    { title: 'CLÍNICA', items: [['Inicio', LayoutDashboard], ['Pacientes', Users], ['Agenda', CalendarDays], ['Expediente', FileHeart]].filter(([label]) => allowed.has(label)) },
    { title: 'ADMINISTRACIÓN', items: [['Planes de tratamiento', ClipboardList], ['Caja y facturación', CircleDollarSign], ['Inventario', ClipboardList], ['Reportes', Activity]].filter(([label]) => allowed.has(label)) },
  ].filter(group => group.items.length > 0);
  const roleLabel = { admin_ti: 'Administración TI', recepcionista: 'Recepcionista', odontologo: 'Odontólogo', asistente: 'Asistente dental', auditor: 'Auditor' }[session?.user?.role] || 'Usuario';
  const initials = session?.user?.name?.split(' ').slice(0,2).map(part => part[0]).join('') || 'SD';
  const pageTitle = section === 'Expediente' ? 'Expediente clínico' : section;

  if (!session?.token) return <LoginPage onLogin={handleLogin} notice={sessionNotice}/>;
  if (!session.user?.mfaEnabled) return <MFASetupPage user={session.user} onComplete={() => { const upgraded={...session,user:{...session.user,mfaEnabled:true}}; persist('sigo-session',upgraded,setSession); setSessionNotice(''); }}/>

  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><div className="brand-mark"><Stethoscope size={21}/></div><div><b>SIGO<span>·</span>Dental</b><small>GESTIÓN CLÍNICA</small></div></div>
      <div className="clinic-switch"><div className="clinic-icon">CD</div><div><b>Clínica Dental Sonrisa</b><small>Sucursal principal</small></div><ChevronDown size={15}/></div>
      <nav>{nav.map(group => <div className="nav-group" key={group.title}><div className="nav-label">{group.title}</div>{group.items.map(([label, Icon]) => <button key={label} onClick={() => { setSection(label); if (label !== 'Expediente') setSelected(null); }} className={`nav-item ${section === label ? 'active' : ''}`}><Icon size={18}/><span>{label}</span>{label === 'Pacientes' && <span className="nav-count">{patients.length}</span>}</button>)}</div>)}</nav>
      <div className="sidebar-bottom"><div className="help-card"><div className="help-icon"><ShieldCheck size={17}/></div><b>Atención segura</b><p>Registra cada atención para mantener el expediente completo.</p><button onClick={() => notify('Guía de uso próximamente')}>Ver guía <span>→</span></button></div><div className="user-profile"><div className="avatar avatar-teal">{initials}</div><div><b>{session.user.name}</b><small>{roleLabel}</small></div><button className="logout-button" onClick={handleLogout}>Salir</button></div></div>
    </aside>
    <main className="main-area">
      <header className="topbar"><button className="mobile-menu" onClick={() => notify('Abre el menú desde una pantalla amplia')}><Menu size={20}/></button><div className="breadcrumbs">Clínica <span>/</span> <b>{pageTitle}</b>{selected && section === 'Expediente' && <><span>/</span><b>{selected.name}</b></>}</div><div className="top-actions"><div className="today-label"><CalendarDays size={16}/>{new Intl.DateTimeFormat('es-EC', { weekday: 'long', day: 'numeric', month: 'long' }).format(new Date())}</div><button className="icon-button" title="Notificaciones" onClick={() => notify('No tienes notificaciones nuevas')}><span className="notification-dot"/>◉</button><div className="avatar avatar-small">{initials}</div></div></header>
      <div className="content">
        {section === 'Inicio' && <Dashboard patients={patients} visits={visits} invoices={invoices} openSection={setSection} openPatient={openPatient} money={money}/>}
        {section === 'Pacientes' && <Patients patients={matchingPatients} query={query} setQuery={setQuery} onNew={() => setModal('patient')} onOpen={openPatient} onEdit={p => { setSelected(p); setModal('editPatient'); }} initials={patientInitials}/>}
        {section === 'Agenda' && <Schedule visits={visits} setVisits={changeVisits} onNew={() => setModal('visit')} notify={notify}/>}
        {section === 'Planes de tratamiento' && <TreatmentPlans backendReady={backendReady} patients={patients} canCreate={['admin_ti','odontologo'].includes(session.user.role)} canApprove={['admin_ti','odontologo','recepcionista'].includes(session.user.role)} notify={notify}/>}
        {section === 'Expediente' && (selected || patients[0]) && <ClinicalRecord backendReady={backendReady} canAmend={['admin_ti','odontologo'].includes(session.user.role)} patient={selected || patients[0]} patients={patients} onSelect={setSelected} tab={tab} setTab={setTab} toothStatus={toothStatus} setToothStatus={v => persist('sigo-teeth', v, setToothStatus)} surfaceStatus={surfaceStatus} setSurfaceStatus={v => persist('sigo-surfaces', v, setSurfaceStatus)} notes={notes} setNotes={v => persist('sigo-notes', v, setNotes)} prescriptions={prescriptions} setPrescriptions={v => persist('sigo-rx', v, setPrescriptions)} consents={consents} setConsents={v => persist('sigo-consents', v, setConsents)} period={period} setPeriod={v => persist('sigo-period', v, setPeriod)} notify={notify} onNew={kind => setModal(kind)}/>}
        {section === 'Caja y facturación' && <Billing backendReady={backendReady} canCloseCash={['admin_ti','recepcionista'].includes(session.user.role)} hideTillSummary={session.user.role==='recepcionista'} invoices={invoices} money={money} onNew={() => setModal('invoice')} notify={notify}/>}
        {section === 'Inventario' && <Inventory backendReady={backendReady} canWrite={['admin_ti','recepcionista','asistente'].includes(session.user.role)} canManage={['admin_ti','recepcionista'].includes(session.user.role)} notify={notify}/>}
        {section === 'Reportes' && <Reports backendReady={backendReady} canManage={session.user.role==='admin_ti'} notify={notify}/>}
      </div>
    </main>
    {modal && <Modal backendReady={backendReady} kind={modal} patients={patients} visits={visits} patient={selected || patients[0]} onCreatePatient={savePatient} onUpdatePatient={editPatient} onCreateVisit={saveVisit} onCreateInvoice={saveInvoice} onSaveRx={async form => { const p = patients.find(x => x.name === form.patient) || selected || patients[0]; const saved = backendReady ? await api.createPrescription(p.id, form) : { ...form, id: Date.now(), patient: p?.id, date: new Date().toLocaleDateString('es-EC') }; persist('sigo-rx', [saved, ...prescriptions.filter(x => x.id !== saved.id)], setPrescriptions); }} onSaveConsent={async form => { const p = patients.find(x => x.name === form.patient) || selected || patients[0]; const saved = backendReady ? await api.createConsent(p.id, form) : { ...form, id: Date.now(), patient: p?.id, date: new Date().toLocaleDateString('es-EC'), state: 'Pendiente de firma' }; persist('sigo-consents', [saved, ...consents.filter(x => x.id !== saved.id)], setConsents); }} setModal={setModal} notify={notify}/>}
    {toast && <div className="toast"><span>✓</span>{toast}</div>}
    <div className={`prototype-note ${backendReady ? 'api-online' : ''}`}><span/>{apiStatus}{backendReady ? ' · pacientes, agenda, expediente clínico, inventario, reportes y caja sincronizados' : ' · los cambios de este navegador se guardan localmente'}</div>
  </div>;
}

function LoginPage({ onLogin, notice }) {
  const [email, setEmail] = useState('admin@sigo.local');
  const [password, setPassword] = useState('Demo123!');
  const [code, setCode] = useState('');
  const [needsCode, setNeedsCode] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const submit = async event => {
    event.preventDefault(); setBusy(true); setError('');
    try { await onLogin(email, password, code); }
    catch (err) { if(err.mfaRequired) setNeedsCode(true); setError(err.mfaRequired ? 'Ingresa el código actual de tu aplicación autenticadora.' : err.message || 'No se pudo iniciar sesión.'); }
    finally { setBusy(false); }
  };
  return <div className="login-screen"><div className="login-card"><div className="login-brand"><div className="brand-mark"><Stethoscope size={22}/></div><div><b>SIGO<span>·</span>Dental</b><small>GESTIÓN CLÍNICA</small></div></div><h1>Bienvenida/o</h1><p className="login-subtitle">Ingresa a tu cuenta para continuar</p>{notice && <div className="login-info">{notice}</div>}{error && <div className="login-error">{error}</div>}<form onSubmit={submit}><label className="field"><span>Correo electrónico</span><input type="email" autoComplete="username" required value={email} onChange={e=>setEmail(e.target.value)}/></label><label className="field"><span>Contraseña</span><input type="password" autoComplete="current-password" required value={password} onChange={e=>setPassword(e.target.value)}/></label>{needsCode && <label className="field"><span>Código de autenticación</span><input inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} required value={code} onChange={e=>setCode(e.target.value.replace(/\D/g,''))} placeholder="000000"/></label>}<button className="btn btn-primary login-submit" disabled={busy}>{busy ? 'Ingresando…' : 'Iniciar sesión'}</button></form><div className="demo-access"><b>Acceso de demostración</b><span>Administrador: admin@sigo.local</span><span>Recepción: recepcion@sigo.local</span><span>Odontología: valeria.ruiz@sigo.local</span><small>Contraseña para estas cuentas: <b>Demo123!</b></small></div><p className="login-notice"><ShieldCheck size={14}/> Entorno de desarrollo con datos ficticios.</p></div></div>;
}

function MFASetupPage({ user, onComplete }) {
  const [setup,setSetup]=useState(null);
  const [code,setCode]=useState('');
  const [error,setError]=useState('');
  const [busy,setBusy]=useState(false);
  useEffect(()=>{let active=true;api.startMFASetup().then(data=>{if(active)setSetup(data)}).catch(err=>{if(active)setError(err.message||'No se pudo iniciar la configuración MFA.')});return()=>{active=false}},[]);
  const submit=async event=>{event.preventDefault();setBusy(true);setError('');try{await api.verifyMFASetup(code);onComplete()}catch(err){setError(err.message||'El código no es válido.')}finally{setBusy(false)}};
  return <div className="login-screen"><div className="login-card mfa-setup-card"><div className="login-brand"><div className="brand-mark"><ShieldCheck size={22}/></div><div><b>SIGO<span>·</span>Dental</b><small>SEGUNDO FACTOR</small></div></div><h1>Protege tu cuenta</h1><p className="login-subtitle">Antes de entrar, activa la verificación en dos pasos para {user?.email}.</p>{error && <div className="login-error">{error}</div>}{setup ? <><div className="mfa-instructions"><b>1. Abre una app autenticadora</b><span>Google Authenticator, Microsoft Authenticator u otra app compatible con TOTP.</span><b>2. Agrega la cuenta con esta clave</b><code>{setup.secret}</code><span>Cuenta: {setup.account}. Ingresa la clave manualmente si no puedes escanear un QR.</span></div><form onSubmit={submit}><label className="field"><span>3. Escribe el código de seis dígitos</span><input inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} required value={code} onChange={e=>setCode(e.target.value.replace(/\D/g,''))} placeholder="000000"/></label><button className="btn btn-primary login-submit" disabled={busy}>{busy?'Verificando…':'Activar y continuar'}</button></form></> : <p className="login-subtitle">Preparando la clave de configuración…</p>}<p className="login-notice"><ShieldCheck size={14}/> Guarda tu clave en la app autenticadora. No la compartas.</p></div></div>;
}

function PageHeading({ eyebrow, title, subtitle, action }) { return <div className="page-heading"><div><div className="eyebrow">{eyebrow}</div><h1>{title}</h1><p>{subtitle}</p></div>{action}</div>; }
function Avatar({ name, initials, tone = '' }) { return <span className={`avatar ${tone}`}>{initials || name.split(' ').slice(0, 2).map(x => x[0]).join('')}</span>; }
function Dashboard({ patients, visits, invoices, openSection, openPatient, money }) {
  const todayVisits = visits.filter(v => v.date === today).length;
  const todayRevenue = invoices.filter(i => i.date === today).reduce((a, i) => a + i.paid, 0);
  return <><PageHeading eyebrow="JUEVES, 29 DE OCTUBRE DE 2026" title="Buenos días, Valeria" subtitle="Aquí tienes el resumen de tu clínica para hoy." action={<button className="btn btn-primary" onClick={() => openSection('Agenda')}><Plus size={17}/> Nueva cita</button>}/>
    <div className="stat-grid"><Stat icon={Users} label="Pacientes activos" value={patients.length.toString()} detail="Fichas registradas" tone="teal"/><Stat icon={CalendarDays} label="Citas de hoy" value={todayVisits.toString().padStart(2, '0')} detail="En 2 sillones" tone="blue"/><Stat icon={CircleDollarSign} label="Ingresos del día" value={money(todayRevenue)} detail="Recaudado en caja" tone="peach"/><Stat icon={Activity} label="Tratamientos activos" value="12" detail="En seguimiento" tone="lavender"/></div>
    <div className="dashboard-grid"><div className="panel schedule-panel"><div className="panel-heading"><div><h2>Agenda de hoy</h2><p>Próximas atenciones programadas</p></div><button className="link-button" onClick={() => openSection('Agenda')}>Ver agenda <span>→</span></button></div>{visits.slice(0, 4).map((v, i) => <div className="appointment-row" key={v.id}><div className={`time-block time-${i % 3}`}><b>{v.time}</b><small>AM</small></div><div className="appointment-main"><b>{v.patient}</b><span>{v.procedure}</span><small>{v.doctor} · {v.chair}</small></div><Status label={v.status}/></div>)}</div>
      <div className="panel patients-panel"><div className="panel-heading"><div><h2>Pacientes recientes</h2><p>Últimas fichas consultadas</p></div><button className="link-button" onClick={() => openSection('Pacientes')}>Ver todos <span>→</span></button></div>{patients.slice(0, 4).map((p, i) => <button className="recent-patient" key={p.id} onClick={() => openPatient(p)}><Avatar name={p.name} tone={`tone-${i}`}/><span><b>{p.name}</b><small>{p.last} · {p.age} años</small></span><span className="arrow">›</span></button>)}</div></div>
    <div className="notice-strip"><div className="notice-icon"><HeartPulse size={20}/></div><div><b>Cuida los detalles clínicos</b><p>Registra alergias y antecedentes antes de iniciar un procedimiento.</p></div><button onClick={() => openSection('Pacientes')}>Revisar pacientes <span>→</span></button></div>
  </>;
}
function Stat({ icon: Icon, label, value, detail, tone }) { return <div className="stat-card"><div className={`stat-icon ${tone}`}><Icon size={19}/></div><div className="stat-label">{label}</div><div className="stat-value">{value}</div><div className="stat-detail">{detail}</div></div>; }
function Status({ label }) { const cls = label.toLowerCase().replaceAll(' ', '-'); return <span className={`status status-${cls}`}>{label}</span>; }

function Patients({ patients, query, setQuery, onNew, onOpen, onEdit, initials }) {
  return <><PageHeading eyebrow="GESTIÓN CLÍNICA" title="Pacientes" subtitle="Administra las fichas y datos de salud de tus pacientes." action={<button className="btn btn-primary" onClick={onNew}><Plus size={17}/> Registrar paciente</button>}/>
    <div className="toolbar"><div className="search-box"><Search size={18}/><input placeholder="Buscar por nombre, cédula o teléfono..." value={query} onChange={e => setQuery(e.target.value)}/><kbd>⌘ K</kbd></div><button className="btn btn-outline">Todos los pacientes <ChevronDown size={15}/></button></div>
    <div className="panel table-panel"><div className="table-top"><b>{patients.length} pacientes</b><span>Datos personales y clínicos</span></div><div className="table-scroll"><table><thead><tr><th>PACIENTE</th><th>CÉDULA</th><th>CONTACTO</th><th>ALERTA MÉDICA</th><th>ÚLTIMA VISITA</th><th></th></tr></thead><tbody>{patients.map((p, i) => <tr key={p.id} onClick={() => onOpen(p)}><td><div className="patient-cell"><Avatar name={p.name} initials={initials(p.name)} tone={`tone-${i % 4}`}/><span><b>{p.name}</b><small>{p.age} años · {p.insurance}</small></span></div></td><td>{p.doc}</td><td><b>{p.phone}</b><small className="subline">{p.email}</small></td><td>{p.allergies !== 'Ninguna conocida' || p.conditions !== 'Ninguna' ? <span className="alert-pill"><AlertTriangle size={13}/>{p.allergies !== 'Ninguna conocida' ? p.allergies : p.conditions}</span> : <span className="no-alert">Sin alertas</span>}</td><td>{p.last}</td><td><button className="row-more" title="Editar paciente" onClick={e => { e.stopPropagation(); onEdit(p); }}><Pencil size={15}/></button></td></tr>)}</tbody></table>{patients.length === 0 && <div className="empty-state">No hay pacientes que coincidan con la búsqueda.</div>}</div><div className="table-footer">Mostrando <b>{patients.length}</b> de <b>{patients.length}</b> pacientes <span>1 / 1</span></div></div>
  </>;
}

function Schedule({ visits, setVisits, onNew, notify }) {
  const [weekOffset, setWeekOffset] = useState(0);
  const [selectedDate, setSelectedDate] = useState(today);
  const weekStart = new Date(); weekStart.setDate(weekStart.getDate() - ((weekStart.getDay() + 6) % 7) + weekOffset * 7);
  const dates = Array.from({ length: 7 }, (_, i) => { const d = new Date(weekStart); d.setDate(weekStart.getDate() + i); return d; });
  const dateKey = d => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  const selectedVisits = visits.filter(v => v.date === selectedDate);
  const selectedLabel = new Date(`${selectedDate}T12:00:00`).toLocaleDateString('es-EC', { weekday: 'long', day: 'numeric', month: 'long' });
  return <><PageHeading eyebrow="ORGANIZACIÓN DE LA CLÍNICA" title="Agenda médica" subtitle="Coordina citas por día, profesional y sillón dental." action={<button className="btn btn-primary" onClick={onNew}><Plus size={17}/> Agendar cita</button>}/>
    <div className="agenda-toolbar"><div className="date-toggle"><button onClick={() => { setWeekOffset(weekOffset - 1); const d = new Date(`${selectedDate}T12:00:00`); d.setDate(d.getDate() - 7); setSelectedDate(dateKey(d)); }}>‹</button><b>{dates[0].toLocaleDateString('es-EC', { month: 'long', year: 'numeric' })}</b><button onClick={() => { setWeekOffset(weekOffset + 1); const d = new Date(`${selectedDate}T12:00:00`); d.setDate(d.getDate() + 7); setSelectedDate(dateKey(d)); }}>›</button></div><div className="view-toggle"><button className="selected">Semana</button><button onClick={() => notify('La vista diaria estará disponible próximamente')}>Día</button></div></div>
    <div className="week-strip">{dates.map((d, i) => { const value = dateKey(d); return <button key={i} onClick={() => setSelectedDate(value)} className={`week-day ${value === selectedDate ? 'chosen' : ''}`}><small>{d.toLocaleDateString('es-EC', { weekday: 'short' })}</small><b>{d.getDate()}</b><span>{visits.some(v => v.date === value) ? '•' : ''}</span></button>; })}</div>
    <div className="schedule-layout"><div className="panel day-schedule"><div className="panel-heading"><div><h2>{selectedLabel}</h2><p>{selectedVisits.length} citas · 2 sillones activos</p></div><button className="link-button" onClick={() => notify('Filtros de agenda próximamente')}>Filtrar <ChevronDown size={15}/></button></div><div className="chair-heading"><span>HORA</span><span>SILLÓN 1 · DRA. RUIZ</span><span>SILLÓN 2 · DR. PAREDES</span></div>{['08:00','09:00','10:00','11:00','12:00','13:00','14:00','15:00','16:00','17:00'].map(time => <div className="time-slot" key={time}><span className="hour-label">{time}</span>{[0, 1].map(chair => { const v = selectedVisits.find(a => a.time === time && a.chair === `Sillón ${chair + 1}`); return <div className="slot-cell" key={chair}>{v && <div className={`slot-card slot-${chair}`}><b>{v.patient}</b><span>{v.procedure}</span><small>{v.time} · {v.status}</small><select className="appointment-status" aria-label="Cambiar estado de cita" value={v.status} onChange={e => setVisits(visits.map(a => a.id === v.id ? { ...a, status: e.target.value } : a))}><option>Programada</option><option>Confirmada</option><option>En sala de espera</option><option>En atención</option><option>Atendido</option><option>Cancelada</option><option>No asistió</option></select></div>}</div>; })}</div>)}</div><aside className="side-panel"><div className="panel"><h3>Equipo de hoy</h3><div className="team-row"><Avatar name="Valeria Ruiz" initials="VR" tone="tone-1"/><div><b>Dra. Valeria Ruiz</b><small>Odontología general</small></div><i className="online"/></div><div className="team-row"><Avatar name="Daniel Paredes" initials="DP" tone="tone-2"/><div><b>Dr. Daniel Paredes</b><small>Endodoncia</small></div><i className="online"/></div><div className="divider"/><h3>Ocupación de sillones</h3><div className="occupancy"><span>Sillón 1</span><b>{selectedVisits.filter(v => v.chair === 'Sillón 1').length} citas</b></div><div className="progress"><i style={{width:`${Math.min(100, selectedVisits.filter(v => v.chair === 'Sillón 1').length * 17)}%`}}/></div><div className="occupancy"><span>Sillón 2</span><b>{selectedVisits.filter(v => v.chair === 'Sillón 2').length} citas</b></div><div className="progress blue-progress"><i style={{width:`${Math.min(100, selectedVisits.filter(v => v.chair === 'Sillón 2').length * 17)}%`}}/></div></div></aside></div>
  </>;
}

function ClinicalRecord({ backendReady, canAmend, patient, patients, onSelect, tab, setTab, toothStatus, setToothStatus, surfaceStatus, setSurfaceStatus, notes, setNotes, prescriptions, setPrescriptions, consents, setConsents, period, setPeriod, notify, onNew }) {
  const [dentition, setDentition] = useState('Permanente');
  const [notation, setNotation] = useState('FDI');
  const [noteText, setNoteText] = useState({ subjetivo: '', objetivo: '', evaluacion: '', plan: '' });
  const [activeTooth, setActiveTooth] = useState(null);
  const key = patient.id ?? patient.doc;
  const ownNotes = notes.filter(n => n.patient === key);
  const ownRx = prescriptions.filter(n => n.patient === key);
  const ownConsents = consents.filter(n => n.patient === key);
  useEffect(() => {
    if (!backendReady || !patient?.id) return;
    let active = true;
    Promise.all([api.clinicalNotes(patient.id), api.odontogram(patient.id), api.periodontogram(patient.id), api.prescriptions(patient.id), api.consents(patient.id)])
      .then(([remoteNotes, chart, periodontal, remoteRx, remoteConsents]) => {
        if (!active) return;
        setNotes([...remoteNotes, ...notes.filter(n => n.patient !== key)]);
        setPrescriptions([...remoteRx, ...prescriptions.filter(n => n.patient !== key)]);
        setConsents([...remoteConsents, ...consents.filter(n => n.patient !== key)]);
        const prefix = `${key}-`;
        const savedTeeth = Object.fromEntries(Object.entries(chart.toothStatus || {}).map(([tooth, status]) => [`${key}-${tooth}`, status]));
        const savedSurfaces = Object.fromEntries(Object.entries(chart.surfaceStatus || {}).map(([surface, value]) => [`${key}-${surface}`, value]));
        const savedPeriod = Object.fromEntries(Object.entries(periodontal.readings || {}).map(([reading, value]) => [`${key}-${reading}`, value]));
        setToothStatus({ ...Object.fromEntries(Object.entries(toothStatus).filter(([k]) => !k.startsWith(prefix))), ...savedTeeth });
        setSurfaceStatus({ ...Object.fromEntries(Object.entries(surfaceStatus).filter(([k]) => !k.startsWith(prefix))), ...savedSurfaces });
        setPeriod({ ...Object.fromEntries(Object.entries(period).filter(([k]) => !k.startsWith(prefix))), ...savedPeriod });
        setDentition(chart.dentition || 'Permanente');
        setNotation(chart.notation || 'FDI');
      }).catch(error => notify(`No se pudo cargar el expediente: ${error.message}`));
    return () => { active = false; };
  }, [backendReady, patient?.id]);
  const saveNote = async () => {
    if (!Object.values(noteText).some(x => x.trim())) return;
    try {
      const soap = { subjective: noteText.subjetivo, objective: noteText.objetivo, assessment: noteText.evaluacion, plan: noteText.plan };
      const saved = backendReady ? await api.createClinicalNote(key, soap) : { id: Date.now(), patient: key, text: Object.entries(noteText).map(([label, value]) => `${label.toUpperCase()}: ${value.trim()}`).filter(x => !x.endsWith(':')).join('\n'), date: new Date().toLocaleString('es-EC'), author: 'Usuario activo' };
      setNotes([saved, ...notes.filter(n => n.id !== saved.id)]);
      setNoteText({ subjetivo: '', objetivo: '', evaluacion: '', plan: '' });
      notify('Nota clínica registrada');
    } catch (error) { notify(`No se pudo guardar la nota: ${error.message}`); }
  };
  const addAmendment = async note => {
    const text = window.prompt('Escribe la aclaración o corrección que debe quedar vinculada a esta nota:');
    if (!text?.trim()) return;
    try {
      const saved = backendReady
        ? await api.createClinicalAmendment(key, note.id, { subjective: `ADENDA: ${text.trim()}` })
        : { id: Date.now(), patient: key, text: `ADENDA A NOTA #${note.id}: ${text.trim()}`, date: new Date().toLocaleString('es-EC'), author: 'Usuario activo' };
      setNotes([saved, ...notes]);
      notify('Adenda registrada y vinculada a la nota original');
    } catch (error) { notify(`No se pudo guardar la adenda: ${error.message}`); }
  };
  const saveOdontogram = async () => {
    try {
      if (backendReady) {
        const teeth = Object.fromEntries(Object.entries(toothStatus).filter(([k]) => k.startsWith(`${key}-`)).map(([k,v]) => [k.slice(`${key}-`.length),v]));
        const surfaces = Object.fromEntries(Object.entries(surfaceStatus).filter(([k,v]) => k.startsWith(`${key}-`) && v).map(([k,v]) => [k.slice(`${key}-`.length),v]));
        await api.saveOdontogram(key, { dentition, notation, toothStatus: teeth, surfaceStatus: surfaces });
      }
      notify('Odontograma guardado');
    } catch (error) { notify(`No se pudo guardar el odontograma: ${error.message}`); }
  };
  const savePeriodontogram = async () => {
    try {
      if (backendReady) {
        const readings = {};
        for (const [storageKey, raw] of Object.entries(period)) {
          if (!storageKey.startsWith(`${key}-`)) continue;
          const siteKey = storageKey.slice(`${key}-`.length);
          const match = siteKey.match(/^(\d+)-(v|l)-(m|c|d)$/);
          if (!match) continue;
          const reading = typeof raw === 'object' && raw !== null ? raw : { depth: raw };
          if (reading.depth === '' || reading.depth == null) continue;
          const tooth = match[1];
          const mobility = period[`${key}-${tooth}-meta-mobility`];
          const furcation = period[`${key}-${tooth}-meta-furcation`];
          readings[siteKey] = { depth: Number(reading.depth), margin: reading.margin === '' || reading.margin == null ? null : Number(reading.margin), bleeding: Boolean(reading.bleeding), mobility: mobility === '' || mobility == null ? null : Number(mobility), furcation: furcation === '' || furcation == null ? null : Number(furcation) };
        }
        await api.savePeriodontogram(key, { readings });
      }
      notify('Periodontograma guardado');
    } catch (error) { notify(`No se pudo guardar el periodontograma: ${error.message}`); }
  };
  const saveRx = () => onNew('rx');
  const front = dentition === 'Permanente' ? { upperR: [18,17,16,15,14,13,12,11], upperL: [21,22,23,24,25,26,27,28], lowerR: [48,47,46,45,44,43,42,41], lowerL: [31,32,33,34,35,36,37,38] } : { upperR: [55,54,53,52,51], upperL: [61,62,63,64,65], lowerR: [85,84,83,82,81], lowerL: [71,72,73,74,75] };
  const patientAllergy = !['','Ninguna','Ninguna conocida'].includes(patient.allergies || '');
  const clinicalAlert = patientAllergy ? patient.allergies : (patient.conditions || 'Ninguna');
  const hasClinicalAlert = !['','Ninguna','Ninguna conocida'].includes(clinicalAlert);
  return <><PageHeading eyebrow="HISTORIA CLÍNICA DIGITAL" title="Expediente clínico" subtitle="Registro longitudinal de atención odontológica." action={<button className="btn btn-outline" onClick={() => onNew('consent')}><ShieldCheck size={17}/> Nuevo consentimiento</button>}/>
    <div className="patient-banner"><Avatar name={patient.name} initials={patient.name.split(' ').slice(0,2).map(x=>x[0]).join('')} tone="tone-1"/><div className="banner-name"><b>{patient.name}</b><span>{patient.doc} <i/> {patient.age} años <i/> Grupo {patient.blood}</span></div><div className={`banner-alert ${hasClinicalAlert?'alert-priority':''}`}><AlertTriangle size={16}/><span><b>{hasClinicalAlert?'Alerta médica prioritaria':'Alerta médica'}</b>{clinicalAlert}</span></div><div className="banner-alert secondary"><HeartPulse size={16}/><span><b>Antecedente</b>{patient.conditions}</span></div><select className="patient-picker" value={patient.id} onChange={e => onSelect(patients.find(p => p.id === Number(e.target.value)))}>{patients.map(p => <option value={p.id} key={p.id}>{p.name}</option>)}</select></div>
    <div className="clinical-tabs">{['Odontograma','Periodontograma','Evolución','Recetas','Consentimientos'].map(name => <button key={name} className={tab === name ? 'selected' : ''} onClick={() => setTab(name)}>{name}{name === 'Evolución' && ownNotes.length > 0 && <small>{ownNotes.length}</small>}</button>)}</div>
    {tab === 'Odontograma' && <div className="record-grid"><div className="panel odontogram-panel"><div className="panel-heading"><div><h2>Odontograma</h2><p>Selecciona un diente y marca sus hallazgos.</p></div><div className="toolbar"><select value={dentition} onChange={e => setDentition(e.target.value)} className="select-control"><option>Permanente</option><option>Temporal</option></select><select value={notation} onChange={e => setNotation(e.target.value)} className="select-control" aria-label="Notación dental"><option>FDI</option><option>Universal</option></select></div></div><div className="dentition-toggle"><span>Tipo de dentición</span><b>{dentition}</b><span>Notación</span><b>{notation}</b></div><div className="tooth-chart"><div className="tooth-row">{front.upperR.map(n => <Tooth key={n} n={n} label={toothLabel(n, dentition, notation)} status={toothStatus[`${key}-${n}`]} active={activeTooth === n} onClick={() => { setActiveTooth(n); const k = `${key}-${n}`; const cycle = [undefined, 'caries', 'restaurado', 'endodoncia', 'implante', 'protesis-fija', 'protesis-removible', 'exodoncia', 'sellante', 'ausente']; const next = cycle[(cycle.indexOf(toothStatus[k]) + 1) % cycle.length]; setToothStatus({ ...toothStatus, [k]: next }); }}/>) }<div className="midline"/>{front.upperL.map(n => <Tooth key={n} n={n} label={toothLabel(n, dentition, notation)} status={toothStatus[`${key}-${n}`]} active={activeTooth === n} onClick={() => { setActiveTooth(n); const k = `${key}-${n}`; const cycle = [undefined, 'caries', 'restaurado', 'endodoncia', 'implante', 'protesis-fija', 'protesis-removible', 'exodoncia', 'sellante', 'ausente']; const next = cycle[(cycle.indexOf(toothStatus[k]) + 1) % cycle.length]; setToothStatus({ ...toothStatus, [k]: next }); }}/>)}</div><div className="arch-label">MAXILAR · DENTICIÓN {dentition.toUpperCase()}</div><div className="tooth-row lower-row">{front.lowerR.map(n => <Tooth key={n} n={n} label={toothLabel(n, dentition, notation)} status={toothStatus[`${key}-${n}`]} active={activeTooth === n} onClick={() => { setActiveTooth(n); const k = `${key}-${n}`; const cycle = [undefined, 'caries', 'restaurado', 'endodoncia', 'implante', 'protesis-fija', 'protesis-removible', 'exodoncia', 'sellante', 'ausente']; const next = cycle[(cycle.indexOf(toothStatus[k]) + 1) % cycle.length]; setToothStatus({ ...toothStatus, [k]: next }); }}/>) }<div className="midline"/>{front.lowerL.map(n => <Tooth key={n} n={n} label={toothLabel(n, dentition, notation)} status={toothStatus[`${key}-${n}`]} active={activeTooth === n} onClick={() => { setActiveTooth(n); const k = `${key}-${n}`; const cycle = [undefined, 'caries', 'restaurado', 'endodoncia', 'implante', 'protesis-fija', 'protesis-removible', 'exodoncia', 'sellante', 'ausente']; const next = cycle[(cycle.indexOf(toothStatus[k]) + 1) % cycle.length]; setToothStatus({ ...toothStatus, [k]: next }); }}/>)}</div><div className="arch-label">MANDÍBULA · DENTICIÓN {dentition.toUpperCase()}</div></div><div className="legend"><span><i className="legend-carious"/>Caries</span><span><i className="legend-restored"/>Restauración</span><span><i className="legend-endo"/>Endodoncia</span><span><i className="legend-missing"/>Ausente</span></div><p className="hint">Haz clic varias veces en un diente para cambiar el hallazgo. Guarda para sincronizarlo con PostgreSQL.</p></div><div className="panel tooth-detail"><div className="detail-icon"><Activity size={21}/></div><h3>{activeTooth ? `Diente ${activeTooth}` : 'Registro dental'}</h3><p>Selecciona un diente y registra las superficies clínicas.</p><div className="surface-diagram"><button className={surfaceStatus[`${key}-${activeTooth}-V`] ? 'marked' : ''} onClick={() => activeTooth && setSurfaceStatus({ ...surfaceStatus, [`${key}-${activeTooth}-V`]: !surfaceStatus[`${key}-${activeTooth}-V`] })}>V</button><button className={`surface-center ${surfaceStatus[`${key}-${activeTooth}-O`] ? 'marked' : ''}`} onClick={() => activeTooth && setSurfaceStatus({ ...surfaceStatus, [`${key}-${activeTooth}-O`]: !surfaceStatus[`${key}-${activeTooth}-O`] })}>O</button><button className={surfaceStatus[`${key}-${activeTooth}-P`] ? 'marked' : ''} onClick={() => activeTooth && setSurfaceStatus({ ...surfaceStatus, [`${key}-${activeTooth}-P`]: !surfaceStatus[`${key}-${activeTooth}-P`] })}>P</button><button className={surfaceStatus[`${key}-${activeTooth}-M`] ? 'marked' : ''} onClick={() => activeTooth && setSurfaceStatus({ ...surfaceStatus, [`${key}-${activeTooth}-M`]: !surfaceStatus[`${key}-${activeTooth}-M`] })}>M</button><button className={surfaceStatus[`${key}-${activeTooth}-D`] ? 'marked' : ''} onClick={() => activeTooth && setSurfaceStatus({ ...surfaceStatus, [`${key}-${activeTooth}-D`]: !surfaceStatus[`${key}-${activeTooth}-D`] })}>D</button></div><div className="detail-options"><span>Superficies seleccionadas</span><b>{['V','P','M','D','O'].filter(s => surfaceStatus[`${key}-${activeTooth}-${s}`]).join(' · ') || 'Ninguna'}</b></div><button className="btn btn-primary full" onClick={saveOdontogram}>Guardar odontograma</button></div></div>}
    {tab === 'Periodontograma' && <div className="panel period-panel"><div className="panel-heading"><div><h2>Periodontograma digital</h2><p>Profundidad y margen gingival en seis sitios · NIC = profundidad + margen</p></div><button className="btn btn-outline" onClick={savePeriodontogram}>Guardar cambios</button></div><div className="period-legend"><span><i className="green-dot"/> 1–3 mm · Saludable</span><span><i className="yellow-dot"/> 4–5 mm · Vigilar</span><span><i className="red-dot"/> 6+ mm · Profundidad</span></div><p className="hint">Registra el margen con signo: positivo si hay recesión y negativo si el margen está coronal a la unión cemento-esmalte.</p><div className="period-grid"><div className="period-head"><b>Diente ({notation})</b><span>Vestibular · Mesial / Medio / Distal</span><span>Lingual · Mesial / Medio / Distal</span><span>Movilidad · Furca</span></div>{[18,17,16,15,14,13,12,11,21,22,23,24,25,26,27,28,48,47,46,45,44,43,42,41,31,32,33,34,35,36,37,38].map(n => <div className="period-row" key={n}><b>{toothLabel(n,dentition,notation)}</b>{['v','l'].map(side => <div className="period-inputs" key={side}>{['m','c','d'].map(site => { const k = `${key}-${n}-${side}-${site}`; const raw=period[k]; const reading=typeof raw==='object'&&raw!==null?raw:{depth:raw??'',margin:'',bleeding:false}; const nic=Number(reading.depth||0)+Number(reading.margin||0); return <div className="period-site" key={k}><small>{site.toUpperCase()}</small><input type="number" min="0" max="15" step="0.5" aria-label={`Diente ${toothLabel(n,dentition,notation)} ${side} ${site} profundidad`} value={reading.depth??''} placeholder="Sondaje" onChange={e=>setPeriod({...period,[k]:{...reading,depth:e.target.value}})}/><input type="number" min="-15" max="15" step="0.5" aria-label={`Diente ${toothLabel(n,dentition,notation)} ${side} ${site} margen`} value={reading.margin??''} placeholder="Margen" onChange={e=>setPeriod({...period,[k]:{...reading,margin:e.target.value}})}/><label><input type="checkbox" checked={Boolean(reading.bleeding)} onChange={e=>setPeriod({...period,[k]:{...reading,bleeding:e.target.checked}})}/> Sangrado</label><small>NIC {nic.toFixed(1)} mm</small></div>; })}</div>)}<div className="period-tooth-meta"><label><span>Movilidad</span><select value={period[`${key}-${n}-meta-mobility`]||''} onChange={e=>setPeriod({...period,[`${key}-${n}-meta-mobility`]:e.target.value})}><option value="">—</option><option value="0">0</option><option value="1">I</option><option value="2">II</option><option value="3">III</option></select></label><label><span>Furca</span><select value={period[`${key}-${n}-meta-furcation`]||''} onChange={e=>setPeriod({...period,[`${key}-${n}-meta-furcation`]:e.target.value})}><option value="">—</option><option value="0">0</option><option value="1">I</option><option value="2">II</option><option value="3">III</option></select></label></div></div>)}</div><p className="hint">Las mediciones y observaciones se guardan en PostgreSQL. Si no se registra margen, el cálculo de NIC equivale a la profundidad al sondaje.</p></div>}
    {tab === 'Evolución' && <div className="record-grid"><div className="panel notes-panel"><div className="panel-heading"><div><h2>Notas de evolución (SOAP)</h2><p>Notas firmadas en el expediente del paciente</p></div><span className="locked-label"><ShieldCheck size={14}/> Registro inmutable</span></div><div className="note-compose"><label>Nueva nota clínica</label><div className="soap-grid">{[['subjetivo','Subjetivo'],['objetivo','Objetivo'],['evaluacion','Evaluación'],['plan','Plan']].map(([field,label]) => <textarea key={field} aria-label={label} value={noteText[field]} onChange={e => setNoteText({ ...noteText, [field]: e.target.value })} placeholder={`${label}...`}/>)}</div><div><small>Al guardar, la nota queda bloqueada y firmada por el usuario activo.</small><button className="btn btn-primary" onClick={saveNote}>Guardar nota firmada</button></div></div><div className="timeline">{ownNotes.length ? ownNotes.map(n => <div className="timeline-item" key={n.id}><i/><small>{n.date} · {n.author}</small><p>{n.text}</p>{canAmend&&<button className="btn btn-outline" onClick={()=>addAmendment(n)}>Crear adenda</button>}</div>) : <Empty title="Sin notas clínicas" text="Las notas de evolución aparecerán aquí en orden cronológico."/>}</div></div><div className="panel side-info"><h3>Registro clínico</h3><p>El historial de atención no se elimina. Las correcciones deben registrarse como una nueva adenda vinculada a la nota original.</p><div className="divider"/><span className="side-meta">PACIENTE</span><b>{patient.name}</b><span className="side-meta">NOTAS REGISTRADAS</span><b>{ownNotes.length}</b></div></div>}
    {tab === 'Recetas' && <div className="panel rx-panel"><div className="panel-heading"><div><h2>Recetas odontológicas</h2><p>Prescripciones asociadas a este expediente</p></div><button className="btn btn-primary" onClick={saveRx}><Plus size={16}/> Nueva receta</button></div>{ownRx.length ? ownRx.map(rx => <div className="rx-card" key={rx.id}><div className="rx-symbol">℞</div><div><b>{rx.med}</b><span>{rx.dose} · {rx.duration}</span><small>{rx.date} · Dra. Valeria Ruiz</small></div><Status label="Emitida"/></div>) : <Empty title="Aún no hay recetas" text="Crea una receta para que quede registrada en el expediente del paciente."/>}</div>}
    {tab === 'Consentimientos' && <div className="panel consent-panel"><div className="panel-heading"><div><h2>Consentimientos informados</h2><p>Autorizaciones de procedimientos clínicos</p></div><button className="btn btn-primary" onClick={() => onNew('consent')}><Plus size={16}/> Generar consentimiento</button></div>{ownConsents.length ? ownConsents.map(c => <div className="consent-card" key={c.id}><div className="consent-icon"><ShieldCheck size={18}/></div><div><b>{c.procedure}</b><span>Creado el {c.date}</span></div><Status label={c.state}/><button className="link-button" onClick={() => notify('Flujo de firma digital pendiente de integración')}>Ver documento →</button></div>) : <Empty title="Sin consentimientos registrados" text="Genera el consentimiento del procedimiento antes de la atención clínica."/>}<div className="consent-foot"><ShieldCheck size={17}/> La firma biométrica/OTP requiere integración con un proveedor autorizado.</div></div>}
  </>;
}
function Tooth({ n, label=n, status, active, onClick }) { return <button title={`Diente ${label}${status ? ` · ${status}` : ''}`} onClick={onClick} className={`tooth ${status || ''} ${active ? 'tooth-selected' : ''}`}><div className="tooth-face"><i/><i/><i/><i/><b/></div><small>{label}</small></button>; }
function Empty({ title, text }) { return <div className="empty-state"><div className="empty-icon"><FileHeart size={21}/></div><b>{title}</b><p>{text}</p></div>; }

function TreatmentPlans({ backendReady, patients, canCreate, canApprove, notify }) {
  const emptyItem = () => ({ procedure:'', phase:1, sessions:1, fee:'', labCost:0, materialsCost:0 });
  const [plans,setPlans]=useState([]);const [form,setForm]=useState({patientId:'',title:'',discount:0,items:[emptyItem()]});const [selectedItems,setSelectedItems]=useState({});const [busy,setBusy]=useState(false);
  const refresh=async()=>{if(!backendReady)return;try{setPlans(await api.treatmentPlans())}catch(e){notify(`No se pudieron cargar los planes: ${e.message}`)}};
  useEffect(()=>{refresh()},[backendReady]);
  const changeItem=(index,field,value)=>setForm({...form,items:form.items.map((item,i)=>i===index?{...item,[field]:value}:item)});
  const save=async e=>{e.preventDefault();setBusy(true);try{await api.createTreatmentPlan({...form,patientId:Number(form.patientId),discount:Number(form.discount),items:form.items.map(item=>({...item,phase:Number(item.phase),sessions:Number(item.sessions),fee:Number(item.fee),labCost:Number(item.labCost),materialsCost:Number(item.materialsCost)}))});setForm({patientId:'',title:'',discount:0,items:[emptyItem()]});await refresh();notify('Presupuesto guardado y presentado al paciente')}catch(error){notify(`No se pudo guardar el presupuesto: ${error.message}`)}finally{setBusy(false)}};
  const approve=async plan=>{const ids=plan.items.filter(item=>selectedItems[item.id]&&!item.approved&&!item.completed).map(item=>item.id);if(!ids.length){notify('Selecciona las partidas que autorizó el paciente');return}try{await api.approveTreatmentPlan(plan.id,ids);await refresh();setSelectedItems({});notify('Partidas aprobadas; ya se pueden agendar')}catch(error){notify(`No se pudo aprobar: ${error.message}`)}};
  const currency=value=>new Intl.NumberFormat('es-EC',{style:'currency',currency:'USD'}).format(Number(value||0));
  return <><PageHeading eyebrow="FASE 1 · PLANIFICACIÓN" title="Planes de tratamiento" subtitle="Arma presupuestos por fases y registra qué procedimientos autoriza el paciente."/>
    {canCreate&&<div className="panel inventory-form report-panel"><div className="panel-heading"><div><h2>Nuevo plan y presupuesto</h2><p>El subtotal incluye honorarios, laboratorio y materiales; el descuento se resta al total.</p></div></div><form className="plan-form" onSubmit={save}><div className="form-grid"><label className="field"><span>Paciente</span><select required value={form.patientId} onChange={e=>setForm({...form,patientId:e.target.value})}><option value="">Selecciona paciente</option>{patients.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select></label><Field label="Título del plan" required value={form.title} onChange={e=>setForm({...form,title:e.target.value})} placeholder="Ej. Rehabilitación oral"/><Field label="Descuento autorizado (USD)" type="number" min="0" step="0.01" value={form.discount} onChange={e=>setForm({...form,discount:e.target.value})}/></div>
      {form.items.map((item,index)=><div className="plan-item-form" key={index}><b>Fase / procedimiento {index+1}</b><div className="form-grid"><Field label="Procedimiento" required value={item.procedure} onChange={e=>changeItem(index,'procedure',e.target.value)} placeholder="Ej. Profilaxis y control"/><Field label="Fase" type="number" min="1" required value={item.phase} onChange={e=>changeItem(index,'phase',e.target.value)}/><Field label="Sesiones" type="number" min="1" required value={item.sessions} onChange={e=>changeItem(index,'sessions',e.target.value)}/><Field label="Honorario (USD)" type="number" min="0" step="0.01" required value={item.fee} onChange={e=>changeItem(index,'fee',e.target.value)}/><Field label="Costo laboratorio (USD)" type="number" min="0" step="0.01" value={item.labCost} onChange={e=>changeItem(index,'labCost',e.target.value)}/><Field label="Costo materiales (USD)" type="number" min="0" step="0.01" value={item.materialsCost} onChange={e=>changeItem(index,'materialsCost',e.target.value)}/></div>{form.items.length>1&&<button type="button" className="btn btn-outline" onClick={()=>setForm({...form,items:form.items.filter((_,i)=>i!==index)})}>Quitar fase</button>}</div>)}
      <div className="plan-form-actions"><button type="button" className="btn btn-outline" disabled={form.items.length>=30} onClick={()=>setForm({...form,items:[...form.items,emptyItem()]})}><Plus size={15}/> Agregar procedimiento</button><button className="btn btn-primary" disabled={busy||!backendReady}>{busy?'Guardando…':'Guardar presupuesto'}</button></div>
    </form></div>}
    <div className="plan-list">{plans.map(plan=><div className="panel plan-card" key={plan.id}><div className="panel-heading"><div><h2>{plan.title}</h2><p>{plan.patient} · {plan.status.replaceAll('_',' ')}</p></div><b>{currency(plan.total)}</b></div><div className="table-scroll"><table><thead><tr><th>FASE</th><th>PROCEDIMIENTO</th><th>SESIONES</th><th>HONORARIOS + COSTOS</th><th>ESTADO</th></tr></thead><tbody>{plan.items.map(item=><tr key={item.id}><td>{item.phase}</td><td>{item.procedure}</td><td>{item.sessions}</td><td>{currency(Number(item.fee)+Number(item.labCost)+Number(item.materialsCost))}</td><td>{item.completed?'Completado':item.approved?'Aprobado · listo para agendar':canApprove?<label className="plan-approval"><input type="checkbox" checked={Boolean(selectedItems[item.id])} onChange={e=>setSelectedItems({...selectedItems,[item.id]:e.target.checked})}/> Seleccionar para aprobar</label>:'Presentado'}</td></tr>)}</tbody></table></div>{canApprove&&plan.items.some(item=>!item.approved&&!item.completed)&&<button className="btn btn-primary" onClick={()=>approve(plan)}>Registrar aprobación parcial o total</button>}</div>)}{!plans.length&&<Empty title="Sin planes registrados" text={canCreate?'Crea un presupuesto por fases para comenzar.':'Los planes presentados aparecerán aquí.'}/>}</div>
    {!backendReady&&<p className="hint">Conecta la API para guardar presupuestos y consultar su estado en PostgreSQL.</p>}
  </>;
}

function Billing({ backendReady, canCloseCash, hideTillSummary, invoices, money, onNew, notify }) {
  const due = invoices.reduce((s, i) => s + i.total - i.paid, 0); const paidToday = invoices.filter(i => i.date === today).reduce((s, i) => s + i.paid, 0);
  const [counted,setCounted]=useState('');const [closure,setClosure]=useState(null);const [closing,setClosing]=useState(false);
  const close=async e=>{e.preventDefault();setClosing(true);try{const result=await api.closeCash(Number(counted));setClosure(result);setCounted('');notify('Cierre de caja registrado')}catch(error){notify(`No se pudo cerrar caja: ${error.message}`)}finally{setClosing(false)}};
  return <><PageHeading eyebrow="ADMINISTRACIÓN FINANCIERA" title="Caja y facturación" subtitle="Registra cobros y consulta el estado de cuenta de los pacientes." action={<button className="btn btn-primary" onClick={onNew}><Plus size={17}/> Registrar cobro</button>}/><div className="stat-grid billing-stats">{!hideTillSummary&&<Stat icon={CircleDollarSign} label="Ingresos hoy" value={money(paidToday)} detail="Caja principal" tone="teal"/>}<Stat icon={ClipboardList} label="Cobros registrados" value={invoices.length.toString()} detail="En el periodo" tone="blue"/><Stat icon={Activity} label="Saldo por cobrar" value={money(due)} detail="Tratamientos con abono" tone="peach"/></div>
    {canCloseCash&&<div className="panel cash-close-panel"><div className="panel-heading"><div><h2>Cierre ciego de caja</h2><p>Cuenta primero el efectivo físico. El total teórico aparecerá después de registrar tu conteo.</p></div></div><form onSubmit={close} className="cash-close-form"><label className="field"><span>Efectivo contado (USD)</span><input type="number" min="0" step="0.01" required value={counted} onChange={e=>setCounted(e.target.value)} placeholder="Ingresa el conteo físico"/></label><button className="btn btn-primary" disabled={!backendReady||closing}>{closing?'Registrando…':'Cerrar caja'}</button></form>{closure&&<div className="cash-result"><b>Cierre registrado</b><span>Efectivo contado: {money(closure.declaredCash)}</span><span>Efectivo esperado: {money(closure.expectedCash)}</span><span>Diferencia: {money(closure.difference)}</span><small>Total cobrado en el periodo: {money(closure.totalCollected)} · {closure.paymentCount} pagos</small></div>}</div>}
    <div className="panel table-panel"><div className="panel-heading"><div><h2>Comprobantes recientes</h2><p>Control de pagos y saldos pendientes</p></div><button className="btn btn-outline">Este mes <ChevronDown size={15}/></button></div><div className="table-scroll"><table><thead><tr><th>N.º COMPROBANTE</th><th>PACIENTE</th><th>CONCEPTO</th><th>FECHA</th><th>TOTAL</th><th>SALDO</th><th>ESTADO</th></tr></thead><tbody>{invoices.map(i => <tr key={i.id}><td><b>{i.id}</b></td><td>{i.patient}</td><td>{i.concept}</td><td>{i.date}</td><td>{money(i.total)}</td><td>{money(i.total - i.paid)}</td><td><Status label={i.state}/></td></tr>)}</tbody></table></div></div></>;
}
function Inventory({ backendReady, canWrite, canManage, notify }) {
  const [products, setProducts] = useState([]);
  const [movements, setMovements] = useState([]);
  const [rules, setRules] = useState([]);
  const [busy, setBusy] = useState(false);
  const [product, setProduct] = useState({ sku: '', name: '', category: 'Insumos', unit: 'unidad', minimumStock: 0, initialStock: 0 });
  const [movement, setMovement] = useState({ productId: '', type: 'receipt', quantity: '', reference: '', notes: '' });
  const [rule, setRule] = useState({ procedure: '', productId: '', quantity: '' });
  const refresh = async () => {
    if (!backendReady) return;
    try { const [items, history, consumptionRules] = await Promise.all([api.inventoryProducts(), api.inventoryMovements(), canManage ? api.inventoryRules() : Promise.resolve([])]); setProducts(items); setMovements(history); setRules(consumptionRules); }
    catch (error) { notify(`No se pudo cargar inventario: ${error.message}`); }
  };
  useEffect(() => { refresh(); }, [backendReady, canManage]);
  const saveProduct = async event => {
    event.preventDefault(); setBusy(true);
    try { await api.createInventoryProduct({ ...product, minimumStock: Number(product.minimumStock), initialStock: Number(product.initialStock) }); setProduct({ sku: '', name: '', category: 'Insumos', unit: 'unidad', minimumStock: 0, initialStock: 0 }); await refresh(); notify('Insumo registrado'); }
    catch (error) { notify(`No se pudo registrar el insumo: ${error.message}`); }
    finally { setBusy(false); }
  };
  const saveMovement = async event => {
    event.preventDefault(); setBusy(true);
    try { await api.createInventoryMovement({ ...movement, productId: Number(movement.productId), quantity: Number(movement.quantity) }); setMovement({ ...movement, quantity: '', reference: '', notes: '' }); await refresh(); notify(movement.type === 'receipt' ? 'Entrada registrada' : 'Consumo registrado'); }
    catch (error) { notify(`No se pudo registrar el movimiento: ${error.message}`); }
    finally { setBusy(false); }
  };
  const saveRule = async event => {
    event.preventDefault(); setBusy(true);
    try { await api.createInventoryRule({ ...rule, productId: Number(rule.productId), quantity: Number(rule.quantity) }); setRule({ procedure: '', productId: '', quantity: '' }); await refresh(); notify('Regla de descargo automático guardada'); }
    catch (error) { notify(`No se pudo guardar la regla: ${error.message}`); }
    finally { setBusy(false); }
  };
  const removeRule = async id => { try { await api.deleteInventoryRule(id); await refresh(); notify('Regla desactivada'); } catch (error) { notify(error.message); } };
  const lowCount = products.filter(p => p.lowStock).length;
  const totalUnits = products.reduce((sum,p) => sum + Number(p.stock), 0);
  const categories = new Set(products.map(p => p.category)).size;
  return <>
    <PageHeading eyebrow="FASE 3 · ADMINISTRACIÓN" title="Inventario clínico" subtitle="Controla insumos odontológicos, existencias y movimientos."/>
    <div className="stat-grid billing-stats"><Stat icon={ClipboardList} label="Insumos activos" value={String(products.length)} detail="Productos registrados" tone="teal"/><Stat icon={Activity} label="Unidades disponibles" value={new Intl.NumberFormat('es-EC',{maximumFractionDigits:2}).format(totalUnits)} detail={`${categories} categorías`} tone="blue"/><Stat icon={AlertTriangle} label="Stock bajo" value={String(lowCount)} detail="En mínimo o por debajo" tone="peach"/></div>
    <div className="inventory-layout">
      {canManage && <div className="panel inventory-form"><div className="panel-heading"><div><h2>Registrar insumo</h2><p>Agrega un producto al catálogo y define su nivel mínimo.</p></div></div><form onSubmit={saveProduct} className="form-grid"><Field label="Código / SKU" name="sku" required value={product.sku} onChange={e=>setProduct({...product,sku:e.target.value})}/><Field label="Nombre del insumo" name="name" required value={product.name} onChange={e=>setProduct({...product,name:e.target.value})}/><Field label="Categoría" name="category" required value={product.category} onChange={e=>setProduct({...product,category:e.target.value})}/><label className="field"><span>Unidad</span><select value={product.unit} onChange={e=>setProduct({...product,unit:e.target.value})}><option value="unidad">Unidad</option><option value="caja">Caja</option><option value="paquete">Paquete</option><option value="ml">Mililitro</option><option value="g">Gramo</option><option value="par">Par</option></select></label><Field label="Stock mínimo" name="minimumStock" type="number" step="0.01" required value={product.minimumStock} onChange={e=>setProduct({...product,minimumStock:e.target.value})}/><Field label="Existencias iniciales" name="initialStock" type="number" step="0.01" required value={product.initialStock} onChange={e=>setProduct({...product,initialStock:e.target.value})}/><button className="btn btn-primary span-two" disabled={busy}>Guardar insumo</button></form></div>}
      {canWrite && <div className="panel inventory-form"><div className="panel-heading"><div><h2>Registrar movimiento</h2><p>Registra una entrada o el consumo de un insumo.</p></div></div><form onSubmit={saveMovement} className="form-grid"><label className="field span-two"><span>Insumo</span><select required value={movement.productId} onChange={e=>setMovement({...movement,productId:e.target.value})}><option value="">Selecciona un insumo</option>{products.map(p=><option value={p.id} key={p.id}>{p.name} · stock {p.stock} {p.unit}</option>)}</select></label><label className="field"><span>Tipo de movimiento</span><select value={movement.type} onChange={e=>setMovement({...movement,type:e.target.value})}><option value="receipt">Entrada</option><option value="consumption">Consumo</option></select></label><Field label="Cantidad" name="quantity" type="number" step="0.01" required value={movement.quantity} onChange={e=>setMovement({...movement,quantity:e.target.value})}/><Field label="Referencia" name="reference" value={movement.reference} placeholder="Factura, área o procedimiento" onChange={e=>setMovement({...movement,reference:e.target.value})}/><Field label="Nota" name="notes" value={movement.notes} onChange={e=>setMovement({...movement,notes:e.target.value})}/><button className="btn btn-primary span-two" disabled={busy || !products.length}>Guardar movimiento</button></form></div>}
    </div>
    <div className="panel table-panel inventory-table"><div className="panel-heading"><div><h2>Existencias</h2><p>El stock se calcula a partir del historial de movimientos.</p></div><button className="btn btn-outline" onClick={refresh}>Actualizar</button></div><div className="table-scroll"><table><thead><tr><th>CÓDIGO</th><th>INSUMO</th><th>CATEGORÍA</th><th>STOCK ACTUAL</th><th>MÍNIMO</th><th>ESTADO</th></tr></thead><tbody>{products.map(p=><tr key={p.id}><td><b>{p.sku}</b></td><td>{p.name}</td><td>{p.category}</td><td>{p.stock} {p.unit}</td><td>{p.minimumStock} {p.unit}</td><td><Status label={p.lowStock?'Stock bajo':'Disponible'}/></td></tr>)}</tbody></table>{!products.length && <Empty title="Sin insumos registrados" text="Registra los materiales de la clínica para empezar a controlar las existencias."/>}</div></div>
    <div className="panel table-panel inventory-table"><div className="panel-heading"><div><h2>Movimientos recientes</h2><p>Últimas entradas y consumos registrados.</p></div></div><div className="table-scroll"><table><thead><tr><th>FECHA</th><th>INSUMO</th><th>MOVIMIENTO</th><th>CANTIDAD</th><th>REFERENCIA</th><th>REGISTRADO POR</th></tr></thead><tbody>{movements.map(m=><tr key={m.id}><td>{m.date}</td><td>{m.product}</td><td>{m.type==='receipt'?'Entrada':m.type==='consumption'?'Consumo':'Ajuste inicial'}</td><td>{m.type==='consumption'?'−':'+'}{m.quantity}</td><td>{m.reference||'—'}</td><td>{m.author}</td></tr>)}</tbody></table>{!movements.length && <Empty title="Sin movimientos" text="Las entradas y consumos se mostrarán en este historial."/>}</div></div>
    {canManage && <div className="panel inventory-form"><div className="panel-heading"><div><h2>Descargo automático por procedimiento</h2><p>Al marcar una cita como Atendido, se descontarán estos insumos automáticamente. Usa exactamente el mismo nombre de Agenda.</p></div></div><form onSubmit={saveRule} className="form-grid"><Field label="Nombre exacto del procedimiento" name="procedure" required value={rule.procedure} placeholder="Ej. Control y profilaxis" onChange={e=>setRule({...rule,procedure:e.target.value})}/><label className="field"><span>Insumo</span><select required value={rule.productId} onChange={e=>setRule({...rule,productId:e.target.value})}><option value="">Selecciona un insumo</option>{products.map(p=><option value={p.id} key={p.id}>{p.name} ({p.unit})</option>)}</select></label><Field label="Cantidad por procedimiento" name="quantity" type="number" step="0.01" required value={rule.quantity} onChange={e=>setRule({...rule,quantity:e.target.value})}/><button className="btn btn-primary" disabled={busy || !products.length}>Guardar regla</button></form>{rules.length>0 && <div className="rule-list">{rules.map(r=><div className="rule-row" key={r.id}><span><b>{r.procedure}</b><small>{r.product} · {r.quantity} {r.unit}</small></span><button className="btn btn-outline" onClick={()=>removeRule(r.id)}>Desactivar</button></div>)}</div>}</div>}
    {!backendReady && <p className="hint">Conecta la API para cargar y guardar el inventario en PostgreSQL.</p>}
  </>;
}

function Reports({ backendReady, canManage, notify }) {
  const dateInput = value => `${value.getFullYear()}-${String(value.getMonth()+1).padStart(2,'0')}-${String(value.getDate()).padStart(2,'0')}`;
  const [from, setFrom] = useState(() => dateInput(new Date(new Date().getFullYear(),new Date().getMonth(),1)));
  const [to, setTo] = useState(() => dateInput(new Date()));
  const [report, setReport] = useState(null);
  const [providers, setProviders] = useState([]);
  const [rules, setRules] = useState([]);
  const [form, setForm] = useState({ providerId:'', procedure:'*', type:'percentage', value:'30', fee:'', labCost:'0', materialsCost:'0' });
  const [busy, setBusy] = useState(false);
  const refresh = async () => {
    if (!backendReady) return;
    try { const [summary, commissionData] = await Promise.all([api.reportSummary(from,to),api.commissionRules()]); setReport(summary); setProviders(commissionData.providers); setRules(commissionData.rules); }
    catch (error) { notify(`No se pudieron cargar los reportes: ${error.message}`); }
  };
  useEffect(() => { refresh(); }, [backendReady,from,to]);
  const saveRule = async e => {
    e.preventDefault(); setBusy(true);
    try { await api.createCommissionRule({ ...form, providerId:Number(form.providerId), value:Number(form.value), fee:Number(form.fee), labCost:Number(form.labCost), materialsCost:Number(form.materialsCost) }); await refresh(); notify('Regla de honorarios guardada'); }
    catch (error) { notify(`No se pudo guardar la comisión: ${error.message}`); }
    finally { setBusy(false); }
  };
  const removeRule = async id => { try { await api.deleteCommissionRule(id); await refresh(); notify('Regla de honorarios desactivada'); } catch (error) { notify(error.message); } };
  const maxCommission=Math.max(1,...(report?.commissions||[]).map(c=>Number(c.amount)));
  const moneyValue=value=>new Intl.NumberFormat('es-EC',{style:'currency',currency:'USD'}).format(Number(value||0));
  return <>
    <PageHeading eyebrow="FASE 3 · FINANZAS AVANZADAS" title="Reportes y honorarios" subtitle="Revisa actividad, cobros, inventario crítico y comisiones del período."/>
    <div className="report-filters"><label className="field"><span>Desde</span><input type="date" value={from} onChange={e=>setFrom(e.target.value)}/></label><label className="field"><span>Hasta</span><input type="date" value={to} onChange={e=>setTo(e.target.value)}/></label><button className="btn btn-outline" onClick={refresh}>Actualizar reporte</button></div>
    {report && <><div className="stat-grid billing-stats"><Stat icon={CalendarDays} label="Citas del período" value={String(report.appointments.total)} detail={`${report.appointments.attended} atendidas · ${report.appointments.noShow} ausentes`} tone="blue"/><Stat icon={CircleDollarSign} label="Cobrado" value={moneyValue(report.collected)} detail={`Por cobrar ${moneyValue(report.receivable)}`} tone="teal"/><Stat icon={AlertTriangle} label="Insumos bajo mínimo" value={String(report.lowStock)} detail="Requieren reposición" tone="peach"/></div>
      <div className="panel report-panel"><div className="panel-heading"><div><h2>Liquidación estimada de honorarios</h2><p>Calculada sobre procedimientos atendidos y las reglas configuradas.</p></div><b>{moneyValue(report.commissionTotal)}</b></div>{report.commissions.length ? report.commissions.map(c=><div className="commission-bar" key={c.provider}><div><b>{c.provider}</b><span>{c.procedures} procedimientos · {moneyValue(c.amount)}</span></div><i><span style={{width:`${Math.max(4,Number(c.amount)/maxCommission*100)}%`}}/></i></div>) : <Empty title="Sin comisiones calculadas" text="Agrega reglas por profesional para generar la liquidación estimada."/>}</div>
      <div className="panel table-panel report-panel"><div className="panel-heading"><div><h2>Resumen de agenda</h2><p>Estados de turnos en el rango seleccionado.</p></div></div><div className="report-status-grid"><div><small>Atendidas</small><b>{report.appointments.attended}</b></div><div><small>Canceladas</small><b>{report.appointments.cancelled}</b></div><div><small>No asistió</small><b>{report.appointments.noShow}</b></div><div><small>Total</small><b>{report.appointments.total}</b></div></div></div>
    </>}
    {canManage && <div className="panel inventory-form report-panel"><div className="panel-heading"><div><h2>Configurar honorarios</h2><p>La regla específica por procedimiento prevalece sobre la regla general (*).</p></div></div><form onSubmit={saveRule} className="form-grid"><label className="field span-two"><span>Odontólogo</span><select required value={form.providerId} onChange={e=>setForm({...form,providerId:e.target.value})}><option value="">Selecciona un profesional</option>{providers.map(p=><option value={p.id} key={p.id}>{p.name}</option>)}</select></label><Field label="Procedimiento (* para todos)" name="procedure" value={form.procedure} onChange={e=>setForm({...form,procedure:e.target.value})}/><label className="field"><span>Tipo de cálculo</span><select value={form.type} onChange={e=>setForm({...form,type:e.target.value})}><option value="percentage">Porcentaje del honorario</option><option value="fixed">Monto fijo por procedimiento</option><option value="net_percentage">Porcentaje después de costos</option></select></label><Field label={form.type==='fixed'?'Monto fijo (USD)':'Porcentaje'} name="value" type="number" step="0.01" required value={form.value} onChange={e=>setForm({...form,value:e.target.value})}/>{form.type!=='fixed' && <Field label="Honorario del procedimiento (USD)" name="fee" type="number" step="0.01" required value={form.fee} onChange={e=>setForm({...form,fee:e.target.value})}/>}{form.type==='net_percentage' && <><Field label="Costo laboratorio (USD)" name="labCost" type="number" step="0.01" value={form.labCost} onChange={e=>setForm({...form,labCost:e.target.value})}/><Field label="Costo materiales (USD)" name="materialsCost" type="number" step="0.01" value={form.materialsCost} onChange={e=>setForm({...form,materialsCost:e.target.value})}/></>}<button className="btn btn-primary span-two" disabled={busy || !providers.length}>Guardar regla</button></form>{rules.length>0 && <div className="rule-list">{rules.map(rule=><div className="rule-row" key={rule.id}><span><b>{rule.provider} · {rule.procedure==='*'?'Todos los procedimientos':rule.procedure}</b><small>{rule.type==='fixed'?moneyValue(rule.value):`${rule.value}%`} · base ${moneyValue(rule.fee)}</small></span>{canManage && <button className="btn btn-outline" onClick={()=>removeRule(rule.id)}>Desactivar</button>}</div>)}</div>}</div>}
    {!backendReady && <p className="hint">Conecta la API para cargar los reportes desde PostgreSQL.</p>}
  </>;
}

function Modal({ backendReady, kind, patients, visits, patient, onCreatePatient, onUpdatePatient, onCreateVisit, onCreateInvoice, onSaveRx, onSaveConsent, setModal, notify }) {
  const [form, setForm] = useState({ date: today, time: '14:00', chair: 'Sillón 1', doctor: 'Dra. Valeria Ruiz', patient: patient?.name || patients[0]?.name || '', status: 'Programada', ...(kind === 'editPatient' ? { ...patient, age: Number(patient?.age || 0), emergency: patient?.emergency || '', guardian: patient?.guardian || '' } : { age: '', blood: '', allergies: '', conditions: '', phone: '', email: '', doc: '', insurance: 'Particular' }), planItemId: 0, concept: '', total: '', paid: '', procedure: '', med: '', dose: '', duration: '' });
  const [approvedTasks,setApprovedTasks]=useState([]);
  useEffect(()=>{if(kind!=='visit'||!backendReady)return;let active=true;api.treatmentPlans().then(plans=>{if(active)setApprovedTasks(plans.flatMap(plan=>plan.items.filter(item=>item.approved&&!item.completed&&item.availableSessions>0).map(item=>({...item,planId:plan.id,planTitle:plan.title,patient:plan.patient,patientId:plan.patientId}))))}).catch(()=>{});return()=>{active=false}},[kind,backendReady]);
  const [formError, setFormError] = useState('');
  const change = e => setForm({ ...form, [e.target.name]: e.target.type === 'number' ? Number(e.target.value) : e.target.value });
  const close = () => setModal('');
  const title = { patient: 'Registrar paciente', editPatient: 'Editar paciente', visit: 'Agendar cita', invoice: 'Registrar cobro', rx: 'Nueva receta', consent: 'Generar consentimiento' }[kind];
  const submit = async e => {
    e.preventDefault();
    setFormError('');
    try {
      if (kind === 'patient') { await onCreatePatient(form); notify('Ficha de paciente guardada'); }
      if (kind === 'editPatient') { await onUpdatePatient(form); notify('Ficha del paciente actualizada'); }
      if (kind === 'visit') {
        const collision = visits.some(v => v.date === form.date && v.time === form.time && v.chair === form.chair);
        if (collision) { setFormError('Ese sillón ya tiene una cita en ese horario.'); return; }
        await onCreateVisit(form); notify('Cita guardada');
      }
      if (kind === 'invoice') { const total = Number(form.total), paid = Number(form.paid); if (paid > total) { setFormError('El abono no puede superar el total.'); return; } await onCreateInvoice({ ...form, total, paid }); notify('Cobro guardado'); }
      if (kind === 'rx') { await onSaveRx(form); notify('Receta creada'); }
      if (kind === 'consent') { await onSaveConsent(form); notify('Consentimiento preparado para firma'); }
      close();
    } catch (error) {
      setFormError(error.message || 'No se pudo guardar la información.');
    }
  };
  return <div className="modal-backdrop" onMouseDown={e => e.target === e.currentTarget && close()}><form className="modal" onSubmit={submit}><div className="modal-heading"><div><div className="eyebrow">SIGO DENTAL</div><h2>{title}</h2></div><button type="button" className="icon-button" onClick={close}><X size={20}/></button></div>
    {(kind === 'patient' || kind === 'editPatient') && <><div className="form-grid"><Field label="Nombres y apellidos" name="name" required value={form.name || ''} onChange={change}/><Field label="Cédula / identificación" name="doc" required value={form.doc || ''} onChange={change}/><Field label="Fecha de nacimiento" name="birth" type="date" value={form.birth || ''} onChange={change}/><Field label="Edad" name="age" type="number" required value={form.age ?? ''} onChange={change}/><Field label="Teléfono" name="phone" value={form.phone || ''} onChange={change}/><Field label="Teléfono de emergencia" name="emergency" value={form.emergency || ''} onChange={change}/><Field label="Correo electrónico" name="email" type="email" value={form.email || ''} onChange={change}/><label className="field"><span>Grupo sanguíneo</span><select name="blood" value={form.blood || ''} onChange={change}><option value="">No registrado</option>{['O+','O-','A+','A-','B+','B-','AB+','AB-'].map(x=><option key={x}>{x}</option>)}</select></label><label className="field"><span>Seguro / convenio</span><select name="insurance" value={form.insurance || 'Particular'} onChange={change}><option>Particular</option><option>IESS</option><option>Saludsa</option><option>Otro</option></select></label><Field label="Tutor legal (si es menor de edad)" name="guardian" value={form.guardian || ''} onChange={change}/><Field label="Alergias" name="allergies" value={form.allergies || ''} placeholder="Escribe 'Ninguna conocida' si aplica" onChange={change}/><Field label="Antecedentes / condiciones" name="conditions" value={form.conditions || ''} placeholder="Enfermedades, medicación, cirugías..." onChange={change}/></div><div className="form-warning"><AlertTriangle size={16}/> Verifica los antecedentes antes de iniciar cada procedimiento.</div></>}
    {kind === 'visit' && <div className="form-grid"><label className="field span-two"><span>Tarea de un presupuesto aprobado (opcional)</span><select value={form.planItemId} onChange={e=>{const task=approvedTasks.find(t=>String(t.id)===e.target.value);setForm({...form,planItemId:task?task.id:0,patient:task?.patient||form.patient,procedure:task?.procedure||form.procedure})}}><option value="0">Cita independiente</option>{approvedTasks.map(task=><option value={task.id} key={task.id}>{task.patient} · {task.procedure} · {task.planTitle}</option>)}</select></label><label className="field span-two"><span>Paciente</span><select name="patient" value={form.patient} onChange={change}>{patients.map(p=><option key={p.id}>{p.name}</option>)}</select></label><label className="field"><span>Fecha</span><input type="date" name="date" value={form.date} onChange={change}/></label><label className="field"><span>Hora</span><input type="time" name="time" value={form.time} onChange={change}/></label><label className="field"><span>Profesional</span><select name="doctor" value={form.doctor} onChange={change}><option>Dra. Valeria Ruiz</option><option>Dr. Daniel Paredes</option></select></label><label className="field"><span>Sillón</span><select name="chair" value={form.chair} onChange={change}><option>Sillón 1</option><option>Sillón 2</option></select></label><Field label="Motivo / procedimiento" name="procedure" required value={form.procedure} onChange={change}/></div>}
    {kind === 'invoice' && <div className="form-grid"><label className="field span-two"><span>Paciente</span><select name="patient" onChange={change}>{patients.map(p=><option key={p.id}>{p.name}</option>)}</select></label><Field label="Concepto" name="concept" required onChange={change}/><Field label="Total (USD)" name="total" type="number" step="0.01" required onChange={change}/><Field label="Monto recibido (USD)" name="paid" type="number" step="0.01" required onChange={change}/><label className="field"><span>Método de pago</span><select name="method" onChange={change}><option>Efectivo</option><option>Tarjeta</option><option>Transferencia</option><option>Seguro / convenio</option></select></label></div>}
    {kind === 'rx' && <div className="form-grid"><label className="field span-two"><span>Paciente</span><select name="patient" onChange={change}>{patients.map(p=><option key={p.id}>{p.name}</option>)}</select></label><Field label="Medicamento" name="med" required onChange={change}/><Field label="Dosis e indicaciones" name="dose" required onChange={change}/><Field label="Duración" name="duration" required onChange={change}/></div>}
    {kind === 'consent' && <><div className="form-grid"><Field label="Procedimiento" name="procedure" required onChange={change}/><label className="field"><span>Paciente</span><select name="patient" onChange={change}>{patients.map(p=><option key={p.id}>{p.name}</option>)}</select></label></div><div className="form-warning"><ShieldCheck size={16}/> Este prototipo no genera firma legal. La integración de firma digital/OTP está pendiente.</div></>}
    {formError && <div className="form-warning"><AlertTriangle size={16}/>{formError}</div>}<div className="modal-actions"><button type="button" className="btn btn-outline" onClick={close}>Cancelar</button><button className="btn btn-primary" type="submit">{kind === 'visit' ? 'Confirmar cita' : kind === 'patient' ? 'Crear ficha' : 'Guardar'}</button></div>
  </form></div>;
}
function Field({ label, name, type = 'text', required, placeholder, value, step, min, max, onChange }) { return <label className="field"><span>{label}</span><input type={type} name={name} required={required} min={min} max={max} placeholder={placeholder} value={value} step={step} onChange={onChange}/></label>; }

