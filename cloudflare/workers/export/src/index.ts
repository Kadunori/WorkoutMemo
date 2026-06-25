import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { verifyJWT } from '../../shared/jwt';

// Service Bindings で workout-service と user-service を呼ぶ
type Env = {
  JWT_SECRET: string;
  WORKOUT_SERVICE: Fetcher;  // wrangler.toml [[services]] で定義
  USER_SERVICE: Fetcher;
  EXPORT_BUCKET?: R2Bucket;  // R2 バインディング (オプション: S3互換ストレージ)
};
type Vars = { userId: string; rawToken: string };

const app = new Hono<{ Bindings: Env; Variables: Vars }>();
app.use('*', cors());
app.get('/health', c => c.json({ status: 'ok', service: 'export' }));

app.use('/api/*', async (c, next) => {
  const header  = c.req.header('Authorization') ?? '';
  const payload = await verifyJWT(header.slice(7), c.env.JWT_SECRET);
  if (!payload) return c.json({ error: 'unauthorized' }, 401);
  c.set('userId', payload.sub as string);
  c.set('rawToken', header.slice(7));
  await next();
});

// Service Binding 経由で他 Worker を呼ぶ
async function callService<T>(fetcher: Fetcher, path: string, token: string): Promise<T> {
  const res = await fetcher.fetch(`https://internal${path}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  return res.json<T>();
}

// CSV エクスポート
app.get('/api/export/workouts', async c => {
  const token = c.get('rawToken');

  // 並列でセッション一覧と体重記録を取得
  const [sessions, weights] = await Promise.all([
    callService<Array<{ id: string; muscle_group: string; date: string }>>(
      c.env.WORKOUT_SERVICE, '/api/workouts', token
    ),
    callService<Array<{ weight: number; recorded_at: string }>>(
      c.env.USER_SERVICE, '/api/users/weight', token
    ),
  ]);

  // 全セッションのセットを並列取得
  const sessionDetails = await Promise.all(
    sessions.map(s =>
      callService<{ sets: Array<{ exercise_name: string; equipment: string; set_number: number; weight: number; reps: number; rir: number | null }> }>(
        c.env.WORKOUT_SERVICE, `/api/workouts/${s.id}`, token
      ).then(d => ({ session: s, sets: d.sets ?? [] }))
    )
  );

  // CSV 生成
  const rows: string[] = ['type,date,muscle_group,exercise,equipment,set,weight_kg,reps,rir'];
  for (const { session, sets } of sessionDetails) {
    for (const s of sets) {
      rows.push([
        'workout', session.date, session.muscle_group,
        `"${s.exercise_name}"`, s.equipment,
        s.set_number, s.weight, s.reps, s.rir ?? '',
      ].join(','));
    }
  }
  for (const bw of weights) {
    rows.push(['bodyweight', bw.recorded_at.slice(0, 10), '', '', '', '', bw.weight, '', ''].join(','));
  }

  const csv = rows.join('\n');

  // R2 へも保存 (バケットが設定されている場合)
  if (c.env.EXPORT_BUCKET) {
    const key = `exports/${c.get('userId')}/${new Date().toISOString().slice(0, 10)}.csv`;
    await c.env.EXPORT_BUCKET.put(key, csv, { httpMetadata: { contentType: 'text/csv' } });
  }

  return new Response(csv, {
    headers: {
      'Content-Type': 'text/csv; charset=utf-8',
      'Content-Disposition': 'attachment; filename="workout_export.csv"',
    },
  });
});

export default app;
