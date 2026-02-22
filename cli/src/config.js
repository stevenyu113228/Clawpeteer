import { readFileSync, existsSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

export function loadConfig(configPath) {
  const paths = [
    configPath,
    join(process.cwd(), 'config.json'),
    join(__dirname, '..', 'config.json'),
    join(process.env.HOME || '', '.clawpeteer', 'config.json'),
  ].filter(Boolean);

  for (const p of paths) {
    if (existsSync(p)) {
      const data = readFileSync(p, 'utf-8');
      return JSON.parse(data);
    }
  }

  throw new Error(
    'Config not found. Create config.json or run with --config <path>\n' +
    'See config.example.json for template.'
  );
}
