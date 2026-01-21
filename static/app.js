const esc = s => s.replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]);

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
        const data = await api('/api/ports');
        render(data);
    } catch (e) {
        if (e.message) {
            showError(tbody, e);
        } else {
            tbody.innerHTML = '<tr><td colspan="3" class="empty">failed to connect</td></tr>';
        }
    }
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

loadTheme();
load();
