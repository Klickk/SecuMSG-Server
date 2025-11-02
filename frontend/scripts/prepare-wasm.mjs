#!/usr/bin/env node
import { spawn } from 'node:child_process';
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, '..');
const publicDir = path.join(projectRoot, 'public');

const GO_CMD = process.env.GO ?? 'go';

async function main() {
  await fs.mkdir(publicDir, { recursive: true });
  const wasmExecPath = await findWasmExec();
  const wasmExecTarget = path.join(publicDir, 'wasm_exec.js');
  await fs.copyFile(wasmExecPath, wasmExecTarget);

  await buildWasmModule();
}

async function findWasmExec() {
  const goroot = (await goEnv('GOROOT')).trim();
  if (!goroot) {
    throw new Error('could not determine GOROOT for locating wasm_exec.js');
  }
  const candidates = [
    path.join(goroot, 'misc', 'wasm', 'wasm_exec.js'),
    path.join(goroot, 'lib', 'wasm', 'wasm_exec.js')
  ];
  for (const candidate of candidates) {
    try {
      await fs.access(candidate);
      return candidate;
    } catch (err) {
      if (err && err.code !== 'ENOENT') {
        throw err;
      }
    }
  }
  throw new Error(`wasm_exec.js not found. Checked: ${candidates.join(', ')}`);
}

async function goEnv(key) {
  return new Promise((resolve, reject) => {
    const child = spawn(GO_CMD, ['env', key], { stdio: ['ignore', 'pipe', 'pipe'] });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (chunk) => {
      stdout += chunk;
    });
    child.stderr.on('data', (chunk) => {
      stderr += chunk;
    });
    child.on('error', reject);
    child.on('close', (code) => {
      if (code !== 0) {
        reject(new Error(`go env ${key} failed: ${stderr || `exit code ${code}`}`));
        return;
      }
      resolve(stdout);
    });
  });
}

async function buildWasmModule() {
  const outputPath = path.join(publicDir, 'msgclient.wasm');
  await new Promise((resolve, reject) => {
    const child = spawn(
      GO_CMD,
      ['build', '-o', outputPath, './services/messages/cmd/msgwasm'],
      {
        cwd: path.resolve(projectRoot, '..'),
        stdio: 'inherit',
        env: {
          ...process.env,
          GOOS: 'js',
          GOARCH: 'wasm'
        }
      }
    );
    child.on('error', reject);
    child.on('close', (code) => {
      if (code !== 0) {
        reject(new Error(`go build failed with status ${code}`));
        return;
      }
      resolve();
    });
  });
}

main().catch((err) => {
  console.error(err.message || err);
  process.exitCode = 1;
});
