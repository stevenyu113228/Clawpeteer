import chalk from 'chalk';
import { loadConfig } from '../config.js';
import { MQTTClient } from '../mqtt-client.js';
import { downloadFile } from '../file-transfer.js';

export function registerDownloadCommand(program) {
  program
    .command('download')
    .description('Download a file from a remote agent')
    .argument('<agent>', 'Agent ID')
    .argument('<remotePath>', 'File path on the remote agent')
    .argument('[localPath]', 'Local destination path (default: current directory)')
    .option('-c, --config <path>', 'Path to config file')
    .action(async (agent, remotePath, localPath, opts) => {
      let mqtt;
      try {
        const config = loadConfig(opts.config);
        mqtt = new MQTTClient(config);
        await mqtt.connect();

        console.log(chalk.cyan(`Downloading ${remotePath} from ${agent}...`));

        const result = await downloadFile(mqtt, agent, remotePath, localPath || null, (received, total) => {
          if (total > 0) {
            const pct = ((received / total) * 100).toFixed(1);
            process.stdout.write(`\rProgress: ${pct}% (${received}/${total} chunks)`);
          }
        });

        console.log(''); // newline after progress

        if (result.status === 'completed') {
          console.log(chalk.green(`Download completed: ${result.localPath}`));
          console.log(`Size: ${formatSize(result.fileSize)}`);
          if (result.verified) {
            console.log(chalk.green('SHA-256 checksum verified.'));
          } else {
            console.log(chalk.yellow('SHA-256 checksum not verified.'));
          }
        } else {
          console.error(chalk.red(`Download failed: ${result.error || 'Unknown error'}`));
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

function formatSize(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}
