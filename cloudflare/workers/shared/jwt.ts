// Web Crypto API ベースの JWT ユーティリティ (Cloudflare Workers 環境用)
// 外部ライブラリ不要 — Workers ランタイム組み込みの crypto.subtle を使用

function b64url(buf: ArrayBuffer | string): string {
  const str = typeof buf === 'string' ? buf : String.fromCharCode(...new Uint8Array(buf));
  return btoa(str).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
}

function b64urlDecode(str: string): ArrayBuffer {
  const b64 = str.replace(/-/g, '+').replace(/_/g, '/').padEnd(
    str.length + (4 - (str.length % 4)) % 4, '='
  );
  return new Uint8Array([...atob(b64)].map(c => c.charCodeAt(0))).buffer;
}

async function hmacKey(secret: string): Promise<CryptoKey> {
  return crypto.subtle.importKey(
    'raw',
    new TextEncoder().encode(secret),
    { name: 'HMAC', hash: 'SHA-256' },
    false,
    ['sign', 'verify'],
  );
}

export async function signJWT(
  payload: Record<string, unknown>,
  secret: string,
  expiresInSec = 86400,
): Promise<string> {
  const now = Math.floor(Date.now() / 1000);
  const full = { ...payload, iat: now, exp: now + expiresInSec };
  const header = b64url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const body   = b64url(JSON.stringify(full));
  const data   = `${header}.${body}`;
  const sig    = await crypto.subtle.sign('HMAC', await hmacKey(secret), new TextEncoder().encode(data));
  return `${data}.${b64url(sig)}`;
}

export async function verifyJWT(
  token: string,
  secret: string,
): Promise<Record<string, unknown> | null> {
  const parts = token.split('.');
  if (parts.length !== 3) return null;
  const data = `${parts[0]}.${parts[1]}`;
  const valid = await crypto.subtle.verify(
    'HMAC',
    await hmacKey(secret),
    b64urlDecode(parts[2]),
    new TextEncoder().encode(data),
  );
  if (!valid) return null;
  const payload = JSON.parse(atob(parts[1].replace(/-/g, '+').replace(/_/g, '/')));
  if (payload.exp && payload.exp < Date.now() / 1000) return null;
  return payload;
}
