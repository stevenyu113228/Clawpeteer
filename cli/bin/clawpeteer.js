#!/usr/bin/env node
import { program } from 'commander';

program
  .name('clawpeteer')
  .description('MQTT remote control CLI for OpenClaw')
  .version('1.0.0');

program.parse();
