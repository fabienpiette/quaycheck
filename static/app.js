const esc = s => s.replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]);

let containersData = [];
let sortColumn = 'name';
let sortAsc = true;

async function copyPort(el) {
    const text = el.textContent.split(':')[0];
    if (!text || text === 'host' || text === 'container') return;
    try {
        await navigator.clipboard.writeText(text);
        const original = el.textContent;
        el.textContent = 'copied';
        el.classList.add('copied');
        setTimeout(() => {
            el.textContent = original;
            el.classList.remove('copied');
        }, 800);
    } catch (e) {}
}

document.addEventListener('click', e => {
    if (e.target.classList.contains('port')) copyPort(e.target);
});

function toggleTheme() {
    const html = document.documentElement;
    const dark = html.getAttribute('data-theme') === 'dark';
    html.setAttribute('data-theme', dark ? '' : 'dark');
    localStorage.setItem('theme', dark ? 'light' : 'dark');
}

function loadTheme() {
    const saved = localStorage.getItem('theme');
    if (saved === 'dark' || (!saved && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
        document.documentElement.setAttribute('data-theme', 'dark');
    }
}

async function api(url) {
    const res = await fetch(url);
    const data = await res.json();
    if (!res.ok) {
        throw { status: res.status, ...data };
    }
    return data;
}

function showError(el, err) {
    const msg = err.message || 'Unknown error';
    const code = err.code || '';
    el.innerHTML = `<div class="error-banner">${esc(msg)}${code ? ` <code>${esc(code)}</code>` : ''}</div>`;
}

async function load() {
    const tbody = document.getElementById('containers');
    try {
        containersData = await api('/api/ports');
        sortAndRender();
    } catch (e) {
        if (e.message) {
            showError(tbody, e);
        } else {
            tbody.innerHTML = '<tr><td colspan="3" class="empty">failed to connect</td></tr>';
        }
    }
}

function sortBy(column) {
    if (sortColumn === column) {
        sortAsc = !sortAsc;
    } else {
        sortColumn = column;
        sortAsc = true;
    }
    sortAndRender();
}

function sortAndRender() {
    const sorted = [...containersData].sort((a, b) => {
        let cmp = 0;
        if (sortColumn === 'name') {
            const nameA = (a.names?.[0] || a.id).toLowerCase();
            const nameB = (b.names?.[0] || b.id).toLowerCase();
            cmp = nameA.localeCompare(nameB);
        } else if (sortColumn === 'state') {
            cmp = (a.state || '').localeCompare(b.state || '');
        } else if (sortColumn === 'ports') {
            const portA = a.ports?.[0]?.public_port || a.ports?.[0]?.private_port || 0;
            const portB = b.ports?.[0]?.public_port || b.ports?.[0]?.private_port || 0;
            cmp = portA - portB;
        }
        return sortAsc ? cmp : -cmp;
    });
    render(sorted);
    updateSortIndicators();
}

function updateSortIndicators() {
    ['name', 'state', 'ports'].forEach(col => {
        const el = document.getElementById(`sort-${col}`);
        if (el) el.textContent = sortColumn === col ? (sortAsc ? '▲' : '▼') : '';
    });
}

function render(containers) {
    const tbody = document.getElementById('containers');
    if (!containers || !containers.length) {
        tbody.innerHTML = '<tr><td colspan="3" class="empty">no containers</td></tr>';
        return;
    }
    tbody.innerHTML = containers.map(c => {
        const name = esc(c.names?.[0]?.replace(/^\//, '') || c.id.slice(0, 12));
        const image = esc(c.image || '');
        const state = esc(c.state || '');
        const ports = c.ports?.length
            ? c.ports.map(p => p.public_port
                ? `<span class="port">${esc(String(p.public_port))}:${esc(String(p.private_port))}</span>`
                : `<span class="port exposed">${esc(String(p.private_port))}</span>`
            ).join('')
            : '<span class="empty">—</span>';
        return `<tr>
            <td data-label="Name"><div class="name">${name}</div><div class="image">${image}</div></td>
            <td data-label="State"><span class="state ${state}">${state}</span></td>
            <td data-label="Ports" class="ports">${ports}</td>
        </tr>`;
    }).join('');
}

function addHistory(port, status, ok) {
    const history = document.getElementById('history');
    const time = new Date().toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
    const entry = document.createElement('div');
    entry.className = `history-entry ${ok ? 'ok' : 'err'}`;
    entry.innerHTML = `<span class="port">${esc(String(port))}</span><span class="status">${esc(status)}</span><span class="time">${time}</span>`;
    history.insertBefore(entry, history.firstChild);
    history.scrollTop = 0;
}

async function check() {
    const port = document.getElementById('port').value;
    if (!port) return;
    try {
        const data = await api(`/api/check?port=${port}`);
        addHistory(port, data.available ? 'available' : 'in use', data.available);
    } catch (e) {
        addHistory(port, e.message || 'error', false);
    }
}

async function suggest() {
    try {
        const data = await api('/api/suggest');
        if (data.port > 0) {
            document.getElementById('port').value = data.port;
            addHistory(data.port, 'suggested', true);
        }
    } catch (e) {
        addHistory('—', e.message || 'error', false);
    }
}

async function loadStats() {
    try {
        const data = await api('/api/stats');
        const el = document.getElementById('stats');
        const mem = data.memory_mb.toFixed(1);
        const bin = data.binary_kb > 1024
            ? (data.binary_kb / 1024).toFixed(1) + 'MB'
            : data.binary_kb + 'KB';
        el.textContent = `${mem}MB ram · ${bin} bin · ${data.goroutines} goroutines`;
    } catch (e) {}
}

loadTheme();
load();
loadStats();
