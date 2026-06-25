// PBKDF2 によるパスワードハッシュ (Web Crypto API)
// bcrypt は Workers 未対応のため PBKDF2-SHA256 (100,000 回) を使用

export async function hashPassword(password: string): Promise<string> {
  const salt = crypto.getRandomValues(new Uint8Array(16));
  const key  = await crypto.subtle.importKey('raw', new TextEncoder().encode(password), 'PBKDF2', false, ['deriveBits']);
  const hash = await crypto.subtle.deriveBits({ name: 'PBKDF2', hash: 'SHA-256', salt, iterations: 100_000 }, key, 256);
  const hex  = (b: Uint8Array) => [...b].map(x => x.toString(16).padStart(2, '0')).join('');
  return `${hex(salt)}:${hex(new Uint8Array(hash))}`;
}

export async function verifyPassword(password: string, stored: string): Promise<boolean> {
  const [saltHex, hashHex] = stored.split(':');
  const salt = new Uint8Array((saltHex.match(/.{2}/g) ?? []).map(b => parseInt(b, 16)));
  const key  = await crypto.subtle.importKey('raw', new TextEncoder().encode(password), 'PBKDF2', false, ['deriveBits']);
  const hash = await crypto.subtle.deriveBits({ name: 'PBKDF2', hash: 'SHA-256', salt, iterations: 100_000 }, key, 256);
  const hex  = [...new Uint8Array(hash)].map(x => x.toString(16).padStart(2, '0')).join('');
  return hex === hashHex;
}
