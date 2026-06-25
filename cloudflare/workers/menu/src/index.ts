import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { verifyJWT } from '../../shared/jwt';

type Env  = { DB: D1Database; JWT_SECRET: string };
type Vars = { userId: string };

const app = new Hono<{ Bindings: Env; Variables: Vars }>();
app.use('*', cors());
app.get('/health', c => c.json({ status: 'ok', service: 'menu' }));

// デフォルト種目一覧 (認証不要)
app.get('/api/menus/defaults', async c => {
  const mg   = c.req.query('muscle_group') ?? '';
  const rows = await c.env.DB.prepare(
    `SELECT * FROM exercises WHERE is_default = 1 AND (? = '' OR muscle_group = ?)
     ORDER BY muscle_group, equipment, name`
  ).bind(mg, mg).all();
  return c.json(rows.results);
});

// 以降は JWT 必須
app.use('/api/menus*', async (c, next) => {
  const header  = c.req.header('Authorization') ?? '';
  const payload = await verifyJWT(header.slice(7), c.env.JWT_SECRET);
  if (!payload) return c.json({ error: 'unauthorized' }, 401);
  c.set('userId', payload.sub as string);
  await next();
});

// ユーザーの種目リスト
app.get('/api/menus', async c => {
  const uid  = c.get('userId');
  const mg   = c.req.query('muscle_group') ?? '';
  const rows = await c.env.DB.prepare(
    `SELECT ue.id, ue.user_id, ue.sort_order,
            e.id AS exercise_id, e.name, e.muscle_group, e.equipment, e.is_default
     FROM user_exercises ue
     JOIN exercises e ON e.id = ue.exercise_id
     WHERE ue.user_id = ? AND (? = '' OR e.muscle_group = ?)
     ORDER BY ue.sort_order`
  ).bind(uid, mg, mg).all();
  return c.json(rows.results);
});

// 種目を追加 (既存種目 or 新規カスタム)
app.post('/api/menus', async c => {
  const uid  = c.get('userId');
  const body = await c.req.json<{
    exercise_id?: string;
    name?: string; muscle_group?: string; equipment?: string;
  }>();

  let exerciseId = body.exercise_id;

  if (!exerciseId) {
    // カスタム種目を新規作成
    if (!body.name) return c.json({ error: 'exercise_id または name が必要です' }, 400);
    exerciseId = crypto.randomUUID();
    await c.env.DB.prepare(
      `INSERT INTO exercises (id, name, muscle_group, equipment, is_default)
       VALUES (?, ?, ?, ?, 0)
       ON CONFLICT(name, muscle_group, equipment) DO NOTHING`
    ).bind(exerciseId, body.name, body.muscle_group ?? '', body.equipment ?? '').run();
    const existing = await c.env.DB.prepare(
      'SELECT id FROM exercises WHERE name = ? AND muscle_group = ? AND equipment = ?'
    ).bind(body.name, body.muscle_group ?? '', body.equipment ?? '').first<{ id: string }>();
    exerciseId = existing!.id;
  }

  const maxRow = await c.env.DB.prepare(
    'SELECT COALESCE(MAX(sort_order), 0) AS m FROM user_exercises WHERE user_id = ?'
  ).bind(uid).first<{ m: number }>();
  const id = crypto.randomUUID();
  await c.env.DB.prepare(
    'INSERT OR IGNORE INTO user_exercises (id, user_id, exercise_id, sort_order) VALUES (?, ?, ?, ?)'
  ).bind(id, uid, exerciseId, (maxRow?.m ?? 0) + 1).run();
  return c.json({ id, user_id: uid, exercise_id: exerciseId }, 201);
});

// 種目をリストから削除
app.delete('/api/menus/:id', async c => {
  await c.env.DB.prepare(
    'DELETE FROM user_exercises WHERE id = ? AND user_id = ?'
  ).bind(c.req.param('id'), c.get('userId')).run();
  return c.body(null, 204);
});

// 並び替え
app.put('/api/menus/order', async c => {
  const uid  = c.get('userId');
  const body = await c.req.json<{ ids: string[] }>();
  const stmts = body.ids.map((id, i) =>
    c.env.DB.prepare('UPDATE user_exercises SET sort_order = ? WHERE id = ? AND user_id = ?')
      .bind(i + 1, id, uid)
  );
  await c.env.DB.batch(stmts);
  return c.body(null, 204);
});

export default app;
