import { createReadStream, statSync } from 'fs';
import { createHash } from 'crypto';
import { readFileSync } from 'fs';
import { basename } from 'path';
import { v4 as uuidv4 } from 'uuid';

export const CHUNK_SIZE = 256 * 1024; // 256 KB

/**
 * Calculate SHA-256 hash of a file.
 * @param {string} filePath
 * @returns {Promise<string>} hex digest
 */
export function calculateSha256(filePath) {
  return new Promise((resolve, reject) => {
    const hash = createHash('sha256');
    const stream = createReadStream(filePath);
    stream.on('data', (chunk) => hash.update(chunk));
    stream.on('end', () => resolve(hash.digest('hex')));
    stream.on('error', reject);
  });
}

/**
 * Upload a file to a remote agent via MQTT chunked transfer.
 * @param {import('./mqtt-client.js').MQTTClient} mqtt
 * @param {string} agentId
 * @param {string} localPath
 * @param {string} remotePath
 * @param {function} onProgress - callback(chunkIndex, totalChunks)
 * @returns {Promise<object>} result from agent
 */
export async function uploadFile(mqtt, agentId, localPath, remotePath, onProgress) {
  const stat = statSync(localPath);
  const fileSize = stat.size;
  const totalChunks = Math.ceil(fileSize / CHUNK_SIZE);
  const transferId = uuidv4();
  const fileName = basename(localPath);

  // Calculate SHA-256
  const sha256 = await calculateSha256(localPath);

  // Subscribe to file status
  await mqtt.subscribe(`agents/${agentId}/files/status`);

  // Publish metadata
  const meta = {
    transferId,
    fileName,
    remotePath,
    fileSize,
    totalChunks,
    chunkSize: CHUNK_SIZE,
    sha256,
    timestamp: Date.now(),
  };

  await mqtt.publish(
    `agents/${agentId}/files/upload/${transferId}/meta`,
    meta
  );

  // Read and send chunks
  const fileData = readFileSync(localPath);

  for (let i = 0; i < totalChunks; i++) {
    const start = i * CHUNK_SIZE;
    const end = Math.min(start + CHUNK_SIZE, fileSize);
    const chunkData = fileData.slice(start, end);
    const base64 = chunkData.toString('base64');

    const chunkPayload = {
      transferId,
      index: i,
      data: base64,
    };

    await mqtt.publish(
      `agents/${agentId}/files/upload/${transferId}/chunks`,
      chunkPayload
    );

    if (onProgress) {
      onProgress(i + 1, totalChunks);
    }
  }

  // Wait for completion status from agent
  const result = await mqtt.waitForMessage(
    `agents/${agentId}/files/status`,
    (msg) => msg.transferId === transferId && (msg.status === 'completed' || msg.status === 'error'),
    60000
  );

  return result;
}
