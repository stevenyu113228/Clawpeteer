#!/usr/bin/env node
import { program } from 'commander';
import { registerSendCommand } from '../src/commands/send.js';

program
  .name('clawpeteer')
  .description('MQTT remote control CLI for OpenClaw')
  .version('1.0.0');

registerSendCommand(program);

program.parse();
