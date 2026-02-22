import chalk from 'chalk';
import { loadConfig } from '../config.js';
import { MQTTClient } from '../mqtt-client.js';

export function registerStatusCommand(program) {
  program
    .command('status')
    .description('Query task status on a remote agent')
    .argument('<agent>', 'Agent ID to query')
    .argument('[taskId]', 'Specific task ID to query')
    .option('-c, --config <path>', 'Path to config file')
    .option('-t, --timeout <ms>', 'Timeout in milliseconds', '10000')
    .action(async (agent, taskId, opts) => {
      let mqtt;
      try {
        const config = loadConfig(opts.config);
        mqtt = new MQTTClient(config);
        await mqtt.connect();

        const timeoutMs = parseInt(opts.timeout, 10);

        await mqtt.subscribe(`agents/${agent}/results`);

        // Send query
        const queryPayload = { action: 'list' };
        await mqtt.publish(`agents/${agent}/control/query`, queryPayload);

        console.log(chalk.cyan(`Querying agent ${agent}...`));

        // Wait for response
        const response = await mqtt.waitForMessage(
          `agents/${agent}/results`,
          (msg) => msg.action === 'list',
          timeoutMs
        );

        const tasks = response.tasks || [];

        if (taskId) {
          const task = tasks.find((t) => t.id === taskId);
          if (!task) {
            console.log(chalk.yellow(`Task ${taskId} not found on agent ${agent}.`));
            return;
          }
          printTaskDetail(task);
        } else {
          if (tasks.length === 0) {
            console.log(chalk.yellow('No tasks found on this agent.'));
            return;
          }

          // Header
          console.log(
            chalk.bold(
              padRight('TASK ID', 40) +
              padRight('COMMAND', 30) +
              padRight('STATUS', 12) +
              padRight('DURATION', 12)
            )
          );
          console.log('-'.repeat(94));

          for (const task of tasks) {
            const statusColor =
              task.status === 'running' ? chalk.green :
              task.status === 'completed' ? chalk.blue :
              task.status === 'error' ? chalk.red :
              task.status === 'killed' ? chalk.yellow :
              chalk.white;

            const duration = task.duration
              ? formatDuration(task.duration)
              : task.startedAt
                ? formatDuration(Date.now() - task.startedAt)
                : '?';

            const cmd = (task.command || '').substring(0, 28);

            console.log(
              padRight(task.id || '?', 40) +
              padRight(cmd, 30) +
              statusColor(padRight(task.status || '?', 12)) +
              padRight(duration, 12)
            );
          }

          console.log(`\n${tasks.length} task(s) on agent ${agent}.`);
        }
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

function printTaskDetail(task) {
  console.log(chalk.bold('Task Details:'));
  console.log(`  ID:       ${task.id}`);
  console.log(`  Command:  ${task.command}`);
  console.log(`  Status:   ${task.status}`);
  if (task.exitCode != null) {
    console.log(`  Exit Code: ${task.exitCode}`);
  }
  if (task.startedAt) {
    console.log(`  Started:  ${new Date(task.startedAt).toISOString()}`);
  }
  if (task.duration) {
    console.log(`  Duration: ${formatDuration(task.duration)}`);
  }
}

function formatDuration(ms) {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const mins = Math.floor(ms / 60000);
  const secs = Math.floor((ms % 60000) / 1000);
  return `${mins}m${secs}s`;
}

function padRight(str, len) {
  return String(str).padEnd(len);
}
