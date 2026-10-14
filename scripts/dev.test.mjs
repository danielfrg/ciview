import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdtemp, mkdir, writeFile, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import net from 'node:net';

const root = fileURLToPath(new URL('../', import.meta.url));
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));

async function eventually(check, message, timeout = 3000) {
    const deadline = Date.now() + timeout;
    while (Date.now() < deadline) {
        if (await check()) return;
        await delay(25);
    }
    assert.fail(message);
}

async function isListening(port) {
    return new Promise(resolve => {
        const socket = net.connect({host: '127.0.0.1', port});
        socket.setTimeout(200);
        socket.once('connect', () => { socket.destroy(); resolve(true); });
        socket.once('error', () => resolve(false));
        socket.once('timeout', () => { socket.destroy(); resolve(false); });
    });
}

async function fixture(failService, stubborn = false) {
    const dir = await mkdtemp(join(tmpdir(), 'civiewer-dev-'));
    const bin = join(dir, 'bin');
    await mkdir(bin);
    const worker = join(dir, 'worker.mjs');
    await writeFile(worker, `
import http from 'node:http';
import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
const server = http.createServer((req, res) => res.end('ok'));
server.listen(0, '127.0.0.1', () => writeFileSync(join(process.env.CIVIEWER_TEST_DIR, process.argv[2] + '.json'), JSON.stringify({pid: process.pid, port: server.address().port})));
process.on('SIGTERM', () => { if (process.env.CIVIEWER_TEST_STUBBORN !== '1') server.close(() => process.exit(0)); });
`);
    for (const [command, service, other] of [['go', 'api', 'frontend'], ['pnpm', 'frontend', 'api']]) {
        await writeFile(join(bin, command), `#!/usr/bin/env node
const { spawn } = require('node:child_process');
const { existsSync } = require('node:fs');
const { join } = require('node:path');
if (process.env.CIVIEWER_TEST_FAIL_SERVICE === ${JSON.stringify(service)}) {
    setInterval(() => { if (existsSync(join(process.env.CIVIEWER_TEST_DIR, ${JSON.stringify(other + '.json')}))) process.exit(9); }, 10);
} else {
    spawn(process.execPath, [${JSON.stringify(worker)}, ${JSON.stringify(service)}], {stdio: 'inherit'});
}
`, {mode: 0o755});
    }
    const child = spawn('just', ['--justfile', resolve(root, 'justfile'), 'dev'], {
        cwd: root, detached: true,
        env: {...process.env, PATH: bin + ':' + process.env.PATH,
            CIVIEWER_TEST_DIR: dir, CIVIEWER_TEST_FAIL_SERVICE: failService ?? '',
            CIVIEWER_TEST_STUBBORN: stubborn ? '1' : ''},
        stdio: ['ignore', 'pipe', 'pipe'],
    });
    let output = '';
    child.stdout.on('data', data => output += data);
    child.stderr.on('data', data => output += data);
    let result;
    child.once('exit', (code, signal) => { result = {code, signal}; });
    return {
        child,
        result: () => result,
        output: () => output,
        async state(service) {
            try { return JSON.parse(await readFile(join(dir, service + '.json'), 'utf8')); }
            catch (error) { if (error.code === 'ENOENT') return null; throw error; }
        },
        async cleanup() {
            try { process.kill(-child.pid, 'SIGKILL'); } catch (error) { if (error.code !== 'ESRCH') throw error; }
            for (const service of ['api', 'frontend']) {
                const state = await this.state(service);
                if (state) { try { process.kill(state.pid, 'SIGKILL'); } catch (error) { if (error.code !== 'ESRCH') throw error; } }
            }
            await rm(dir, {recursive: true, force: true});
        },
    };
}

for (const failed of ['frontend', 'api']) {
    test(`dev stops the surviving service and its descendant when ${failed} fails`, async () => {
        const f = await fixture(failed);
        const survivor = failed === 'api' ? 'frontend' : 'api';
        try {
            await eventually(async () => !!(await f.state(survivor)), 'surviving service did not start');
            const {port} = await f.state(survivor);
            await eventually(() => f.result(), 'dev stayed running after a service failed: ' + f.output());
            assert.notEqual(f.result().code, 0);
            await eventually(async () => !(await isListening(port)), 'surviving service descendant remained listening');
        } finally { await f.cleanup(); }
    });
}

test('Ctrl+C stops both development services and their descendants', async () => {
    const f = await fixture();
    try {
        await eventually(async () => !!(await f.state('api')) && !!(await f.state('frontend')), 'both services did not start');
        const ports = [(await f.state('api')).port, (await f.state('frontend')).port];
        process.kill(-f.child.pid, 'SIGINT');
        await eventually(() => f.result(), 'dev stayed running after Ctrl+C');
        for (const port of ports) await eventually(async () => !(await isListening(port)), 'service descendant remained listening after Ctrl+C');
    } finally { await f.cleanup(); }
});


test('dev stops a stubborn descendant after its service leader exits', async () => {
    const f = await fixture('frontend', true);
    try {
        await eventually(async () => !!(await f.state('api')), 'API descendant did not start');
        const {port} = await f.state('api');
        await eventually(() => f.result(), 'dev did not finish bounded cleanup', 8000);
        assert.notEqual(f.result().code, 0);
        await eventually(async () => !(await isListening(port)), 'stubborn descendant remained listening');
    } finally { await f.cleanup(); }
});
