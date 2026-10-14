import { spawn } from 'node:child_process';
import { convertProcessSignalToExitCode } from 'node:util';

const children = [];
let stopping = false;

function signalGroup(pid, signal) {
    if (!pid || pid <= 1) return false;
    try {
        process.kill(-pid, signal);
        return true;
    } catch (error) {
        if (error.code !== 'ESRCH') throw error;
        return false;
    }
}

function stop(code) {
    if (stopping) return;
    stopping = true;
    for (const child of children) signalGroup(child.pid, 'SIGTERM');

    // Retain process groups after their leaders exit: go run and pnpm can have
    // descendants that still need to stop before this launcher finishes.
    const timeout = setTimeout(() => {
        for (const child of children) signalGroup(child.pid, 'SIGKILL');
        process.exit(code);
    }, 5000);
    const poll = setInterval(() => {
        if (children.every(child => !signalGroup(child.pid, 0))) {
            clearTimeout(timeout);
            clearInterval(poll);
            process.exit(code);
        }
    }, 50);
}

for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
    process.on(signal, () => stop(convertProcessSignalToExitCode(signal)));
}

for (const recipe of ['api', 'frontend']) {
    const child = spawn('just', [recipe], {detached: true, stdio: 'inherit'});
    children.push(child);
    child.once('error', error => {
        console.error(`Could not start ${recipe}: ${error.message}`);
        stop(1);
    });
    child.once('exit', (code, signal) => {
        if (!stopping) {
            console.error(`${recipe} stopped; shutting down development services.`);
            stop(signal ? convertProcessSignalToExitCode(signal) : code ?? 1);
        }
    });
}
