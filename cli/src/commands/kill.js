import chalk from 'chalk';
import { loadConfig } from '../config.js';
import { MQTTClient } from '../mqtt-client.js';

export function registerKillCommand(program) {
  program
    .command('kill')
    .description('Kill a running task on a remote agent')
    .argument('<agent>', 'Agent ID')
    .argument('<taskId>', 'Task ID to kill')
    .option('-s, --signal <signal>', 'Signal to send', 'SIGTERM')
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

        // Send kill command
        const killPayload = {
          action: 'kill',
          signal: opts.signal,
        };
        await mqtt.publish(`agents/${agent}/control/${taskId}`, killPayload);

        console.log(chalk.cyan(`Kill signal (${opts.signal}) sent for task ${taskId} on ${agent}`));

        // Wait for kill confirmation
        const result = await mqtt.waitForMessage(
          `agents/${agent}/results`,
          (msg) => msg.id === taskId && msg.status === 'killed',
          timeoutMs
        );

        console.log(chalk.green(`Task ${taskId} killed successfully.`));
        if (result.signal) {
          console.log(`Signal: ${result.signal}`);
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
