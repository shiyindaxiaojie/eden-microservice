import { readFileSync } from "node:fs";
import { resolve } from "node:path";

let payload = "";
process.stdin.on("data", (chunk) => { payload += chunk; });
process.stdin.on("end", () => {
  const warnings = [];
  const root = resolve(process.cwd());
  try {
    const agents = readFileSync(resolve(root, "AGENTS.md"), "utf8");
    if (agents.split(/\r?\n/).length > 200) warnings.push("AGENTS.md exceeds the 200-line limit.");
  } catch {}
  const lower = payload.toLowerCase();
  if (lower.includes(".tmp/") || lower.includes("\\.tmp\\")) warnings.push("Keep temporary artifacts under .tmp/ and out of commits.");
  if (lower.includes("config.yaml") || lower.includes("configs/")) warnings.push("Commit only configuration examples; do not commit local runtime configuration.");
  if (warnings.length) process.stderr.write("[eden-engineering] " + warnings.join(" ") + "\n");
});
