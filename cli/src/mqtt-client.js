import mqtt from 'mqtt';
import { readFileSync } from 'fs';
import { EventEmitter } from 'events';

export class MQTTClient extends EventEmitter {
  constructor(config) {
    super();
    this.config = config;
    this.client = null;
  }

  async connect() {
    return new Promise((resolve, reject) => {
      const options = {
        username: this.config.username,
        password: this.config.password,
        clientId: this.config.clientId || undefined,
        clean: true,
        connectTimeout: 10000,
      };

      if (this.config.caFile) {
        options.ca = [readFileSync(this.config.caFile)];
        options.rejectUnauthorized = true;
      }

      this.client = mqtt.connect(this.config.brokerUrl, options);

      this.client.on('connect', () => {
        resolve();
      });

      this.client.on('error', (err) => {
        this.emit('error', err);
        reject(err);
      });

      this.client.on('message', (topic, payload) => {
        let parsed;
        try {
          parsed = JSON.parse(payload.toString());
        } catch {
          parsed = payload.toString();
        }
        this.emit('message', topic, parsed);
      });

      this.client.on('close', () => {
        this.emit('close');
      });

      this.client.on('offline', () => {
        this.emit('offline');
      });
    });
  }

  subscribe(topic, qos = 1) {
    return new Promise((resolve, reject) => {
      this.client.subscribe(topic, { qos }, (err) => {
        if (err) reject(err);
        else resolve();
      });
    });
  }

  publish(topic, payload, qos = 1, retain = false) {
    return new Promise((resolve, reject) => {
      const data = typeof payload === 'string' ? payload : JSON.stringify(payload);
      this.client.publish(topic, data, { qos, retain }, (err) => {
        if (err) reject(err);
        else resolve();
      });
    });
  }

  waitForMessage(topicPattern, filter, timeoutMs = 30000) {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.removeListener('message', handler);
        reject(new Error(`Timeout waiting for message on ${topicPattern} after ${timeoutMs}ms`));
      }, timeoutMs);

      const handler = (topic, msg) => {
        const matches = topicPattern instanceof RegExp
          ? topicPattern.test(topic)
          : topic === topicPattern || this._matchMqttTopic(topicPattern, topic);

        if (matches && (!filter || filter(msg))) {
          clearTimeout(timer);
          this.removeListener('message', handler);
          resolve(msg);
        }
      };

      this.on('message', handler);
    });
  }

  _matchMqttTopic(pattern, topic) {
    const patternParts = pattern.split('/');
    const topicParts = topic.split('/');

    for (let i = 0; i < patternParts.length; i++) {
      if (patternParts[i] === '#') return true;
      if (patternParts[i] === '+') continue;
      if (patternParts[i] !== topicParts[i]) return false;
    }

    return patternParts.length === topicParts.length;
  }

  disconnect() {
    return new Promise((resolve) => {
      if (this.client) {
        this.client.end(false, {}, () => {
          resolve();
        });
      } else {
        resolve();
      }
    });
  }
}
