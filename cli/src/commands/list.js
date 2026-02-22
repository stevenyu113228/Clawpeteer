import chalk from 'chalk';
import { loadConfig } from '../config.js';
import { MQTTClient } from '../mqtt-client.js';

export function registerListCommand(program) {
  program
    .command('list')
    .description('List all online agents')
    .option('-w, --wait <seconds>', 'Seconds to wait for heartbeats', '3')
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

        // Header
        console.log(
          chalk.bold(
            padRight('AGENT ID', 24) +
            padRight('PLATFORM', 16) +
            padRight('STATUS', 12) +
            padRight('TASKS', 8)
          )
        );
        console.log('-'.repeat(60));

        // Rows
        for (const [id, hb] of agents) {
          const platform = hb.platform
            ? `${hb.platform}/${hb.arch || '?'}`
            : 'unknown';
          const status = hb.status || 'unknown';
          const tasks = hb.runningTasks != null ? String(hb.runningTasks) : '?';

          const statusColor = status === 'idle'
            ? chalk.green
            : status === 'busy'
              ? chalk.yellow
              : chalk.white;

          console.log(
            padRight(id, 24) +
            padRight(platform, 16) +
            statusColor(padRight(status, 12)) +
            padRight(tasks, 8)
          );
        }

        console.log(`\n${agents.size} agent(s) found.`);
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

function padRight(str, len) {
  return String(str).padEnd(len);
}
