import { createReadStream, statSync, writeFileSync } from 'fs';
import { createHash } from 'crypto';
import { readFileSync } from 'fs';
import { basename, join } from 'path';
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

  // Publish metadata (field names must match Agent's UploadMeta struct)
  const meta = {
    transferId,
    filename: fileName,
    destPath: remotePath,
    size: fileSize,
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

/**
 * Download a file from a remote agent via MQTT chunked transfer.
 * @param {import('./mqtt-client.js').MQTTClient} mqtt
 * @param {string} agentId
 * @param {string} remotePath
 * @param {string} localPath
 * @param {function} onProgress - callback(receivedChunks, totalChunks)
 * @returns {Promise<object>} result with localPath and verified flag
 */
export async function downloadFile(mqtt, agentId, remotePath, localPath, onProgress) {
  const transferId = uuidv4();

  // Subscribe to download topics
  await mqtt.subscribe(`agents/${agentId}/files/download/+/meta`);
  await mqtt.subscribe(`agents/${agentId}/files/download/+/chunks`);

  let meta = null;
  const chunks = new Map();

  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      cleanup();
      reject(new Error('Download timed out after 120 seconds'));
    }, 120000);

    const handler = (topic, msg) => {
      // Match meta message
      const metaMatch = topic.match(/^agents\/[^/]+\/files\/download\/([^/]+)\/meta$/);
      if (metaMatch && msg.transferId) {
        meta = msg;
        if (onProgress) {
          onProgress(0, meta.totalChunks);
        }
        return;
      }

      // Match chunk message
      const chunkMatch = topic.match(/^agents\/[^/]+\/files\/download\/([^/]+)\/chunks$/);
      if (chunkMatch && msg.transferId && meta && msg.transferId === meta.transferId) {
        chunks.set(msg.index, msg.data);

        if (onProgress) {
          onProgress(chunks.size, meta.totalChunks);
        }

        // Check if all chunks received
        if (chunks.size === meta.totalChunks) {
          clearTimeout(timeout);
          mqtt.removeListener('message', handler);
          assembleFile();
        }
      }
    };

    function assembleFile() {
      try {
        // Assemble chunks in order
        const buffers = [];
        for (let i = 0; i < meta.totalChunks; i++) {
          const chunkData = chunks.get(i);
          if (!chunkData) {
            reject(new Error(`Missing chunk ${i}`));
            return;
          }
          buffers.push(Buffer.from(chunkData, 'base64'));
        }

        const fileBuffer = Buffer.concat(buffers);

        // Determine output path
        let outputPath = localPath;
        if (!outputPath) {
          outputPath = join(process.cwd(), meta.filename || basename(remotePath));
        }

        // Write file
        writeFileSync(outputPath, fileBuffer);

        // Verify SHA-256
        let verified = false;
        if (meta.sha256) {
          const hash = createHash('sha256').update(fileBuffer).digest('hex');
          verified = hash === meta.sha256;
        }

        resolve({
          status: 'completed',
          localPath: outputPath,
          fileSize: fileBuffer.length,
          verified,
        });
      } catch (err) {
        reject(err);
      }
    }

    function cleanup() {
      clearTimeout(timeout);
      mqtt.removeListener('message', handler);
    }

    mqtt.on('message', handler);

    // Send file download command to agent (sourcePath matches Agent's Command struct)
    const downloadCommand = {
      id: transferId,
      type: 'file_download',
      sourcePath: remotePath,
      transferId,
      timestamp: Date.now(),
    };

    mqtt.publish(`agents/${agentId}/commands`, downloadCommand).catch((err) => {
      cleanup();
      reject(err);
    });
  });
}
