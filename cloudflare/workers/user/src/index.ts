import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { verifyJWT } from '../../shared/jwt';

type Env  = { DB: D1Database; JWT_SECRET: string };
type Vars = { userId: string };

const app = new Hono<{ Bindings: Env; Variables: Vars }>();
app.use('*', cors());
app.get('/health', c => c.json({ status: 'ok', service: 'user' }));

app.use('/api/*', async (c, next) => {
  const header  = c.req.header('Authorization') ?? '';
  const payload = await verifyJWT(header.slice(7), c.env.JWT_SECRET);
  if (!payload) return c.json({ error: 'unauthorized' }, 401);
  c.set('userId', payload.sub as string);
  await next();
});

// プロフィール取得
app.get('/api/users/profile', async c => {
  const uid = c.get('userId');
  const row = await c.env.DB.prepare(
    'SELECT * FROM user_profiles WHERE user_id = ?'
  ).bind(uid).first();
  return c.json(row ?? { user_id: uid, height: null, unit: 'kg' });
});

// プロフィール更新 (upsert)
app.put('/api/users/profile', async c => {
  const uid  = c.get('userId');
  const body = await c.req.json<{ height?: number; unit?: string }>();
  await c.env.DB.prepare(
    `INSERT INTO user_profiles (user_id, height, unit)
     VALUES (?, ?, ?)
     ON CONFLICT(user_id) DO UPDATE SET height = excluded.height, unit = excluded.unit, updated_at = datetime('now')`
  ).bind(uid, body.height ?? null, body.unit ?? 'kg').run();
  const row = await c.env.DB.prepare('SELECT * FROM user_profiles WHERE user_id = ?').bind(uid).first();
  return c.json(row);
});

// 体重記録一覧
app.get('/api/users/weight', async c => {
  const uid  = c.get('userId');
  const from = c.req.query('from') ?? '';
  const rows = await c.env.DB.prepare(
    `SELECT * FROM body_weight_records
     WHERE user_id = ? AND (? = '' OR recorded_at >= ?)
     ORDER BY recorded_at ASC`
  ).bind(uid, from, from).all();
  return c.json(rows.results);
});

// 体重記録追加
app.post('/api/users/weight', async c => {
  const uid  = c.get('userId');
  const body = await c.req.json<{ weight: number; recorded_at?: string }>();
  if (!body.weight || body.weight <= 0) return c.json({ error: 'weight required' }, 400);
  const id  = crypto.randomUUID();
  const at  = body.recorded_at ?? new Date().toISOString();
  await c.env.DB.prepare(
    'INSERT INTO body_weight_records (id, user_id, weight, recorded_at) VALUES (?, ?, ?, ?)'
  ).bind(id, uid, body.weight, at).run();
  const row = await c.env.DB.prepare('SELECT * FROM body_weight_records WHERE id = ?').bind(id).first();
  return c.json(row, 201);
});

// 体重記録削除
app.delete('/api/users/weight/:id', async c => {
  await c.env.DB.prepare(
    'DELETE FROM body_weight_records WHERE id = ? AND user_id = ?'
  ).bind(c.req.param('id'), c.get('userId')).run();
  return c.body(null, 204);
});

export default app;
