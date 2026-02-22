import { existsSync, statSync } from 'fs';
import chalk from 'chalk';
import { loadConfig } from '../config.js';
import { MQTTClient } from '../mqtt-client.js';
import { uploadFile } from '../file-transfer.js';

export function registerUploadCommand(program) {
  program
    .command('upload')
    .description('Upload a file to a remote agent')
    .argument('<agent>', 'Agent ID')
    .argument('<localPath>', 'Local file path to upload')
    .argument('<remotePath>', 'Destination path on the agent')
    .action(async (agent, localPath, remotePath, opts, cmd) => {
      let mqtt;
      try {
        // Validate local file exists
        if (!existsSync(localPath)) {
          console.error(chalk.red(`File not found: ${localPath}`));
          process.exit(1);
        }

        const stat = statSync(localPath);
        if (!stat.isFile()) {
          console.error(chalk.red(`Not a file: ${localPath}`));
          process.exit(1);
        }

        const config = loadConfig(cmd.optsWithGlobals().config);
        mqtt = new MQTTClient(config);
        await mqtt.connect();

        const fileSize = stat.size;
        console.log(chalk.cyan(`Uploading ${localPath} (${formatSize(fileSize)}) to ${agent}:${remotePath}`));

        const result = await uploadFile(mqtt, agent, localPath, remotePath, (sent, total) => {
          const pct = ((sent / total) * 100).toFixed(1);
          process.stdout.write(`\rProgress: ${pct}% (${sent}/${total} chunks)`);
        });

        console.log(''); // newline after progress

        if (result.status === 'completed') {
          console.log(chalk.green('Upload completed successfully.'));
          if (result.verified) {
            console.log(chalk.green('SHA-256 checksum verified.'));
          }
        } else {
          console.error(chalk.red(`Upload failed: ${result.error || 'Unknown error'}`));
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
