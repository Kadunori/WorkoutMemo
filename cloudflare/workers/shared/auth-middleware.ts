import { Context, Next } from 'hono';
import { verifyJWT } from './jwt';

type Env = { JWT_SECRET: string };

// Hono ミドルウェア: JWT 検証 → c.set('userId', ...) にセット
export function jwtMiddleware() {
  return async (c: Context<{ Bindings: Env }>, next: Next) => {
    const header = c.req.header('Authorization') ?? '';
    if (!header.startsWith('Bearer ')) {
      return c.json({ error: 'missing authorization' }, 401);
    }
    const payload = await verifyJWT(header.slice(7), c.env.JWT_SECRET);
    if (!payload) {
      return c.json({ error: 'invalid or expired token' }, 401);
    }
    c.set('userId', payload.sub as string);
    await next();
  };
}
