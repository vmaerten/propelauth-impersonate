import { getPreferenceValues } from "@raycast/api";
import { execFile, spawn } from "node:child_process";
import { homedir } from "node:os";
import { promisify } from "node:util";

export type Env = { name: string; color: string };
export type Org = { org_id: string; name: string };
export type User = {
  user_id: string;
  email: string;
  first_name: string;
  last_name: string;
  role: string;
  enabled: boolean;
  locked: boolean;
};

const execFileAsync = promisify(execFile);

function binary(): string {
  return getPreferenceValues<Preferences>().binary.replace(/^~(?=$|\/)/, homedir());
}

// run runs impersonate with args and parses the JSON it prints.
export async function run<T>(args: string[]): Promise<T> {
  try {
    const { stdout } = await execFileAsync(binary(), args, { maxBuffer: 64 * 1024 * 1024 });
    return JSON.parse(stdout) as T;
  } catch (error) {
    throw new Error(describe(error));
  }
}

// open starts the impersonation without waiting for it: signing in to the dashboard first can
// take minutes, and impersonate reports a failure in a notification. Tinycast only lets a
// detached process with its output ignored outlive the command.
export function open(env: string, userId: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const child = spawn(binary(), ["open", "--env", env, "--notify", userId], { detached: true, stdio: "ignore" });
    child.once("error", (error) => reject(new Error(describe(error))));
    child.once("spawn", () => {
      child.unref();
      resolve();
    });
  });
}

function describe(error: unknown): string {
  const { code, stderr, message } = error as { code?: unknown; stderr?: string; message?: string };
  if (code === "ENOENT") {
    return `No impersonate command at ${binary()}: set its path in the extension's preferences`;
  }
  return stderr?.trim().replace(/^Error: /, "") || message || String(error);
}
