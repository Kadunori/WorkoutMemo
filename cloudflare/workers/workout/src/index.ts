import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { verifyJWT } from '../../shared/jwt';

type Env = { DB: D1Database; JWT_SECRET: string };
type Vars = { userId: string };

const app = new Hono<{ Bindings: Env; Variables: Vars }>();
app.use('*', cors());
app.get('/health', c => c.json({ status: 'ok', service: 'workout' }));

// JWT ミドルウェア
app.use('/api/*', async (c, next) => {
  const header = c.req.header('Authorization') ?? '';
  const payload = await verifyJWT(header.slice(7), c.env.JWT_SECRET);
  if (!payload) return c.json({ error: 'unauthorized' }, 401);
  c.set('userId', payload.sub as string);
  await next();
});

// セッション一覧
app.get('/api/workouts', async c => {
  const uid = c.get('userId');
  const { muscle_group = '', from, to } = c.req.query();
  const rows = await c.env.DB.prepare(
    `SELECT id, user_id, muscle_group, date, created_at
     FROM workout_sessions
     WHERE user_id = ?
       AND (? = '' OR muscle_group = ?)
       AND (? = '' OR date >= ?)
       AND (? = '' OR date <= ?)
     ORDER BY date DESC`
  ).bind(uid, muscle_group, muscle_group, from ?? '', from ?? '', to ?? '', to ?? '').all();
  return c.json(rows.results);
});

// セッション作成
app.post('/api/workouts', async c => {
  const uid  = c.get('userId');
  const body = await c.req.json<{ muscle_group: string; date?: string }>();
  if (!body.muscle_group) return c.json({ error: 'muscle_group required' }, 400);
  const id   = crypto.randomUUID();
  const date = body.date ?? new Date().toISOString().slice(0, 10);
  await c.env.DB.prepare(
    'INSERT INTO workout_sessions (id, user_id, muscle_group, date) VALUES (?, ?, ?, ?)'
  ).bind(id, uid, body.muscle_group, date).run();
  const row = await c.env.DB.prepare('SELECT * FROM workout_sessions WHERE id = ?').bind(id).first();
  return c.json(row, 201);
});

// セッション詳細 (セット含む)
app.get('/api/workouts/:id', async c => {
  const uid     = c.get('userId');
  const session = await c.env.DB.prepare(
    'SELECT * FROM workout_sessions WHERE id = ? AND user_id = ?'
  ).bind(c.req.param('id'), uid).first();
  if (!session) return c.json({ error: 'not found' }, 404);
  const sets = await c.env.DB.prepare(
    'SELECT * FROM workout_sets WHERE session_id = ? ORDER BY exercise_name, set_number'
  ).bind(c.req.param('id')).all();
  return c.json({ ...session, sets: sets.results });
});

// セッション削除
app.delete('/api/workouts/:id', async c => {
  await c.env.DB.prepare(
    'DELETE FROM workout_sessions WHERE id = ? AND user_id = ?'
  ).bind(c.req.param('id'), c.get('userId')).run();
  return c.body(null, 204);
});

// セット追加
app.post('/api/workouts/:id/sets', async c => {
  const uid  = c.get('userId');
  const sess = await c.env.DB.prepare(
    'SELECT id FROM workout_sessions WHERE id = ? AND user_id = ?'
  ).bind(c.req.param('id'), uid).first();
  if (!sess) return c.json({ error: 'session not found' }, 404);

  const body = await c.req.json<{
    exercise_name: string; equipment?: string;
    set_number: number; weight: number; reps: number; rir?: number;
  }>();
  const id = crypto.randomUUID();
  await c.env.DB.prepare(
    `INSERT INTO workout_sets (id, session_id, exercise_name, equipment, set_number, weight, reps, rir)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
  ).bind(id, c.req.param('id'), body.exercise_name, body.equipment ?? '',
    body.set_number, body.weight, body.reps, body.rir ?? null).run();
  const row = await c.env.DB.prepare('SELECT * FROM workout_sets WHERE id = ?').bind(id).first();
  return c.json(row, 201);
});

// セット更新
app.put('/api/workouts/:id/sets/:setId', async c => {
  const body = await c.req.json<{ weight: number; reps: number; rir?: number }>();
  await c.env.DB.prepare(
    'UPDATE workout_sets SET weight = ?, reps = ?, rir = ? WHERE id = ?'
  ).bind(body.weight, body.reps, body.rir ?? null, c.req.param('setId')).run();
  const row = await c.env.DB.prepare('SELECT * FROM workout_sets WHERE id = ?')
    .bind(c.req.param('setId')).first();
  return c.json(row);
});

// セット削除
app.delete('/api/workouts/:id/sets/:setId', async c => {
  await c.env.DB.prepare('DELETE FROM workout_sets WHERE id = ?')
    .bind(c.req.param('setId')).run();
  return c.body(null, 204);
});

// 前回重量取得 (自動入力機能)
app.get('/api/workouts/last-set', async c => {
  const uid      = c.get('userId');
  const exercise = c.req.query('exercise') ?? '';
  const setNum   = Number(c.req.query('set') ?? 1);
  const row = await c.env.DB.prepare(
    `SELECT ws.* FROM workout_sets ws
     JOIN workout_sessions sess ON sess.id = ws.session_id
     WHERE sess.user_id = ? AND ws.exercise_name = ? AND ws.set_number = ?
     ORDER BY ws.created_at DESC LIMIT 1`
  ).bind(uid, exercise, setNum).first();
  if (!row) return c.json({ error: 'no previous record' }, 404);
  return c.json(row);
});

export default app;
