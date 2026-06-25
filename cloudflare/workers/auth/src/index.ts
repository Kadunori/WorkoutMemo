import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { signJWT, verifyJWT } from '../../shared/jwt';
import { hashPassword, verifyPassword } from '../../shared/password';

type Env = {
  DB: D1Database;
  JWT_SECRET: string;
};

const app = new Hono<{ Bindings: Env }>();
app.use('*', cors());

// ヘルスチェック
app.get('/health', c => c.json({ status: 'ok', service: 'auth' }));

// ユーザー登録
app.post('/api/auth/register', async c => {
  const body = await c.req.json<{ email: string; password: string }>();
  if (!body.email || !body.password || body.password.length < 8) {
    return c.json({ error: 'email と 8文字以上のパスワードが必要です' }, 400);
  }

  const existing = await c.env.DB
    .prepare('SELECT id FROM users WHERE email = ?')
    .bind(body.email).first();
  if (existing) return c.json({ error: 'このメールアドレスは登録済みです' }, 409);

  const id           = crypto.randomUUID();
  const passwordHash = await hashPassword(body.password);

  await c.env.DB
    .prepare('INSERT INTO users (id, email, password_hash) VALUES (?, ?, ?)')
    .bind(id, body.email, passwordHash).run();

  const token = await signJWT({ sub: id, email: body.email }, c.env.JWT_SECRET);
  return c.json({ token, user: { id, email: body.email } }, 201);
});

// ログイン
app.post('/api/auth/login', async c => {
  const body = await c.req.json<{ email: string; password: string }>();
  const user = await c.env.DB
    .prepare('SELECT id, email, password_hash FROM users WHERE email = ?')
    .bind(body.email).first<{ id: string; email: string; password_hash: string }>();

  if (!user || !(await verifyPassword(body.password, user.password_hash))) {
    return c.json({ error: 'メールアドレスまたはパスワードが違います' }, 401);
  }

  const token = await signJWT({ sub: user.id, email: user.email }, c.env.JWT_SECRET);
  return c.json({ token, user: { id: user.id, email: user.email } });
});

// トークン検証 (他 Worker から呼ばれる / フロントエンドの確認用)
app.get('/api/auth/validate', async c => {
  const header = c.req.header('Authorization') ?? '';
  if (!header.startsWith('Bearer ')) {
    return c.json({ error: 'missing token' }, 401);
  }
  const payload = await verifyJWT(header.slice(7), c.env.JWT_SECRET);
  if (!payload) return c.json({ error: 'invalid token' }, 401);
  return c.json({ user_id: payload.sub, email: payload.email });
});

export default app;
