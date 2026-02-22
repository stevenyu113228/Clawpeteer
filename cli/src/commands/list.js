import chalk from 'chalk';
import { loadConfig } from '../config.js';
import { MQTTClient } from '../mqtt-client.js';

const STALE_THRESHOLD_MS = 90 * 1000; // 3x heartbeat interval (30s)

export function registerListCommand(program) {
  program
    .command('list')
    .description('List all online agents')
    .option('-w, --wait <seconds>', 'Seconds to wait for heartbeats', '3')
    .option('-a, --all', 'Show offline agents too', false)
    .action(async (opts, cmd) => {
      let mqtt;
      try {
        const config = loadConfig(cmd.optsWithGlobals().config);
        mqtt = new MQTTClient(config);
        await mqtt.connect();

        const agents = new Map();
        const waitMs = parseInt(opts.wait, 10) * 1000;

        mqtt.on('message', (topic, msg) => {
          const match = topic.match(/^agents\/([^/]+)\/heartbeat$/);
          if (match) {
            agents.set(match[1], msg);
          }
        });

        await mqtt.subscribe('agents/+/heartbeat');

        // Wait for retained heartbeats and any live ones
        await new Promise((resolve) => setTimeout(resolve, waitMs));

        if (agents.size === 0) {
          console.log(chalk.yellow('No agents found. Agents may be offline or not yet reporting.'));
          return;
        }

        // Determine online/offline based on heartbeat timestamp
        const now = Date.now();
        const entries = [];
        for (const [id, hb] of agents) {
          const age = hb.timestamp ? now - hb.timestamp : Infinity;
          const isOnline = age < STALE_THRESHOLD_MS;
          entries.push({ id, hb, isOnline, age });
        }

        // Filter unless --all
        const visible = opts.all ? entries : entries.filter((e) => e.isOnline);

        if (visible.length === 0) {
          const total = entries.length;
          console.log(chalk.yellow(`All ${total} agent(s) are offline. Use --all to show them.`));
          return;
        }

        // Header
        console.log(
          chalk.bold(
            padRight('AGENT ID', 24) +
            padRight('PLATFORM', 16) +
            padRight('STATUS', 12) +
            padRight('TASKS', 8) +
            padRight('LAST SEEN', 16)
          )
        );
        console.log('-'.repeat(76));

        // Rows
        for (const { id, hb, isOnline, age } of visible) {
          const platform = hb.platform
            ? `${hb.platform}/${hb.arch || '?'}`
            : 'unknown';
          const status = isOnline ? 'online' : 'offline';
          const tasks = hb.runningTasks != null ? String(hb.runningTasks) : '?';
          const lastSeen = isOnline ? 'just now' : formatAge(age);

          const statusColor = isOnline ? chalk.green : chalk.red;

          console.log(
            padRight(id, 24) +
            padRight(platform, 16) +
            statusColor(padRight(status, 12)) +
            padRight(tasks, 8) +
            padRight(lastSeen, 16)
          );
        }

        const onlineCount = entries.filter((e) => e.isOnline).length;
        const offlineCount = entries.length - onlineCount;
        let summary = `${onlineCount} online`;
        if (offlineCount > 0) {
          summary += `, ${offlineCount} offline`;
          if (!opts.all) summary += ' (use --all)';
        }
        console.log(`\n${entries.length} agent(s) found (${summary}).`);
      } catch (err) {
        console.error(chalk.red(`Error: ${err.message}`));
        process.exit(1);
      } finally {
        if (mqtt) {
          await mqtt.disconnect();
        }
      }
    });
}

function formatAge(ms) {
  const sec = Math.floor(ms / 1000);
  if (sec < 60) return `${sec}s ago`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const days = Math.floor(hr / 24);
  return `${days}d ago`;
}

function padRight(str, len) {
  return String(str).padEnd(len);
}
