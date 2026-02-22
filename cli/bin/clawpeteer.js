#!/usr/bin/env node
import { program } from 'commander';
import { registerSendCommand } from '../src/commands/send.js';
import { registerListCommand } from '../src/commands/list.js';
import { registerStatusCommand } from '../src/commands/status.js';
import { registerKillCommand } from '../src/commands/kill.js';
import { registerUploadCommand } from '../src/commands/upload.js';
import { registerDownloadCommand } from '../src/commands/download.js';

program
  .name('clawpeteer')
  .description('MQTT remote control CLI for OpenClaw')
  .version('1.0.0')
  .option('-c, --config <path>', 'Path to config file')
  .enablePositionalOptions()
  .passThroughOptions();

registerSendCommand(program);
registerListCommand(program);
registerStatusCommand(program);
registerKillCommand(program);
registerUploadCommand(program);
registerDownloadCommand(program);

program.parse();
