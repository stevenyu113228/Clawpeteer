import { v4 as uuidv4 } from 'uuid';
import chalk from 'chalk';
import { loadConfig } from '../config.js';
import { MQTTClient } from '../mqtt-client.js';

export function registerSendCommand(program) {
  program
    .command('send')
    .description('Send a command to a remote agent for execution')
    .argument('<agent>', 'Agent ID to send the command to')
    .argument('<command>', 'Command string to execute')
    .option('-s, --stream', 'Stream stdout/stderr in real time', false)
    .option('-b, --background', 'Run command in background on agent', false)
    .option('-t, --timeout <ms>', 'Command timeout in milliseconds', '30000')
    .option('-c, --config <path>', 'Path to config file')
    .action(async (agent, command, opts) => {
      let mqtt;
      try {
        const config = loadConfig(opts.config);
        mqtt = new MQTTClient(config);
        await mqtt.connect();

        const taskId = uuidv4();
        const timeoutMs = parseInt(opts.timeout, 10);

        // Subscribe to results
        await mqtt.subscribe(`agents/${agent}/results`);

        // Subscribe to stream if requested
        if (opts.stream) {
          await mqtt.subscribe(`agents/${agent}/stream/${taskId}`);
        }

        // Publish command
        const payload = {
          id: taskId,
          type: 'execute',
          command: command,
          timeout: timeoutMs,
          background: opts.background,
          stream: opts.stream,
          timestamp: Date.now(),
        };

        await mqtt.publish(`agents/${agent}/commands`, payload);
        console.log(chalk.cyan(`Task ${taskId} sent to ${agent}`));

        if (opts.background) {
          console.log(chalk.yellow('Running in background. Use "clawpeteer status" to check.'));
          return;
        }

        // Handle streaming output
        if (opts.stream) {
          mqtt.on('message', (topic, msg) => {
            if (topic === `agents/${agent}/stream/${taskId}`) {
              if (msg.stream === 'stdout' && msg.data) {
                process.stdout.write(msg.data);
              } else if (msg.stream === 'stderr' && msg.data) {
                process.stderr.write(msg.data);
              }
            }
          });
        }

        // Wait for final result
        const result = await mqtt.waitForMessage(
          `agents/${agent}/results`,
          (msg) => msg.id === taskId && msg.status !== 'started',
          timeoutMs + 5000
        );

        if (result.status === 'completed') {
          if (result.stdout && !opts.stream) {
            process.stdout.write(result.stdout);
          }
          if (result.stderr && !opts.stream) {
            process.stderr.write(result.stderr);
          }
          process.exit(result.exitCode || 0);
        } else if (result.status === 'error') {
          console.error(chalk.red(`Error: ${result.error || 'Unknown error'}`));
          process.exit(1);
        } else if (result.status === 'killed') {
          console.error(chalk.yellow(`Process killed (signal: ${result.signal || 'unknown'})`));
          process.exit(137);
        } else {
          console.error(chalk.red(`Unexpected status: ${result.status}`));
          process.exit(1);
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
