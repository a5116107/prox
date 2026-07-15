const http = require('http');
const { randomUUID } = require('crypto');

const MISSKEY_HOST = process.env.MISSKEY_HOST || 'https://dc.hhhl.cc';
const MIAUTH_PERMISSION = process.env.MIAUTH_PERMISSION || 'read:account';
const PORT = Number(process.env.PORT || 8787);
const USER_CACHE_TTL_MS = 10 * 60 * 1000;
const COMMON_HEADERS = {
  'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36',
  'Accept': 'application/json',
  'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8'
};
const userCache = new Map();

function json(res, status, body) {
  const payload = JSON.stringify(body);
  res.writeHead(status, {
    'Content-Type': 'application/json',
    'Access-Control-Allow-Origin': '*',
    'Content-Length': Buffer.byteLength(payload),
    'Cache-Control': 'no-store'
  });
  res.end(payload);
}
function redirect(res, target) {
  res.writeHead(302, { Location: target, 'Cache-Control': 'no-store' });
  res.end();
}
async function readBody(req) {
  const chunks = [];
  for await (const c of req) chunks.push(c);
  return Buffer.concat(chunks).toString('utf8');
}
function getOrigin(req) {
  const proto = req.headers['x-forwarded-proto'] || 'https';
  const host = req.headers['x-forwarded-host'] || req.headers.host;
  return `${proto}://${host}`;
}
function pruneUserCache() {
  const now = Date.now();
  for (const [key, value] of userCache.entries()) {
    if (!value || (now - value.at) > USER_CACHE_TTL_MS) userCache.delete(key);
  }
}
function normalizeUserPayload(payload) {
  if (!payload || typeof payload !== 'object') return null;
  const id = payload.id ?? payload.userId ?? payload.user_id;
  if (id === undefined || id === null || id === '') return null;
  const username = payload.username ?? payload.userName ?? payload.handle ?? payload.name ?? '';
  const display = payload.display_name ?? payload.displayName ?? payload.name ?? username;
  return {
    id: String(id),
    username: String(username || ''),
    display_name: String(display || ''),
    usernyame: String(username || ''),
    display_nyame: String(display || '')
  };
}
function cacheUser(token, user) {
  const normalized = normalizeUserPayload(user);
  if (!token || !normalized) return;
  pruneUserCache();
  userCache.set(token, { at: Date.now(), user: normalized });
}
function getCachedUser(token) {
  if (!token) return null;
  pruneUserCache();
  const record = userCache.get(token);
  return record ? record.user : null;
}

http.createServer(async (req, res) => {
  try {
    const url = new URL(req.url, getOrigin(req));

    if (req.method === 'OPTIONS') return json(res, 200, { ok: true });
    if (url.pathname === '/health') {
      res.writeHead(200, { 'Content-Type': 'text/plain', 'Cache-Control': 'no-store' });
      return res.end('ok');
    }

    // Original-compatible MiAuth authorization endpoint. New API state is passed through untouched.
    if (url.pathname === '/authorize') {
      const redirectUri = url.searchParams.get('redirect_uri');
      const state = url.searchParams.get('state') || '';
      if (!redirectUri) return json(res, 400, { error: 'invalid_request', error_description: 'Missing redirect_uri' });
      const sessionId = randomUUID();
      const workerCallback = `${getOrigin(req)}/callback?r=${encodeURIComponent(redirectUri)}&s=${encodeURIComponent(state)}`;
      const miAuthUrl = `${MISSKEY_HOST}/miauth/${sessionId}?name=NewAPI%E7%99%BB%E5%BD%95&callback=${encodeURIComponent(workerCallback)}&permission=${encodeURIComponent(MIAUTH_PERMISSION)}`;
      return redirect(res, miAuthUrl);
    }

    if (url.pathname === '/callback') {
      const originalRedirectUri = url.searchParams.get('r');
      const state = url.searchParams.get('s') || '';
      const session = url.searchParams.get('session') || '';
      if (!originalRedirectUri) return json(res, 400, { error: 'invalid_request', error_description: 'Missing original redirect uri' });
      const target = new URL(originalRedirectUri);
      target.searchParams.set('code', session);
      target.searchParams.set('state', state);
      return redirect(res, target.toString());
    }

    if (url.pathname === '/token') {
      let code = url.searchParams.get('code');
      if (!code) {
        const bodyText = await readBody(req);
        try { code = JSON.parse(bodyText).code; }
        catch { code = new URLSearchParams(bodyText).get('code'); }
      }
      if (!code) return json(res, 400, { error: 'invalid_request', error_description: 'Missing code' });

      const tokenResponse = await fetch(`${MISSKEY_HOST}/api/miauth/${encodeURIComponent(code)}/check`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...COMMON_HEADERS },
        body: '{}'
      });
      const tokenData = await tokenResponse.json().catch(() => ({}));
      if (!tokenData.ok || !tokenData.token) {
        console.error('token invalid_grant', JSON.stringify({ status: tokenResponse.status, body: tokenData }));
        return json(res, 400, { error: 'invalid_grant', error_description: 'Failed to validate MiAuth session' });
      }

      cacheUser(tokenData.token, tokenData.user);
      const response = {
        access_token: tokenData.token,
        token_type: 'Bearer'
      };
      if (tokenData.user) response.user = normalizeUserPayload(tokenData.user);
      return json(res, 200, response);
    }

    if (url.pathname === '/userinfo') {
      let token = url.searchParams.get('access_token');
      if (!token) token = String(req.headers.authorization || '').replace(/^Bearer\s+/i, '').trim();
      if (!token) return json(res, 401, { error: 'invalid_request', error_description: 'Missing token' });

      const cachedUser = getCachedUser(token);
      if (cachedUser) {
        return json(res, 200, cachedUser);
      }

      const userResponse = await fetch(`${MISSKEY_HOST}/api/i`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...COMMON_HEADERS },
        body: JSON.stringify({ i: token })
      });
      const userData = await userResponse.json().catch(() => ({}));
      if (!userData || !userData.id) {
        console.error('userinfo invalid_token', JSON.stringify({ status: userResponse.status, body: userData }));
        return json(res, 401, { error: 'invalid_token' });
      }

      const normalized = normalizeUserPayload(userData);
      cacheUser(token, normalized);
      return json(res, 200, normalized);
    }

    return json(res, 404, { error: 'not_found' });
  } catch (e) {
    console.error('oauth-worker server_error', String((e && e.stack) || e));
    return json(res, 500, { error: 'server_error', error_description: String((e && e.message) || e) });
  }
}).listen(PORT, '0.0.0.0', () => console.log(`oauth-worker original-compatible listening on ${PORT}`));
